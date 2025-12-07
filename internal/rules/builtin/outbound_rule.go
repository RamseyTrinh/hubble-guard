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

type OutboundRule struct {
	name            string
	enabled         bool
	severity        string
	suspiciousPorts map[int]bool
	prometheusAPI   PrometheusQueryClient
	logger          *logrus.Logger
	mu              sync.RWMutex
	interval        time.Duration
	stopChan        chan struct{}
	alertEmitter    func(*model.Alert)
	namespaces      []string
	// Alert cooldown tracking
	alertedPorts  map[string]time.Time // key: "namespace:port"
	alertCooldown time.Duration
}

func NewOutboundRule(enabled bool, severity string, promClient PrometheusQueryClient, logger *logrus.Logger) *OutboundRule {
	suspiciousPorts := map[int]bool{
		22:   false, // SSH - may be suspicious depending on context
		23:   true,  // Telnet - suspicious
		135:  true,  // RPC - suspicious
		445:  true,  // SMB - suspicious
		1433: true,  // SQL Server - suspicious
		3306: true,  // MySQL - suspicious
		5432: true,  // PostgreSQL - suspicious
	}

	return &OutboundRule{
		name:            "suspicious_outbound",
		enabled:         enabled,
		severity:        severity,
		suspiciousPorts: suspiciousPorts,
		prometheusAPI:   promClient,
		logger:          logger,
		interval:        10 * time.Second,
		stopChan:        make(chan struct{}),
		namespaces:      []string{"default"},
		alertedPorts:    make(map[string]time.Time),
		alertCooldown:   60 * time.Second,
	}
}

func (r *OutboundRule) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

func (r *OutboundRule) SetNamespaces(namespaces []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(namespaces) == 0 {
		r.namespaces = []string{"default"}
	} else {
		r.namespaces = namespaces
	}
}

func (r *OutboundRule) Name() string {
	return r.name
}

func (r *OutboundRule) IsEnabled() bool {
	return r.enabled
}

func (r *OutboundRule) Start(ctx context.Context) {
	if !r.enabled {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.Infof("[Suspicious Outbound] Starting periodic checks from Prometheus (interval: %v) - alerts on ANY connection to dangerous ports", r.interval)

	for {
		select {
		case <-ticker.C:
			r.checkFromPrometheus(ctx)
		case <-ctx.Done():
			r.logger.Info("[Suspicious Outbound] Stopping periodic checks")
			return
		case <-r.stopChan:
			r.logger.Info("[Suspicious Outbound] Rule stopped")
			return
		}
	}
}

func (r *OutboundRule) Stop() {
	close(r.stopChan)
}

func (r *OutboundRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	return nil
}

func (r *OutboundRule) checkFromPrometheus(ctx context.Context) {
	r.mu.RLock()
	namespaces := r.namespaces
	suspiciousPorts := make(map[int]bool)
	for port, val := range r.suspiciousPorts {
		suspiciousPorts[port] = val
	}
	r.mu.RUnlock()

	if len(namespaces) == 0 {
		namespaces = []string{"default"}
	}

	for _, namespace := range namespaces {
		r.checkNamespace(ctx, namespace, suspiciousPorts)
	}
}

func (r *OutboundRule) checkNamespace(ctx context.Context, namespace string, suspiciousPorts map[int]bool) {
	for port, isSuspicious := range suspiciousPorts {
		if !isSuspicious {
			continue
		}

		// Query for any increase in connections to this port in the last 1 minute
		query := fmt.Sprintf(`sum(increase(hubble_suspicious_outbound_total{namespace="%s"}[1m]))`, namespace)

		result, err := r.prometheusAPI.Query(ctx, query, 10*time.Second)
		if err != nil {
			r.logger.Errorf("[Suspicious Outbound] Failed to query Prometheus for namespace %s: %v", namespace, err)
			continue
		}

		var count float64
		if vector, ok := result.(prommodel.Vector); ok && len(vector) > 0 {
			count = float64(vector[0].Value)
		} else {
			continue
		}

		// Alert on ANY connection (count > 0) to dangerous port
		if count > 0 {
			alertKey := fmt.Sprintf("%s:%d", namespace, port)

			// Check cooldown
			r.mu.RLock()
			lastAlert, exists := r.alertedPorts[alertKey]
			r.mu.RUnlock()

			if exists && time.Since(lastAlert) < r.alertCooldown {
				r.logger.Debugf("[Suspicious Outbound] Alert for %s in cooldown, skipping", alertKey)
				continue
			}

			// Update cooldown
			r.mu.Lock()
			r.alertedPorts[alertKey] = time.Now()
			r.mu.Unlock()

			portName := r.getPortName(port)
			alert := &model.Alert{
				Type:      r.name,
				Severity:  r.severity,
				Namespace: namespace,
				Message:   fmt.Sprintf("⚠️ Suspicious outbound connection detected: %.0f connection(s) to port %d (%s) from namespace %s", count, port, portName, namespace),
				Timestamp: time.Now(),
			}
			r.logger.Warnf("Suspicious Outbound Rule Alert: %s", alert.Message)
			if r.alertEmitter != nil {
				r.alertEmitter(alert)
			}
		}
	}
}

func (r *OutboundRule) getPortName(port int) string {
	names := map[int]string{
		22:   "SSH",
		23:   "Telnet",
		135:  "RPC",
		445:  "SMB",
		1433: "SQL Server",
		3306: "MySQL",
		5432: "PostgreSQL",
	}
	if name, ok := names[port]; ok {
		return name
	}
	return "Unknown"
}
