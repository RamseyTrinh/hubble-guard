package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

// NamespaceAccessRule detects unauthorized cross-namespace access
type NamespaceAccessRule struct {
	name          string
	enabled       bool
	severity      string
	allowedNS     map[string]map[string]bool // source namespace -> allowed destination namespaces
	logger        *logrus.Logger
	mu            sync.RWMutex
}

// NewNamespaceAccessRule creates a new namespace access rule
func NewNamespaceAccessRule(enabled bool, severity string, logger *logrus.Logger) *NamespaceAccessRule {
	return &NamespaceAccessRule{
		name:      "unauthorized_namespace_access",
		enabled:   enabled,
		severity: severity,
		allowedNS: make(map[string]map[string]bool),
		logger:    logger,
	}
}

// Name returns the rule name
func (r *NamespaceAccessRule) Name() string {
	return r.name
}

// IsEnabled returns whether the rule is enabled
func (r *NamespaceAccessRule) IsEnabled() bool {
	return r.enabled
}

// Evaluate evaluates the rule against a flow
func (r *NamespaceAccessRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	if !r.enabled || flow == nil || flow.Source == nil || flow.Destination == nil {
		return nil
	}

	sourceNS := flow.Source.Namespace
	destNS := flow.Destination.Namespace

	// Allow same namespace
	if sourceNS == destNS || sourceNS == "" || destNS == "" {
		return nil
	}

	r.mu.RLock()
	allowed, exists := r.allowedNS[sourceNS]
	r.mu.RUnlock()

	// If no rules defined, allow all (permissive mode)
	if !exists {
		return nil
	}

	// Check if destination namespace is allowed
	if !allowed[destNS] {
		alert := &model.Alert{
			Type:      r.name,
			Severity:  r.severity,
			Message:   fmt.Sprintf("Unauthorized cross-namespace access detected: %s -> %s", sourceNS, destNS),
			Timestamp: time.Now(),
		}

		r.logger.Warnf("Namespace Access Rule Alert: %s", alert.Message)
		return alert
	}

	return nil
}

// SetAllowedNamespaces sets allowed destination namespaces for a source namespace
func (r *NamespaceAccessRule) SetAllowedNamespaces(sourceNS string, allowedNS []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.allowedNS[sourceNS] = make(map[string]bool)
	for _, ns := range allowedNS {
		r.allowedNS[sourceNS][ns] = true
	}
}

