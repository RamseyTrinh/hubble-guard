package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

type DDoSRule struct {
	name           string
	enabled        bool
	severity       string
	threshold      float64 // Multiplier threshold (e.g., 3.0x baseline)
	flowCounts     map[string]int64
	baseline       map[string]float64
	lastReset      map[string]time.Time
	baselineStart  map[string]time.Time
	logger         *logrus.Logger
	mu             sync.RWMutex
	window         time.Duration // Time window for counting flows (e.g., 1 minute)
	baselineWindow time.Duration // Time window for baseline calculation (e.g., 5 minutes)
	alertEmitter   func(*model.Alert)
}

func NewDDoSRule(enabled bool, severity string, threshold float64, logger *logrus.Logger) *DDoSRule {
	if threshold <= 0 {
		threshold = 3.0
	}
	return &DDoSRule{
		name:           "ddos",
		enabled:        enabled,
		severity:       severity,
		threshold:      threshold,
		flowCounts:     make(map[string]int64),
		baseline:       make(map[string]float64),
		lastReset:      make(map[string]time.Time),
		baselineStart:  make(map[string]time.Time),
		logger:         logger,
		window:         1 * time.Minute,
		baselineWindow: 5 * time.Minute,
	}
}

func (r *DDoSRule) Name() string {
	return r.name
}

func (r *DDoSRule) IsEnabled() bool {
	return r.enabled
}

func (r *DDoSRule) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

func (r *DDoSRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	if !r.enabled || flow == nil {
		return nil
	}

	namespace := "unknown"
	if flow.Source != nil && flow.Source.Namespace != "" {
		namespace = flow.Source.Namespace
	} else if flow.Destination != nil && flow.Destination.Namespace != "" {
		namespace = flow.Destination.Namespace
	}

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize baseline collection if needed
	baselineStart, baselineExists := r.baselineStart[namespace]
	if !baselineExists {
		r.baselineStart[namespace] = now
		r.flowCounts[namespace] = 1
		r.logger.Debugf("[DDoS] Namespace: %s | Starting baseline collection", namespace)
		return nil
	}

	// Collect baseline data
	baselineElapsed := now.Sub(baselineStart)
	if baselineElapsed < r.baselineWindow {
		r.flowCounts[namespace]++
		r.logger.Debugf("[DDoS] Namespace: %s | Collecting baseline... Flows: %d | Elapsed: %.1fs",
			namespace, r.flowCounts[namespace], baselineElapsed.Seconds())
		return nil
	}

	// Calculate baseline if window is complete
	if _, exists := r.baseline[namespace]; !exists {
		baselineRate := float64(r.flowCounts[namespace]) / r.baselineWindow.Minutes()
		r.baseline[namespace] = baselineRate
		r.flowCounts[namespace] = 1
		r.lastReset[namespace] = now
		r.logger.Infof("[DDoS] Namespace: %s | Baseline calculated: %.2f flows/min", namespace, baselineRate)
		return nil
	}

	// Count flows in current window
	lastReset, resetExists := r.lastReset[namespace]
	if !resetExists {
		r.lastReset[namespace] = now
		r.flowCounts[namespace] = 1
		return nil
	}

	r.flowCounts[namespace]++

	// Check if window has elapsed
	elapsed := now.Sub(lastReset)
	if elapsed >= r.window {
		alert := r.checkDDoSAttack(namespace, elapsed)
		r.flowCounts[namespace] = 0
		r.lastReset[namespace] = now
		return alert
	}

	return nil
}

func (r *DDoSRule) checkDDoSAttack(namespace string, elapsed time.Duration) *model.Alert {
	baseline, exists := r.baseline[namespace]
	if !exists || baseline <= 0 {
		// Update baseline if we have data
		if r.flowCounts[namespace] > 0 {
			currentRate := float64(r.flowCounts[namespace]) / elapsed.Minutes()
			r.baseline[namespace] = currentRate
			r.logger.Debugf("[DDoS] Namespace: %s | Updating baseline: %.2f flows/min", namespace, currentRate)
		}
		return nil
	}

	currentRate := float64(r.flowCounts[namespace]) / elapsed.Minutes()
	multiplier := currentRate / baseline

	r.logger.Debugf("[DDoS] Namespace: %s | Current: %.2f flows/min | Baseline: %.2f flows/min | Multiplier: %.2fx",
		namespace, currentRate, baseline, multiplier)

	if multiplier > r.threshold {
		alert := &model.Alert{
			Type:      r.name,
			Severity:  r.severity,
			Namespace: namespace,
			Message:   fmt.Sprintf("DDoS attack detected in namespace %s: %.2fx baseline (%.2f flows/min vs %.2f baseline/min)", namespace, multiplier, currentRate, baseline),
			Timestamp: time.Now(),
		}

		r.logger.Warnf("DDoS Rule Alert: %s", alert.Message)
		if r.alertEmitter != nil {
			r.alertEmitter(alert)
		}
		return alert
	}

	return nil
}
