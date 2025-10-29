package rules

import (
	"context"
	"sync"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

// Engine evaluates rules against flows and emits alerts
type Engine struct {
	rules          []RuleInterface
	alertNotifiers []NotifierInterface
	logger         *logrus.Logger
	mu             sync.RWMutex
	alertChannel   chan model.Alert
}

// NotifierInterface defines the interface for alert notification
type NotifierInterface interface {
	SendAlert(alert model.Alert) error
}

// NewEngine creates a new rule engine
func NewEngine(logger *logrus.Logger) *Engine {
	return &Engine{
		rules:          make([]RuleInterface, 0),
		alertNotifiers: make([]NotifierInterface, 0),
		logger:         logger,
		alertChannel:   make(chan model.Alert, 100),
	}
}

// RegisterRule registers a rule with the engine
func (e *Engine) RegisterRule(rule RuleInterface) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
	e.logger.Infof("Registered rule: %s", rule.Name())
}

// RegisterNotifier registers an alert notifier
func (e *Engine) RegisterNotifier(notifier NotifierInterface) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alertNotifiers = append(e.alertNotifiers, notifier)
}

// Evaluate evaluates all registered rules against a flow and emits alerts
func (e *Engine) Evaluate(ctx context.Context, flow *model.Flow) []model.Alert {
	e.mu.RLock()
	rules := make([]RuleInterface, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	var alerts []model.Alert

	for _, rule := range rules {
		if rule.IsEnabled() {
			if alert := rule.Evaluate(ctx, flow); alert != nil {
				alerts = append(alerts, *alert)
				// Emit alert immediately
				e.EmitAlert(*alert)
			}
		}
	}

	return alerts
}

// EmitAlert sends an alert to all registered notifiers
func (e *Engine) EmitAlert(alert model.Alert) {
	select {
	case e.alertChannel <- alert:
	default:
		e.logger.Error("Alert channel is full, dropping alert")
	}

	e.mu.RLock()
	notifiers := make([]NotifierInterface, len(e.alertNotifiers))
	copy(notifiers, e.alertNotifiers)
	e.mu.RUnlock()

	for _, notifier := range notifiers {
		if err := notifier.SendAlert(alert); err != nil {
			e.logger.Errorf("Failed to send alert: %v", err)
		}
	}
}

// GetAlertChannel returns the alert channel
func (e *Engine) GetAlertChannel() <-chan model.Alert {
	return e.alertChannel
}

// RuleInterface defines the interface for rules
type RuleInterface interface {
	Name() string
	IsEnabled() bool
	Evaluate(ctx context.Context, flow *model.Flow) *model.Alert
}
