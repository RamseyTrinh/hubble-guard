package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

// NewDestinationRule detects connections to new destinations
type NewDestinationRule struct {
	name           string
	enabled        bool
	severity       string
	knownDest      map[string]map[string]bool // namespace -> destIP -> true
	logger         *logrus.Logger
	mu             sync.RWMutex
}

// NewNewDestinationRule creates a new destination rule
func NewNewDestinationRule(enabled bool, severity string, logger *logrus.Logger) *NewDestinationRule {
	return &NewDestinationRule{
		name:      "new_destination",
		enabled:   enabled,
		severity:  severity,
		knownDest: make(map[string]map[string]bool),
		logger:    logger,
	}
}

// Name returns the rule name
func (r *NewDestinationRule) Name() string {
	return r.name
}

// IsEnabled returns whether the rule is enabled
func (r *NewDestinationRule) IsEnabled() bool {
	return r.enabled
}

// Evaluate evaluates the rule against a flow
func (r *NewDestinationRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	if !r.enabled || flow == nil || flow.IP == nil {
		return nil
	}

	namespace := "unknown"
	if flow.Source != nil && flow.Source.Namespace != "" {
		namespace = flow.Source.Namespace
	} else if flow.Destination != nil && flow.Destination.Namespace != "" {
		namespace = flow.Destination.Namespace
	}

	destIP := flow.IP.Destination

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.knownDest[namespace] == nil {
		r.knownDest[namespace] = make(map[string]bool)
	}

	// Check if this is a new destination
	if !r.knownDest[namespace][destIP] {
		r.knownDest[namespace][destIP] = true

		alert := &model.Alert{
			Type:      r.name,
			Severity:  r.severity,
			Message:   fmt.Sprintf("New destination anomaly detected in namespace %s: %s -> %s", namespace, flow.IP.Source, destIP),
			Timestamp: time.Now(),
		}

		r.logger.Warnf("New Destination Rule Alert: %s", alert.Message)
		return alert
	}

	return nil
}

