package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"

	"hubble-guard/internal/model"

	"github.com/sirupsen/logrus"
)

type TelegramNotifier struct {
	botToken        string
	chatID          string
	parseMode       string
	enabled         bool
	messageTemplate *template.Template
	client          *http.Client
	logger          *logrus.Logger
}

type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type TelegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

func NewTelegramNotifier(botToken, chatID, parseMode string, enabled bool, logger *logrus.Logger) *TelegramNotifier {
	return NewTelegramNotifierWithTemplate(botToken, chatID, parseMode, enabled, "", logger)
}

func NewTelegramNotifierWithTemplate(botToken, chatID, parseMode string, enabled bool, messageTemplate string, logger *logrus.Logger) *TelegramNotifier {
	tn := &TelegramNotifier{
		botToken:  botToken,
		chatID:    chatID,
		parseMode: parseMode,
		enabled:   enabled,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}

	if messageTemplate != "" && strings.TrimSpace(messageTemplate) != "" {
		funcMap := template.FuncMap{
			"formatTime": func(t time.Time, layout string) string {
				return t.Format(layout)
			},
		}
		tmpl, err := template.New("telegram_message").Funcs(funcMap).Parse(messageTemplate)
		if err != nil {
			logger.Warnf("Failed to parse Telegram message template: %v, using default format", err)
		} else {
			tn.messageTemplate = tmpl
		}
	}

	return tn
}

func (tn *TelegramNotifier) SendAlert(alert model.Alert) error {
	if !tn.enabled {
		tn.logger.Debug("Telegram notifier is disabled, skipping alert")
		return nil
	}

	message := tn.formatAlertMessage(alert)

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		err := tn.sendMessage(message)
		if err == nil {
			return nil
		}

		tn.logger.Warnf("Failed to send alert (attempt %d/%d): %v", i+1, maxRetries, err)

		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	return fmt.Errorf("failed to send alert after %d attempts", maxRetries)
}

func (tn *TelegramNotifier) formatAlertMessage(alert model.Alert) string {
	if tn.messageTemplate != nil {
		var buf bytes.Buffer
		err := tn.messageTemplate.Execute(&buf, alert)
		if err != nil {
			tn.logger.Warnf("Failed to execute message template: %v, using default format", err)
		} else {
			return buf.String()
		}
	}

	// Format timestamp
	timestamp := alert.Timestamp.Format("2006-01-02 15:04:05")

	// Get namespace - prefer from alert, fallback to FlowData
	namespace := alert.Namespace
	if namespace == "" && alert.FlowData != nil {
		if alert.FlowData.Source != nil && alert.FlowData.Source.Namespace != "" {
			namespace = alert.FlowData.Source.Namespace
		} else if alert.FlowData.Destination != nil && alert.FlowData.Destination.Namespace != "" {
			namespace = alert.FlowData.Destination.Namespace
		}
	}
	if namespace == "" {
		namespace = "unknown"
	}

	// Format message according to user's requirement
	message := fmt.Sprintf("ALERT FIRING: Anomaly Detect\n\n"+
		"alert_name: %s\n"+
		"time: %s\n"+
		"severity: %s\n"+
		"namespace: %s\n"+
		"description: %s",
		alert.Type,
		timestamp,
		alert.Severity,
		namespace,
		alert.Message)

	return message
}

func (tn *TelegramNotifier) sendMessage(text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tn.botToken)

	// Use empty parse_mode to avoid parsing errors with special characters
	parseMode := ""
	if tn.parseMode != "" && tn.parseMode != "Markdown" && tn.parseMode != "MarkdownV2" {
		parseMode = tn.parseMode
	}

	message := TelegramMessage{
		ChatID:    tn.chatID,
		Text:      text,
		ParseMode: parseMode,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := tn.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	var telegramResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("telegram API error: %s", telegramResp.Description)
	}

	tn.logger.Infof("Alert sent to Telegram successfully")
	return nil
}

func (tn *TelegramNotifier) SendTestMessage() error {
	if !tn.enabled {
		return fmt.Errorf("telegram notifier is disabled")
	}

	message := "Test Message\n\nAnomaly Detector is working correctly!"
	return tn.sendMessage(message)
}

func (tn *TelegramNotifier) IsEnabled() bool {
	return tn.enabled
}

func (tn *TelegramNotifier) UpdateConfig(botToken, chatID, parseMode string, enabled bool) {
	tn.botToken = botToken
	tn.chatID = chatID
	tn.parseMode = parseMode
	tn.enabled = enabled
	tn.logger.Infof("Telegram notifier config updated: enabled=%v", enabled)
}
