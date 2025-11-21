package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hubble-anomaly-detector/internal/model"

	prommodel "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type OutboundRule struct {
	name            string
	enabled         bool
	severity        string
	threshold       float64
	suspiciousPorts map[int]bool
	prometheusAPI   PrometheusQueryClient
	logger          *logrus.Logger
	mu              sync.RWMutex
	interval        time.Duration
	stopChan        chan struct{}
	alertEmitter    func(*model.Alert)
	namespaces      []string
}

func NewOutboundRule(enabled bool, severity string, threshold float64, promClient PrometheusQueryClient, logger *logrus.Logger) *OutboundRule {
	if threshold <= 0 {
		threshold = 10.0
	}

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
		threshold:       threshold,
		suspiciousPorts: suspiciousPorts,
		prometheusAPI:   promClient,
		logger:          logger,
		interval:        10 * time.Second,
		stopChan:        make(chan struct{}),
		namespaces:      []string{"default"},
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

	r.logger.Infof("[Suspicious Outbound] Starting periodic checks from Prometheus (interval: %v, threshold: %.0f)", r.interval, r.threshold)

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

		query := fmt.Sprintf(`sum(increase(hubble_suspicious_outbound_total{namespace="%s", destination_port="%d"}[1m]))`, namespace, port)

		result, err := r.prometheusAPI.Query(ctx, query, 10*time.Second)
		if err != nil {
			r.logger.Errorf("[Suspicious Outbound] Failed to query Prometheus for namespace %s, port %d: %v", namespace, port, err)
			continue
		}

		var count float64
		if vector, ok := result.(prommodel.Vector); ok && len(vector) > 0 {
			count = float64(vector[0].Value)
		} else {
			r.logger.Debugf("[Suspicious Outbound] No suspicious outbound connections from Prometheus for namespace %s, port %d", namespace, port)
			continue
		}

		r.logger.Debugf("[Suspicious Outbound] Namespace: %s | Port: %d | Connections in last 1m: %.0f (threshold: %.0f)", namespace, port, count, r.threshold)

		if count > r.threshold {
			alert := &model.Alert{
				Type:      r.name,
				Severity:  r.severity,
				Namespace: namespace,
				Message:   fmt.Sprintf("Suspicious outbound connection detected in namespace %s: %.0f connections to port %d in 1 minute (threshold: %.0f)", namespace, count, port, r.threshold),
				Timestamp: time.Now(),
			}
			r.logger.Warnf("Suspicious Outbound Rule Alert: %s", alert.Message)
			if r.alertEmitter != nil {
				r.alertEmitter(alert)
			}
		}
	}
}
