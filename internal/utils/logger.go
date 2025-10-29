package utils

import (
	"github.com/sirupsen/logrus"
)

// NewLogger creates a new logger instance
func NewLogger(level string) *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	if level != "" {
		switch level {
		case "DEBUG":
			logger.SetLevel(logrus.DebugLevel)
		case "INFO":
			logger.SetLevel(logrus.InfoLevel)
		case "WARN":
			logger.SetLevel(logrus.WarnLevel)
		case "ERROR":
			logger.SetLevel(logrus.ErrorLevel)
		}
	}

	return logger
}

