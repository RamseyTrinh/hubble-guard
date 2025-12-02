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

type TrafficSpikeRule struct {
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

type PrometheusQueryClient interface {
	Query(ctx context.Context, query string, timeout time.Duration) (prommodel.Value, error)
}

func NewTrafficSpikeRule(enabled bool, severity string, threshold float64, promClient PrometheusQueryClient, logger *logrus.Logger) *TrafficSpikeRule {
	if threshold <= 0 {
		threshold = 3.0
	}
	return &TrafficSpikeRule{
		name:           "traffic_spike",
		enabled:        enabled,
		severity:       severity,
		threshold:      threshold,
		prometheusAPI:  promClient,
		baseline:       make(map[string]float64),
		baselineWindow: make(map[string]time.Duration),
		baselineStart:  make(map[string]time.Time),
		baselineRates:  make(map[string][]float64),
		logger:         logger,
		interval:       10 * time.Second,
		stopChan:       make(chan struct{}),
		namespaces:     []string{"default"},
	}
}

func (r *TrafficSpikeRule) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

func (r *TrafficSpikeRule) SetNamespaces(namespaces []string) {
	if len(namespaces) == 0 {
		r.namespaces = []string{"default"}
	} else {
		r.namespaces = namespaces
	}
}

func (r *TrafficSpikeRule) Name() string {
	return r.name
}

func (r *TrafficSpikeRule) IsEnabled() bool {
	return r.enabled
}

func (r *TrafficSpikeRule) Start(ctx context.Context) {
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

func (r *TrafficSpikeRule) Stop() {
	close(r.stopChan)
}

func (r *TrafficSpikeRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	return nil
}

func (r *TrafficSpikeRule) checkFromPrometheus(ctx context.Context) {
	for _, namespace := range r.namespaces {
		r.checkNamespace(ctx, namespace)
	}
}

func (r *TrafficSpikeRule) checkNamespace(ctx context.Context, namespace string) {
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

	baseline, exists := r.baseline[namespace]
	if !exists {
		baselineStart, baselineStarted := r.baselineStart[namespace]
		if !baselineStarted {
			r.baselineStart[namespace] = time.Now()
			r.baselineWindow[namespace] = 1 * time.Minute
			r.baselineRates[namespace] = []float64{currentRate}
			r.logger.Infof("[Traffic Spike] Namespace: %s | Starting baseline collection (1 minute) | Rate: %.2f flows/sec", namespace, currentRate)
			return
		}

		now := time.Now()
		elapsed := now.Sub(baselineStart)
		if elapsed < r.baselineWindow[namespace] {
			r.baselineRates[namespace] = append(r.baselineRates[namespace], currentRate)
			remaining := r.baselineWindow[namespace] - elapsed
			r.logger.Infof("[Traffic Spike] Namespace: %s | Collecting baseline... Rate: %.2f flows/sec | Remaining: %.1f seconds", namespace, currentRate, remaining.Seconds())
			return
		}

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
			Namespace: namespace,
			Message:   fmt.Sprintf("Traffic spike detected in namespace %s: %.2fx baseline (%.2f flows/sec vs %.2f baseline)", namespace, multiplier, currentRate, baseline),
			Timestamp: time.Now(),
		}
		r.logger.Warnf("Traffic Spike Rule Alert: %s", alert.Message)
		if r.alertEmitter != nil {
			r.alertEmitter(alert)
		}
	}
}
