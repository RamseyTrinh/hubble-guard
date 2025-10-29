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

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

// TelegramNotifier xử lý việc gửi thông báo qua Telegram
type TelegramNotifier struct {
	botToken        string
	chatID          string
	parseMode       string
	enabled         bool
	messageTemplate *template.Template
	client          *http.Client
	logger          *logrus.Logger
}

// TelegramMessage cấu trúc message gửi đến Telegram API
type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// TelegramResponse cấu trúc response từ Telegram API
type TelegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

// NewTelegramNotifier tạo instance mới của TelegramNotifier
func NewTelegramNotifier(botToken, chatID, parseMode string, enabled bool, logger *logrus.Logger) *TelegramNotifier {
	return NewTelegramNotifierWithTemplate(botToken, chatID, parseMode, enabled, "", logger)
}

// NewTelegramNotifierWithTemplate tạo instance mới với message template
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

	// Parse message template nếu có
	if messageTemplate != "" && strings.TrimSpace(messageTemplate) != "" {
		// Thêm custom functions cho template
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

// SendAlert implements Notifier interface - gửi alert qua Telegram với retry logic
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

// formatAlertMessage format alert thành message cho Telegram
func (tn *TelegramNotifier) formatAlertMessage(alert model.Alert) string {
	// Nếu có template, sử dụng template
	if tn.messageTemplate != nil {
		var buf bytes.Buffer
		err := tn.messageTemplate.Execute(&buf, alert)
		if err != nil {
			tn.logger.Warnf("Failed to execute message template: %v, using default format", err)
		} else {
			return buf.String()
		}
	}

	// Format mặc định
	timestamp := alert.Timestamp.Format("2006-01-02 15:04:05")
	message := fmt.Sprintf("🚨 *%s Alert*\n\n*Type:* %s\n*Time:* %s\n*Message:* %s",
		alert.Severity,
		alert.Type,
		timestamp,
		alert.Message)

	// Thêm thông tin flow nếu có
	if alert.FlowData != nil {
		if alert.FlowData.Source != nil {
			message += fmt.Sprintf("\n*Source:* %s/%s", alert.FlowData.Source.Namespace, alert.FlowData.Source.PodName)
		}
		if alert.FlowData.Destination != nil {
			message += fmt.Sprintf("\n*Destination:* %s/%s", alert.FlowData.Destination.Namespace, alert.FlowData.Destination.PodName)
		}
	}

	return message
}

// sendMessage gửi message đến Telegram API
func (tn *TelegramNotifier) sendMessage(text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tn.botToken)

	message := TelegramMessage{
		ChatID:    tn.chatID,
		Text:      text,
		ParseMode: tn.parseMode,
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

// SendTestMessage gửi message test để kiểm tra kết nối
func (tn *TelegramNotifier) SendTestMessage() error {
	if !tn.enabled {
		return fmt.Errorf("telegram notifier is disabled")
	}

	message := "🤖 Test Message\n\nAnomaly Detector is working correctly!"
	return tn.sendMessage(message)
}

// IsEnabled kiểm tra xem Telegram notifier có được enable không
func (tn *TelegramNotifier) IsEnabled() bool {
	return tn.enabled
}

// UpdateConfig cập nhật cấu hình Telegram
func (tn *TelegramNotifier) UpdateConfig(botToken, chatID, parseMode string, enabled bool) {
	tn.botToken = botToken
	tn.chatID = chatID
	tn.parseMode = parseMode
	tn.enabled = enabled
	tn.logger.Infof("Telegram notifier config updated: enabled=%v", enabled)
}
