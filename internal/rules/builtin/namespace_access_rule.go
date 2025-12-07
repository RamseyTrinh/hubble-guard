package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hubble-guard/internal/model"

	prommodel "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type NamespaceAccessRule struct {
	name          string
	enabled       bool
	severity      string
	forbiddenNS   map[string]bool
	prometheusAPI PrometheusQueryClient
	logger        *logrus.Logger
	mu            sync.RWMutex
	interval      time.Duration
	stopChan      chan struct{}
	alertEmitter  func(*model.Alert)

	// Alert cooldown to prevent duplicate alerts
	alertedPairs  map[string]time.Time
	alertedMu     sync.RWMutex
	alertCooldown time.Duration
}

func NewNamespaceAccessRule(enabled bool, severity string, promClient PrometheusQueryClient, logger *logrus.Logger) *NamespaceAccessRule {
	return &NamespaceAccessRule{
		name:          "unauthorized_namespace_access",
		enabled:       enabled,
		severity:      severity,
		forbiddenNS:   make(map[string]bool),
		prometheusAPI: promClient,
		logger:        logger,
		interval:      10 * time.Second,
		stopChan:      make(chan struct{}),
		alertedPairs:  make(map[string]time.Time),
		alertCooldown: 60 * time.Second, // 1 minute cooldown
	}
}

func (r *NamespaceAccessRule) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

func (r *NamespaceAccessRule) SetForbiddenNamespaces(forbiddenNS []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.forbiddenNS = make(map[string]bool)
	for _, ns := range forbiddenNS {
		r.forbiddenNS[ns] = true
	}
	r.logger.Infof("Namespace Access Rule: Set %d forbidden namespaces: %v", len(forbiddenNS), forbiddenNS)
}

func (r *NamespaceAccessRule) Name() string {
	return r.name
}

func (r *NamespaceAccessRule) IsEnabled() bool {
	return r.enabled
}

func (r *NamespaceAccessRule) Start(ctx context.Context) {
	if !r.enabled {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.Infof("[Namespace Access] Starting periodic checks from Prometheus (interval: %v)", r.interval)

	for {
		select {
		case <-ticker.C:
			r.checkFromPrometheus(ctx)
		case <-ctx.Done():
			r.logger.Info("[Namespace Access] Stopping periodic checks")
			return
		case <-r.stopChan:
			r.logger.Info("[Namespace Access] Rule stopped")
			return
		}
	}
}
func (r *NamespaceAccessRule) Stop() {
	close(r.stopChan)
}

func (r *NamespaceAccessRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	return nil
}

func (r *NamespaceAccessRule) checkFromPrometheus(ctx context.Context) {
	r.mu.RLock()
	forbiddenNS := make(map[string]bool)
	for ns := range r.forbiddenNS {
		forbiddenNS[ns] = true
	}
	r.mu.RUnlock()

	if len(forbiddenNS) == 0 {
		return
	}

	for forbiddenNSName := range forbiddenNS {
		query := fmt.Sprintf(`sum(increase(namespace_access_total{dest_namespace="%s"}[1m])) by (source_namespace, dest_namespace, dest_service, dest_pod)`, forbiddenNSName)

		result, err := r.prometheusAPI.Query(ctx, query, 10*time.Second)
		if err != nil {
			r.logger.Errorf("[Namespace Access] Failed to query Prometheus for namespace %s: %v", forbiddenNSName, err)
			continue
		}

		vector, ok := result.(prommodel.Vector)
		if !ok {
			r.logger.Debugf("[Namespace Access] No data from Prometheus for namespace %s", forbiddenNSName)
			continue
		}

		for _, sample := range vector {
			sourceNS := ""
			destNS := ""
			destService := ""
			destPod := ""

			if val, exists := sample.Metric["source_namespace"]; exists {
				sourceNS = string(val)
			}
			if val, exists := sample.Metric["dest_namespace"]; exists {
				destNS = string(val)
			}
			if val, exists := sample.Metric["dest_service"]; exists {
				destService = string(val)
			}
			if val, exists := sample.Metric["dest_pod"]; exists {
				destPod = string(val)
			}

			accessCount := float64(sample.Value)

			// Skip if no actual access or same namespace
			if accessCount <= 0 || sourceNS == destNS || sourceNS == "" || destNS == "" {
				continue
			}

			if forbiddenNS[destNS] {
				// Check cooldown - avoid duplicate alerts
				alertKey := fmt.Sprintf("%s->%s", sourceNS, destNS)
				if r.isAlertedRecently(alertKey) {
					continue
				}

				isDNSRequest := false
				if destService == "kube-dns" || destService == "coredns" || destPod == "kube-dns" || destPod == "coredns" {
					isDNSRequest = true
				}

				var message string
				if isDNSRequest && sourceNS != "kube-system" {
					message = fmt.Sprintf("Unauthorized DNS access detected: pod in namespace '%s' accessing kube-dns in kube-system namespace", sourceNS)
				} else {
					message = fmt.Sprintf("Unauthorized access to sensitive namespace detected: pod in namespace '%s' accessing namespace '%s'", sourceNS, destNS)
				}

				// Mark as alerted
				r.markAlerted(alertKey)

				alert := &model.Alert{
					Type:      r.name,
					Severity:  r.severity,
					Message:   message,
					Timestamp: time.Now(),
				}

				r.logger.Warnf("Namespace Access Rule Alert: %s", alert.Message)
				if r.alertEmitter != nil {
					r.alertEmitter(alert)
				}
			}
		}
	}
}

func (r *NamespaceAccessRule) isAlertedRecently(key string) bool {
	r.alertedMu.RLock()
	defer r.alertedMu.RUnlock()

	lastAlerted, exists := r.alertedPairs[key]
	if !exists {
		return false
	}
	return time.Since(lastAlerted) < r.alertCooldown
}

func (r *NamespaceAccessRule) markAlerted(key string) {
	r.alertedMu.Lock()
	defer r.alertedMu.Unlock()
	r.alertedPairs[key] = time.Now()
}
