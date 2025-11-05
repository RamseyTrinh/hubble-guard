package rules

import (
	"context"
	"sync"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

type Engine struct {
	rules          []RuleInterface
	alertNotifiers []NotifierInterface
	logger         *logrus.Logger
	mu             sync.RWMutex
	alertChannel   chan model.Alert
}

type NotifierInterface interface {
	SendAlert(alert model.Alert) error
}

func NewEngine(logger *logrus.Logger) *Engine {
	return &Engine{
		rules:          make([]RuleInterface, 0),
		alertNotifiers: make([]NotifierInterface, 0),
		logger:         logger,
		alertChannel:   make(chan model.Alert, 100),
	}
}

func (e *Engine) RegisterRule(rule RuleInterface) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
	e.logger.Infof("Registered rule: %s", rule.Name())
}

func (e *Engine) RegisterNotifier(notifier NotifierInterface) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alertNotifiers = append(e.alertNotifiers, notifier)
}

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
				e.EmitAlert(*alert)
			}
		}
	}

	return alerts
}

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

func (e *Engine) GetAlertChannel() <-chan model.Alert {
	return e.alertChannel
}

type RuleInterface interface {
	Name() string
	IsEnabled() bool
	Evaluate(ctx context.Context, flow *model.Flow) *model.Alert
}
