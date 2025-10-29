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

// DDoSRulePrometheus detects potential DDoS attacks by querying Prometheus metrics
type DDoSRulePrometheus struct {
	name           string
	enabled        bool
	severity       string
	threshold      float64
	prometheusAPI  PrometheusQueryClient
	baseline       map[string]float64
	baselineWindow map[string]time.Duration
	baselineStart  map[string]time.Time
	baselineRates  map[string][]float64
	logger         *logrus.Logger
	mu             sync.RWMutex
	interval       time.Duration
	stopChan       chan struct{}
	alertEmitter   func(*model.Alert)
	namespaces     []string
}

// PrometheusQueryClient interface for querying Prometheus
type PrometheusQueryClient interface {
	Query(ctx context.Context, query string, timeout time.Duration) (prommodel.Value, error)
}

// NewDDoSRulePrometheus creates a new DDoS rule that queries Prometheus
func NewDDoSRulePrometheus(enabled bool, severity string, threshold float64, promClient PrometheusQueryClient, logger *logrus.Logger) *DDoSRulePrometheus {
	if threshold <= 0 {
		threshold = 3.0
	}
	return &DDoSRulePrometheus{
		name:           "ddos_traffic_spike",
		enabled:        enabled,
		severity:       severity,
		threshold:      threshold,
		prometheusAPI:  promClient,
		baseline:       make(map[string]float64),
		baselineWindow: make(map[string]time.Duration),
		baselineStart:  make(map[string]time.Time),
		baselineRates:  make(map[string][]float64),
		logger:         logger,
		interval:       10 * time.Second, // Check every 10 seconds
		stopChan:       make(chan struct{}),
		namespaces:     []string{"default", "kube-system"},
	}
}

// SetAlertEmitter sets the function to emit alerts
func (r *DDoSRulePrometheus) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

// SetNamespaces sets namespaces to check
func (r *DDoSRulePrometheus) SetNamespaces(namespaces []string) {
	r.namespaces = namespaces
}

// Name returns the rule name
func (r *DDoSRulePrometheus) Name() string {
	return r.name
}

// IsEnabled returns whether the rule is enabled
func (r *DDoSRulePrometheus) IsEnabled() bool {
	return r.enabled
}

// Start begins periodic checking from Prometheus
func (r *DDoSRulePrometheus) Start(ctx context.Context) {
	if !r.enabled {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.Infof("[Traffic Spike] Starting periodic checks from Prometheus (interval: %v)", r.interval)

	for {
		select {
		case <-ticker.C:
			r.checkFromPrometheus(ctx)
		case <-ctx.Done():
			r.logger.Info("[Traffic Spike] Stopping periodic checks")
			return
		case <-r.stopChan:
			r.logger.Info("[Traffic Spike] Rule stopped")
			return
		}
	}
}

// Stop stops the rule
func (r *DDoSRulePrometheus) Stop() {
	close(r.stopChan)
}

// Evaluate is called for each flow but we don't process flows directly
func (r *DDoSRulePrometheus) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	// Rules now query from Prometheus, not from individual flows
	return nil
}

// checkFromPrometheus queries Prometheus and checks for traffic spikes
func (r *DDoSRulePrometheus) checkFromPrometheus(ctx context.Context) {
	// Get namespaces from config
	namespaces := r.namespaces
	if len(namespaces) == 0 {
		namespaces = r.getNamespacesFromConfig()
	}

	for _, namespace := range namespaces {
		r.checkNamespace(ctx, namespace)
	}
}

func (r *DDoSRulePrometheus) checkNamespace(ctx context.Context, namespace string) {
	// Query current rate from Prometheus
	query := fmt.Sprintf(`rate(hubble_flows_total{namespace="%s"}[1m])`, namespace)

	result, err := r.prometheusAPI.Query(ctx, query, 10*time.Second)
	if err != nil {
		r.logger.Errorf("[Traffic Spike] Failed to query Prometheus for namespace %s: %v", namespace, err)
		return
	}

	var currentRate float64
	if vector, ok := result.(prommodel.Vector); ok && len(vector) > 0 {
		currentRate = float64(vector[0].Value)
	} else {
		r.logger.Debugf("[Traffic Spike] No data from Prometheus for namespace %s", namespace)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if we have baseline
	baseline, exists := r.baseline[namespace]
	if !exists {
		// Start baseline collection (1 minute)
		baselineStart, baselineStarted := r.baselineStart[namespace]
		if !baselineStarted {
			r.baselineStart[namespace] = time.Now()
			r.baselineWindow[namespace] = 1 * time.Minute
			r.baselineRates[namespace] = []float64{currentRate}
			r.logger.Infof("[Traffic Spike] Namespace: %s | Starting baseline collection (1 minute) | Rate: %.2f flows/sec", namespace, currentRate)
			return
		}

		// Check if baseline collection is complete
		now := time.Now()
		elapsed := now.Sub(baselineStart)
		if elapsed < r.baselineWindow[namespace] {
			r.baselineRates[namespace] = append(r.baselineRates[namespace], currentRate)
			remaining := r.baselineWindow[namespace] - elapsed
			r.logger.Infof("[Traffic Spike] Namespace: %s | Collecting baseline... Rate: %.2f flows/sec | Remaining: %.1f seconds", namespace, currentRate, remaining.Seconds())
			return
		}

		// Calculate baseline
		if len(r.baselineRates[namespace]) > 0 {
			sum := 0.0
			for _, rate := range r.baselineRates[namespace] {
				sum += rate
			}
			avgBaseline := sum / float64(len(r.baselineRates[namespace]))
			r.baseline[namespace] = avgBaseline
			r.logger.Infof("[Traffic Spike] Baseline calculated for namespace %s: %.2f flows/sec (from %d samples over 1 minute)", namespace, avgBaseline, len(r.baselineRates[namespace]))
			delete(r.baselineStart, namespace)
			delete(r.baselineWindow, namespace)
			delete(r.baselineRates, namespace)
		}
		return
	}

	r.logger.Infof("[Traffic Spike] Namespace: %s | Rate: %.2f flows/sec", namespace, currentRate)

	if baseline <= 0 {
		r.baseline[namespace] = currentRate
		r.logger.Infof("[Traffic Spike] Updating baseline for namespace %s: %.2f flows/sec", namespace, currentRate)
		return
	}

	multiplier := currentRate / baseline
	r.logger.Infof("[Traffic Spike] Namespace: %s | Current rate: %.2f flows/sec | Baseline: %.2f flows/sec | Multiplier: %.2fx", namespace, currentRate, baseline, multiplier)

	if multiplier > r.threshold {
		alert := &model.Alert{
			Type:      r.name,
			Severity:  r.severity,
			Message:   fmt.Sprintf("Traffic spike detected in namespace %s: %.2fx baseline (%.2f flows/sec vs %.2f baseline)", namespace, multiplier, currentRate, baseline),
			Timestamp: time.Now(),
		}
		r.logger.Warnf("DDoS Rule Alert: %s", alert.Message)
		// Emit alert through emitter function
		if r.alertEmitter != nil {
			r.alertEmitter(alert)
		}
	}
}

// getNamespacesFromConfig returns list of namespaces to check
func (r *DDoSRulePrometheus) getNamespacesFromConfig() []string {
	// Use namespaces set via SetNamespaces
	if len(r.namespaces) > 0 {
		return r.namespaces
	}
	// Default fallback
	return []string{"default", "kube-system"}
}
