package alert

import "hubble-anomaly-detector/internal/model"

// Notifier interface for alert notification
type Notifier interface {
	SendAlert(alert model.Alert) error
}
