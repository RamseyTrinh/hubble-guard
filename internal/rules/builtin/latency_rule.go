package builtin

import (
	"context"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

type LatencyRule struct {
	name     string
	enabled  bool
	severity string
	logger   *logrus.Logger
}

func NewLatencyRule(enabled bool, severity string, logger *logrus.Logger) *LatencyRule {
	return &LatencyRule{
		name:     "high_latency",
		enabled:  enabled,
		severity: severity,
		logger:   logger,
	}
}

func (r *LatencyRule) Name() string {
	return r.name
}

func (r *LatencyRule) IsEnabled() bool {
	return r.enabled
}

func (r *LatencyRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	return nil
}
