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
		interval:      10 * time.Second, // Check every 10 seconds
		stopChan:      make(chan struct{}),
		namespaces:    []string{"default", "kube-system"},
	}
}

// SetAlertEmitter sets the function to emit alerts
func (r *BlockConnectionRule) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

// SetNamespaces sets namespaces to check
func (r *BlockConnectionRule) SetNamespaces(namespaces []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.namespaces = namespaces
}

// Name returns the rule name
func (r *BlockConnectionRule) Name() string {
	return r.name
}

// IsEnabled returns whether the rule is enabled
func (r *BlockConnectionRule) IsEnabled() bool {
	return r.enabled
}

// Start begins periodic checking from Prometheus
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

// Stop stops the rule
func (r *BlockConnectionRule) Stop() {
	close(r.stopChan)
}

// Evaluate is called for each flow but we don't process flows directly
func (r *BlockConnectionRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	// Rules now query from Prometheus, not from individual flows
	return nil
}

// checkFromPrometheus queries Prometheus and checks for blocked connections
func (r *BlockConnectionRule) checkFromPrometheus(ctx context.Context) {
	r.mu.RLock()
	namespaces := r.namespaces
	r.mu.RUnlock()

	if len(namespaces) == 0 {
		namespaces = r.getNamespacesFromConfig()
	}

	for _, namespace := range namespaces {
		r.checkNamespace(ctx, namespace)
	}
}

func (r *BlockConnectionRule) checkNamespace(ctx context.Context, namespace string) {
	// Query for count of DROP flows in the last 1 minute
	// Using increase() to count the number of DROP flows in the time window
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
			Message:   fmt.Sprintf("Blocked connections detected in namespace %s: %.0f DROP flows in 1 minute (threshold: %.0f)", namespace, dropCount, r.threshold),
			Timestamp: time.Now(),
		}
		r.logger.Warnf("Block Connection Rule Alert: %s", alert.Message)
		// Emit alert through emitter function
		if r.alertEmitter != nil {
			r.alertEmitter(alert)
		}
	}
}

// getNamespacesFromConfig returns list of namespaces to check
func (r *BlockConnectionRule) getNamespacesFromConfig() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.namespaces) > 0 {
		return r.namespaces
	}
	// Default fallback
	return []string{"default", "kube-system"}
}
