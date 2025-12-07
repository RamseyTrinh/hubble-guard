package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"hubble-guard/internal/model"

	prommodel "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type UnusualTrafficRule struct {
	name           string
	enabled        bool
	severity       string
	prometheusAPI  PrometheusQueryClient
	logger         *logrus.Logger
	interval       time.Duration
	stopChan       chan struct{}
	alertEmitter   func(*model.Alert)
	namespaces     []string
	allowedSources map[string][]string // key: destination service, value: list of allowed source pods/services

	// Track alerted pairs to avoid duplicate alerts
	alertedPairs  map[string]time.Time
	alertedMu     sync.RWMutex
	alertCooldown time.Duration
}

func NewUnusualTrafficRule(enabled bool, severity string, promClient PrometheusQueryClient, logger *logrus.Logger) *UnusualTrafficRule {
	return &UnusualTrafficRule{
		name:          "unusual_traffic",
		enabled:       enabled,
		severity:      severity,
		prometheusAPI: promClient,
		logger:        logger,
		interval:      10 * time.Second,
		stopChan:      make(chan struct{}),
		namespaces:    []string{"default"},
		allowedSources: map[string][]string{
			"demo-api": {"demo-frontend"},
		},
		alertedPairs:  make(map[string]time.Time),
		alertCooldown: 60 * time.Second,
	}
}

func (r *UnusualTrafficRule) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

func (r *UnusualTrafficRule) SetNamespaces(namespaces []string) {
	if len(namespaces) == 0 {
		r.namespaces = []string{"default"}
	} else {
		r.namespaces = namespaces
	}
}

func (r *UnusualTrafficRule) SetAllowedSources(allowedSources map[string][]string) {
	if allowedSources != nil {
		r.allowedSources = allowedSources
	}
}

func (r *UnusualTrafficRule) Name() string {
	return r.name
}

func (r *UnusualTrafficRule) IsEnabled() bool {
	return r.enabled
}

func (r *UnusualTrafficRule) Start(ctx context.Context) {
	if !r.enabled {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkFromPrometheus(ctx)
			r.cleanupOldAlerts()
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		}
	}
}

func (r *UnusualTrafficRule) Stop() {
	close(r.stopChan)
}

func (r *UnusualTrafficRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	return nil
}

func (r *UnusualTrafficRule) checkFromPrometheus(ctx context.Context) {
	for _, namespace := range r.namespaces {
		r.checkNamespace(ctx, namespace)
	}
}

func (r *UnusualTrafficRule) checkNamespace(ctx context.Context, namespace string) {
	// Query source_dest_traffic_total metric for this namespace
	// Get traffic in last 30 seconds
	query := fmt.Sprintf(`sum by (source_pod, dest_pod, dest_service) (increase(source_dest_traffic_total{namespace="%s"}[30s]))`, namespace)

	result, err := r.prometheusAPI.Query(ctx, query, 30*time.Second)
	if err != nil {
		return
	}

	vector, ok := result.(prommodel.Vector)
	if !ok {
		return
	}

	for _, sample := range vector {
		sourcePod := ""
		destPod := ""
		destService := ""

		if val, exists := sample.Metric["source_pod"]; exists {
			sourcePod = string(val)
		}
		if val, exists := sample.Metric["dest_pod"]; exists {
			destPod = string(val)
		}
		if val, exists := sample.Metric["dest_service"]; exists {
			destService = string(val)
		}

		trafficCount := float64(sample.Value)

		// Only check if there's actual traffic
		if trafficCount <= 0 {
			continue
		}

		// Check if this source is unusual for the destination
		isUnusual := r.isUnusualSource(sourcePod, destService)

		if isUnusual {
			r.emitAlert(namespace, sourcePod, destPod, destService, trafficCount)
		}
	}
}

// isUnusualSource checks if the source pod is NOT in the allowed list for the destination service
func (r *UnusualTrafficRule) isUnusualSource(sourcePod, destService string) bool {
	if sourcePod == "" || destService == "" {
		return false
	}

	// Check if destination is a protected service
	allowedList, isProtected := r.allowedSources[destService]

	if !isProtected {
		// Service is not in the protected list, so any source is allowed
		return false
	}

	// Check if source is in the allowed list
	for _, allowed := range allowedList {
		if allowed == "*" {
			return false // Wildcard - all sources allowed
		}
		// Check if source pod name starts with the allowed prefix
		if strings.HasPrefix(sourcePod, allowed) {
			return false // Source is allowed
		}
	}

	// Source is NOT in allowed list - this is unusual
	r.logger.Warnf("[Unusual Traffic] UNUSUAL: %s is NOT in allowed list %v for %s", sourcePod, allowedList, destService)
	return true
}

func (r *UnusualTrafficRule) emitAlert(namespace, sourcePod, destPod, destService string, trafficCount float64) {
	// Create unique key for this source-dest pair
	alertKey := fmt.Sprintf("%s:%s:%s", namespace, sourcePod, destService)

	// Check if we already alerted for this pair recently
	r.alertedMu.RLock()
	lastAlerted, exists := r.alertedPairs[alertKey]
	r.alertedMu.RUnlock()

	if exists && time.Since(lastAlerted) < r.alertCooldown {
		// Already alerted recently, skip
		return
	}

	// Update last alerted time
	r.alertedMu.Lock()
	r.alertedPairs[alertKey] = time.Now()
	r.alertedMu.Unlock()

	// Get allowed sources for this service
	allowedSources := r.allowedSources[destService]

	alert := &model.Alert{
		Type:      r.name,
		Severity:  r.severity,
		Namespace: namespace,
		Message: fmt.Sprintf("Unusual traffic detected: '%s' is accessing '%s' (allowed sources: %v). Traffic count: %.0f",
			sourcePod, destService, allowedSources, trafficCount),
		Timestamp: time.Now(),
	}

	r.logger.Warnf("[Unusual Traffic] Alert: %s -> %s (dest_pod: %s) in namespace %s",
		sourcePod, destService, destPod, namespace)

	if r.alertEmitter != nil {
		r.alertEmitter(alert)
	}
}

// cleanupOldAlerts removes expired entries from alertedPairs map
func (r *UnusualTrafficRule) cleanupOldAlerts() {
	r.alertedMu.Lock()
	defer r.alertedMu.Unlock()

	now := time.Now()
	for key, lastAlerted := range r.alertedPairs {
		if now.Sub(lastAlerted) > r.alertCooldown*2 {
			delete(r.alertedPairs, key)
		}
	}
}
