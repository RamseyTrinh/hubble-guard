package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

// TCPDropRule detects TCP drop surges
type TCPDropRule struct {
	name       string
	enabled    bool
	severity   string
	threshold  int              // drops per minute
	dropCounts map[string]int64 // namespace -> count
	lastReset  map[string]time.Time
	logger     *logrus.Logger
	mu         sync.RWMutex
}

// NewTCPDropRule creates a new TCP drop rule
func NewTCPDropRule(enabled bool, severity string, threshold int, logger *logrus.Logger) *TCPDropRule {
	if threshold <= 0 {
		threshold = 10 // default 10 drops per minute
	}
	return &TCPDropRule{
		name:       "tcp_drop_surge",
		enabled:    enabled,
		severity:   severity,
		threshold:  threshold,
		dropCounts: make(map[string]int64),
		lastReset:  make(map[string]time.Time),
		logger:     logger,
	}
}

// Name returns the rule name
func (r *TCPDropRule) Name() string {
	return r.name
}

// IsEnabled returns whether the rule is enabled
func (r *TCPDropRule) IsEnabled() bool {
	return r.enabled
}

// Evaluate evaluates the rule against a flow
func (r *TCPDropRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	if !r.enabled || flow == nil {
		return nil
	}

	// Check for TCP drop (verdict DROPPED)
	if flow.Verdict != model.Verdict_DROPPED {
		return nil
	}

	namespace := "unknown"
	if flow.Source != nil && flow.Source.Namespace != "" {
		namespace = flow.Source.Namespace
	} else if flow.Destination != nil && flow.Destination.Namespace != "" {
		namespace = flow.Destination.Namespace
	}

	r.mu.Lock()
	r.dropCounts[namespace]++
	lastReset, exists := r.lastReset[namespace]
	if !exists {
		lastReset = time.Now()
		r.lastReset[namespace] = lastReset
	}
	r.mu.Unlock()

	// Check every minute
	now := time.Now()
	if now.Sub(lastReset) >= 1*time.Minute {
		alert := r.checkDropSurge(namespace)
		r.mu.Lock()
		r.dropCounts[namespace] = 0
		r.lastReset[namespace] = now
		r.mu.Unlock()
		if alert != nil {
			return alert
		}
	}

	return nil
}

func (r *TCPDropRule) checkDropSurge(namespace string) *model.Alert {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := r.dropCounts[namespace]
	if count > int64(r.threshold) {
		alert := &model.Alert{
			Type:      r.name,
			Severity:  r.severity,
			Message:   fmt.Sprintf("TCP drop surge detected in namespace %s: %d drops in 1 minute (threshold: %d)", namespace, count, r.threshold),
			Timestamp: time.Now(),
		}

		r.logger.Warnf("TCP Drop Rule Alert: %s", alert.Message)
		return alert
	}
	return nil
}
