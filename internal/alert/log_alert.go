package alert

import (
	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

// LogAlertNotifier sends alerts to local logs
type LogAlertNotifier struct {
	logger *logrus.Logger
}

// NewLogAlertNotifier creates a new log alert notifier
func NewLogAlertNotifier(logger *logrus.Logger) *LogAlertNotifier {
	return &LogAlertNotifier{
		logger: logger,
	}
}

// SendAlert implements Notifier interface - sends alert to logs
func (ln *LogAlertNotifier) SendAlert(alert model.Alert) error {
	ln.logger.Warnf("ALERT [%s] %s: %s", alert.Severity, alert.Type, alert.Message)
	return nil
}

