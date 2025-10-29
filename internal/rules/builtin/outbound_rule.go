package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

// OutboundRule detects suspicious outbound connections
type OutboundRule struct {
	name            string
	enabled         bool
	severity        string
	suspiciousPorts map[int]bool
	portCounts      map[string]map[int]int64 // namespace -> port -> count
	lastReset       map[string]time.Time
	threshold       int64
	logger          *logrus.Logger
	mu              sync.RWMutex
}

// NewOutboundRule creates a new outbound rule
func NewOutboundRule(enabled bool, severity string, threshold int64, logger *logrus.Logger) *OutboundRule {
	if threshold <= 0 {
		threshold = 10 // default 10 connections per minute
	}

	suspiciousPorts := map[int]bool{
		22:   false, // SSH
		23:   true,  // Telnet
		135:  true,  // MS-RPC
		445:  true,  // SMB
		1433: true,  // SQL Server
		3306: true,  // MySQL
		5432: true,  // PostgreSQL
	}

	return &OutboundRule{
		name:            "suspicious_outbound",
		enabled:         enabled,
		severity:        severity,
		suspiciousPorts: suspiciousPorts,
		portCounts:      make(map[string]map[int]int64),
		lastReset:       make(map[string]time.Time),
		threshold:       threshold,
		logger:          logger,
	}
}

// Name returns the rule name
func (r *OutboundRule) Name() string {
	return r.name
}

// IsEnabled returns whether the rule is enabled
func (r *OutboundRule) IsEnabled() bool {
	return r.enabled
}

// Evaluate evaluates the rule against a flow
func (r *OutboundRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	if !r.enabled || flow == nil || flow.L4 == nil {
		return nil
	}

	var destPort int
	if flow.L4.TCP != nil {
		destPort = int(flow.L4.TCP.DestinationPort)
	} else if flow.L4.UDP != nil {
		destPort = int(flow.L4.UDP.DestinationPort)
	} else {
		return nil
	}

	// Check if port is marked as suspicious
	if !r.suspiciousPorts[destPort] {
		return nil
	}

	namespace := "unknown"
	if flow.Source != nil && flow.Source.Namespace != "" {
		namespace = flow.Source.Namespace
	} else if flow.Destination != nil && flow.Destination.Namespace != "" {
		namespace = flow.Destination.Namespace
	}

	r.mu.Lock()
	if r.portCounts[namespace] == nil {
		r.portCounts[namespace] = make(map[int]int64)
	}
	r.portCounts[namespace][destPort]++
	lastReset, exists := r.lastReset[namespace]
	if !exists {
		lastReset = time.Now()
		r.lastReset[namespace] = lastReset
	}
	r.mu.Unlock()

	// Check every minute
	now := time.Now()
	if now.Sub(lastReset) >= 1*time.Minute {
		alert := r.checkSuspiciousPorts(namespace)
		r.mu.Lock()
		r.portCounts[namespace] = make(map[int]int64)
		r.lastReset[namespace] = now
		r.mu.Unlock()
		if alert != nil {
			return alert
		}
	}

	return nil
}

func (r *OutboundRule) checkSuspiciousPorts(namespace string) *model.Alert {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for port, count := range r.portCounts[namespace] {
		if count > r.threshold {
			alert := &model.Alert{
				Type:      r.name,
				Severity:  r.severity,
				Message:   fmt.Sprintf("Suspicious outbound connection detected in namespace %s: %d connections to port %d in 1 minute (threshold: %d)", namespace, count, port, r.threshold),
				Timestamp: time.Now(),
			}

			r.logger.Warnf("Outbound Rule Alert: %s", alert.Message)
			return alert
		}
	}
	return nil
}
