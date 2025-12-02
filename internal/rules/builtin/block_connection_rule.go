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

// BlockConnectionRule detects blocked connections by querying Prometheus metrics
// Alerts when count(flows{verdict="DROP"}) > threshold in 1 minute
type BlockConnectionRule struct {
	name          string
	enabled       bool
	severity      string
	threshold     float64
	prometheusAPI PrometheusQueryClient
	logger        *logrus.Logger
	mu            sync.RWMutex
	interval      time.Duration
	stopChan      chan struct{}
	alertEmitter  func(*model.Alert)
	namespaces    []string
}

// NewBlockConnectionRule creates a new Block Connection rule that queries Prometheus
func NewBlockConnectionRule(enabled bool, severity string, threshold float64, promClient PrometheusQueryClient, logger *logrus.Logger) *BlockConnectionRule {
	if threshold <= 0 {
		threshold = 10.0
	}
	return &BlockConnectionRule{
		name:          "block_connection",
		enabled:       enabled,
		severity:      severity,
		threshold:     threshold,
		prometheusAPI: promClient,
		logger:        logger,
		interval:      10 * time.Second,
		stopChan:      make(chan struct{}),
		namespaces:    []string{"default"},
	}
}

func (r *BlockConnectionRule) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

func (r *BlockConnectionRule) SetNamespaces(namespaces []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(namespaces) == 0 {
		r.namespaces = []string{"default"}
	} else {
		r.namespaces = namespaces
	}
}

func (r *BlockConnectionRule) Name() string {
	return r.name
}

func (r *BlockConnectionRule) IsEnabled() bool {
	return r.enabled
}

func (r *BlockConnectionRule) Start(ctx context.Context) {
	if !r.enabled {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.Infof("[Block Connection] Starting periodic checks from Prometheus (interval: %v, threshold: %.0f)", r.interval, r.threshold)

	for {
		select {
		case <-ticker.C:
			r.checkFromPrometheus(ctx)
		case <-ctx.Done():
			r.logger.Info("[Block Connection] Stopping periodic checks")
			return
		case <-r.stopChan:
			r.logger.Info("[Block Connection] Rule stopped")
			return
		}
	}
}

func (r *BlockConnectionRule) Stop() {
	close(r.stopChan)
}

func (r *BlockConnectionRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	return nil
}

func (r *BlockConnectionRule) checkFromPrometheus(ctx context.Context) {
	r.mu.RLock()
	namespaces := r.namespaces
	r.mu.RUnlock()

	for _, namespace := range namespaces {
		r.checkNamespace(ctx, namespace)
	}
}

func (r *BlockConnectionRule) checkNamespace(ctx context.Context, namespace string) {
	query := fmt.Sprintf(`sum(increase(hubble_flows_by_verdict_total{verdict="DROP", namespace="%s"}[1m]))`, namespace)

	result, err := r.prometheusAPI.Query(ctx, query, 10*time.Second)
	if err != nil {
		r.logger.Errorf("[Block Connection] Failed to query Prometheus for namespace %s: %v", namespace, err)
		return
	}

	var dropCount float64
	if vector, ok := result.(prommodel.Vector); ok && len(vector) > 0 {
		dropCount = float64(vector[0].Value)
	} else {
		r.logger.Debugf("[Block Connection] No DROP flows from Prometheus for namespace %s", namespace)
		return
	}

	r.logger.Debugf("[Block Connection] Namespace: %s | DROP flows in last 1m: %.0f (threshold: %.0f)", namespace, dropCount, r.threshold)

	if dropCount > r.threshold {
		alert := &model.Alert{
			Type:      r.name,
			Severity:  r.severity,
			Namespace: namespace,
			Message:   fmt.Sprintf("Blocked connections detected in namespace %s: %.0f DROP flows in 1 minute (threshold: %.0f)", namespace, dropCount, r.threshold),
			Timestamp: time.Now(),
		}
		r.logger.Warnf("Block Connection Rule Alert: %s", alert.Message)
		if r.alertEmitter != nil {
			r.alertEmitter(alert)
		}
	}
}
