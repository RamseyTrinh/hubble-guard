package builtin

import (
	"context"
	"sync"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

// LatencyRule detects high latency in flows (placeholder - would need timing data)
type LatencyRule struct {
	name     string
	enabled  bool
	severity string
	logger   *logrus.Logger
	mu       sync.RWMutex
}

// NewLatencyRule creates a new latency rule
func NewLatencyRule(enabled bool, severity string, logger *logrus.Logger) *LatencyRule {
	return &LatencyRule{
		name:     "high_latency",
		enabled:  enabled,
		severity: severity,
		logger:   logger,
	}
}

// Name returns the rule name
func (r *LatencyRule) Name() string {
	return r.name
}

// IsEnabled returns whether the rule is enabled
func (r *LatencyRule) IsEnabled() bool {
	return r.enabled
}

// Evaluate evaluates the rule against a flow
func (r *LatencyRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	// Placeholder - would need timing/performance data from Hubble
	// For now, this is a stub that can be implemented later
	return nil
}
