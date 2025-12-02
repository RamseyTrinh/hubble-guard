package alert

import (
	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

type LogAlertNotifier struct {
	logger *logrus.Logger
}

func NewLogAlertNotifier(logger *logrus.Logger) *LogAlertNotifier {
	return &LogAlertNotifier{
		logger: logger,
	}
}

func (ln *LogAlertNotifier) SendAlert(alert model.Alert) error {
	ln.logger.Warnf("ALERT [%s] %s: %s", alert.Severity, alert.Type, alert.Message)
	return nil
}
