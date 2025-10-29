package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

// TCPResetRule detects TCP reset surges
type TCPResetRule struct {
	name        string
	enabled     bool
	severity    string
	threshold   int              // resets per minute
	resetCounts map[string]int64 // namespace -> count
	lastReset   map[string]time.Time
	logger      *logrus.Logger
	mu          sync.RWMutex
}

// NewTCPResetRule creates a new TCP reset rule
func NewTCPResetRule(enabled bool, severity string, threshold int, logger *logrus.Logger) *TCPResetRule {
	if threshold <= 0 {
		threshold = 10 // default 10 resets per minute
	}
	return &TCPResetRule{
		name:        "tcp_reset_surge",
		enabled:     enabled,
		severity:    severity,
		threshold:   threshold,
		resetCounts: make(map[string]int64),
		lastReset:   make(map[string]time.Time),
		logger:      logger,
	}
}

// Name returns the rule name
func (r *TCPResetRule) Name() string {
	return r.name
}

// IsEnabled returns whether the rule is enabled
func (r *TCPResetRule) IsEnabled() bool {
	return r.enabled
}

// Evaluate evaluates the rule against a flow
func (r *TCPResetRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	if !r.enabled || flow == nil {
		return nil
	}

	// Check for TCP reset
	if flow.L4 == nil || flow.L4.TCP == nil || flow.L4.TCP.Flags == nil || !flow.L4.TCP.Flags.RST {
		return nil
	}

	namespace := "unknown"
	if flow.Source != nil && flow.Source.Namespace != "" {
		namespace = flow.Source.Namespace
	} else if flow.Destination != nil && flow.Destination.Namespace != "" {
		namespace = flow.Destination.Namespace
	}

	r.mu.Lock()
	r.resetCounts[namespace]++
	lastReset, exists := r.lastReset[namespace]
	if !exists {
		lastReset = time.Now()
		r.lastReset[namespace] = lastReset
	}
	r.mu.Unlock()

	// Check every minute
	now := time.Now()
	if now.Sub(lastReset) >= 1*time.Minute {
		alert := r.checkResetSurge(namespace)
		r.mu.Lock()
		r.resetCounts[namespace] = 0
		r.lastReset[namespace] = now
		r.mu.Unlock()
		if alert != nil {
			return alert
		}
	}

	return nil
}

func (r *TCPResetRule) checkResetSurge(namespace string) *model.Alert {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := r.resetCounts[namespace]
	if count > int64(r.threshold) {
		alert := &model.Alert{
			Type:      r.name,
			Severity:  r.severity,
			Message:   fmt.Sprintf("TCP reset surge detected in namespace %s: %d resets in 1 minute (threshold: %d)", namespace, count, r.threshold),
			Timestamp: time.Now(),
		}

		r.logger.Warnf("TCP Reset Rule Alert: %s", alert.Message)
		return alert
	}
	return nil
}
