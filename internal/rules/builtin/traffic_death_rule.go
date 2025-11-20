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

type TrafficDeathRulePrometheus struct {
	name           string
	enabled        bool
	severity       string
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

func NewTrafficDeathRulePrometheus(enabled bool, severity string, promClient PrometheusQueryClient, logger *logrus.Logger) *TrafficDeathRulePrometheus {
	return &TrafficDeathRulePrometheus{
		name:           "traffic_death",
		enabled:        enabled,
		severity:       severity,
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

func (r *TrafficDeathRulePrometheus) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

func (r *TrafficDeathRulePrometheus) SetNamespaces(namespaces []string) {
	r.namespaces = namespaces
}

func (r *TrafficDeathRulePrometheus) Name() string {
	return r.name
}

func (r *TrafficDeathRulePrometheus) IsEnabled() bool {
	return r.enabled
}

func (r *TrafficDeathRulePrometheus) Start(ctx context.Context) {
	if !r.enabled {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.Infof("[Traffic Death] Starting periodic checks from Prometheus (interval: %v)", r.interval)

	for {
		select {
		case <-ticker.C:
			r.checkFromPrometheus(ctx)
		case <-ctx.Done():
			r.logger.Info("[Traffic Death] Stopping periodic checks")
			return
		case <-r.stopChan:
			r.logger.Info("[Traffic Death] Rule stopped")
			return
		}
	}
}

func (r *TrafficDeathRulePrometheus) Stop() {
	close(r.stopChan)
}

func (r *TrafficDeathRulePrometheus) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	return nil
}

func (r *TrafficDeathRulePrometheus) checkFromPrometheus(ctx context.Context) {
	namespaces := r.namespaces
	if len(namespaces) == 0 {
		namespaces = r.getNamespacesFromConfig()
	}

	for _, namespace := range namespaces {
		r.checkNamespace(ctx, namespace)
	}
}

func (r *TrafficDeathRulePrometheus) checkNamespace(ctx context.Context, namespace string) {
	query := fmt.Sprintf(`rate(hubble_flows_total{namespace="%s"}[1m])`, namespace)

	result, err := r.prometheusAPI.Query(ctx, query, 10*time.Second)
	if err != nil {
		r.logger.Errorf("[Traffic Death] Failed to query Prometheus for namespace %s: %v", namespace, err)
		return
	}

	var currentRate float64
	if vector, ok := result.(prommodel.Vector); ok && len(vector) > 0 {
		currentRate = float64(vector[0].Value)
	} else {
		// No data means rate is 0
		currentRate = 0.0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	baseline, exists := r.baseline[namespace]
	if !exists {
		baselineStart, baselineStarted := r.baselineStart[namespace]
		if !baselineStarted {
			r.baselineStart[namespace] = time.Now()
			r.baselineWindow[namespace] = 1 * time.Minute
			r.baselineRates[namespace] = []float64{currentRate}
			r.logger.Infof("[Traffic Death] Namespace: %s | Starting baseline collection (1 minute) | Rate: %.2f flows/sec", namespace, currentRate)
			return
		}

		now := time.Now()
		elapsed := now.Sub(baselineStart)
		if elapsed < r.baselineWindow[namespace] {
			r.baselineRates[namespace] = append(r.baselineRates[namespace], currentRate)
			remaining := r.baselineWindow[namespace] - elapsed
			r.logger.Infof("[Traffic Death] Namespace: %s | Collecting baseline... Rate: %.2f flows/sec | Remaining: %.1f seconds", namespace, currentRate, remaining.Seconds())
			return
		}

		if len(r.baselineRates[namespace]) > 0 {
			sum := 0.0
			for _, rate := range r.baselineRates[namespace] {
				sum += rate
			}
			avgBaseline := sum / float64(len(r.baselineRates[namespace]))
			r.baseline[namespace] = avgBaseline
			r.logger.Infof("[Traffic Death] Baseline calculated for namespace %s: %.2f flows/sec (from %d samples over 1 minute)", namespace, avgBaseline, len(r.baselineRates[namespace]))
			delete(r.baselineStart, namespace)
			delete(r.baselineWindow, namespace)
			delete(r.baselineRates, namespace)
		}
		return
	}

	r.logger.Infof("[Traffic Death] Namespace: %s | Current rate: %.2f flows/sec | Baseline: %.2f flows/sec", namespace, currentRate, baseline)

	// Update baseline if it's 0 and we have current rate
	if baseline <= 0 && currentRate > 0 {
		r.baseline[namespace] = currentRate
		r.logger.Infof("[Traffic Death] Updating baseline for namespace %s: %.2f flows/sec", namespace, currentRate)
		return
	}

	// Alert if current rate is 0 and baseline is positive (traffic has died)
	if currentRate == 0.0 && baseline > 0 {
		alert := &model.Alert{
			Type:      r.name,
			Severity:  r.severity,
			Message:   fmt.Sprintf("Traffic death detected in namespace %s: No traffic (0.00 flows/sec) but baseline was %.2f flows/sec. Service may be down!", namespace, baseline),
			Timestamp: time.Now(),
		}
		r.logger.Warnf("Traffic Death Rule Alert: %s", alert.Message)
		// Emit alert through emitter function
		if r.alertEmitter != nil {
			r.alertEmitter(alert)
		}
	}
}

func (r *TrafficDeathRulePrometheus) getNamespacesFromConfig() []string {
	if len(r.namespaces) > 0 {
		return r.namespaces
	}
	return []string{"default", "kube-system"}
}
