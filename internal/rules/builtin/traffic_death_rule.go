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

type TrafficDeathRule struct {
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

func NewTrafficDeathRule(enabled bool, severity string, promClient PrometheusQueryClient, logger *logrus.Logger) *TrafficDeathRule {
	return &TrafficDeathRule{
		name:           "traffic_death",
		enabled:        enabled,
		severity:       severity,
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

func (r *TrafficDeathRule) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

func (r *TrafficDeathRule) SetNamespaces(namespaces []string) {
	if len(namespaces) == 0 {
		r.namespaces = []string{"default"}
	} else {
		r.namespaces = namespaces
	}
}

func (r *TrafficDeathRule) Name() string {
	return r.name
}

func (r *TrafficDeathRule) IsEnabled() bool {
	return r.enabled
}

func (r *TrafficDeathRule) Start(ctx context.Context) {
	if !r.enabled {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkFromPrometheus(ctx)
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		}
	}
}

func (r *TrafficDeathRule) Stop() {
	close(r.stopChan)
}

func (r *TrafficDeathRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	return nil
}

func (r *TrafficDeathRule) checkFromPrometheus(ctx context.Context) {
	for _, namespace := range r.namespaces {
		r.checkNamespace(ctx, namespace)
	}
}

func (r *TrafficDeathRule) checkNamespace(ctx context.Context, namespace string) {
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
			return
		}

		now := time.Now()
		elapsed := now.Sub(baselineStart)
		if elapsed < r.baselineWindow[namespace] {
			r.baselineRates[namespace] = append(r.baselineRates[namespace], currentRate)
			return
		}

		if len(r.baselineRates[namespace]) > 0 {
			sum := 0.0
			for _, rate := range r.baselineRates[namespace] {
				sum += rate
			}
			avgBaseline := sum / float64(len(r.baselineRates[namespace]))
			r.baseline[namespace] = avgBaseline
			delete(r.baselineStart, namespace)
			delete(r.baselineWindow, namespace)
			delete(r.baselineRates, namespace)
		}
		return
	}

	if baseline <= 0 && currentRate > 0 {
		r.baseline[namespace] = currentRate
		return
	}

	if currentRate == 0.0 && baseline > 0 {
		alert := &model.Alert{
			Type:      r.name,
			Severity:  r.severity,
			Message:   fmt.Sprintf("Traffic death detected in namespace %s: No traffic (0.00 flows/sec) but baseline was %.2f flows/sec. Service may be down!", namespace, baseline),
			Timestamp: time.Now(),
		}
		r.logger.Warnf("Traffic Death Rule Alert: %s", alert.Message)
		if r.alertEmitter != nil {
			r.alertEmitter(alert)
		}
	}
}
