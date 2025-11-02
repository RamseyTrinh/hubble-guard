package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

// ApplicationConfig cấu hình cho ứng dụng chính
type ApplicationConfig struct {
	HubbleServer     string `json:"hubble_server"`
	PrometheusPort   string `json:"prometheus_port"`
	DefaultNamespace string `json:"default_namespace"`
}

// PrometheusConfig cấu hình cho Prometheus connection
type PrometheusConfig struct {
	URL               string `json:"url"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
	RetryAttempts     int    `json:"retry_attempts"`
	RetryDelaySeconds int    `json:"retry_delay_seconds"`
}

// DetectionConfig cấu hình cho anomaly detection
type DetectionConfig struct {
	BaselineMultiplier    float64  `json:"baseline_multiplier"`
	BaselineWindowMinutes int      `json:"baseline_window_minutes"`
	CheckIntervalSeconds  int      `json:"check_interval_seconds"`
	Namespaces            []string `json:"namespaces"`
}

// TelegramConfig cấu hình cho Telegram bot
type TelegramConfig struct {
	BotToken  string `json:"bot_token"`
	ChatID    string `json:"chat_id"`
	ParseMode string `json:"parse_mode"`
	Enabled   bool   `json:"enabled"`
}

// AlertingConfig cấu hình cho alerting
type AlertingConfig struct {
	Enabled              bool `json:"enabled"`
	MaxAlertsPerMinute   int  `json:"max_alerts_per_minute"`
	AlertCooldownSeconds int  `json:"alert_cooldown_seconds"`
	Channels             struct {
		Log      bool `json:"log"`
		Webhook  bool `json:"webhook"`
		Email    bool `json:"email"`
		Telegram bool `json:"telegram"`
	} `json:"channels"`
	Telegram TelegramConfig `json:"telegram"`
}

// LoggingConfig cấu hình cho logging
type LoggingConfig struct {
	Level    string `json:"level"`
	Format   string `json:"format"`
	FilePath string `json:"file_path"`
}

// AnomalyRuleConfig represents a rule configuration
type AnomalyRuleConfig struct {
	Enabled             bool     `json:"enabled"`
	Severity            string   `json:"severity"`
	Description         string   `json:"description"`
	ThresholdMultiplier *float64 `json:"threshold_multiplier,omitempty"`
	ThresholdPerMinute  *int     `json:"threshold_per_minute,omitempty"`
}

// DefaultConfig cấu hình mặc định cho Anomaly Detector (tương thích ngược với format JSON cũ)
type DefaultConfig struct {
	Application ApplicationConfig            `json:"application"`
	Prometheus  PrometheusConfig             `json:"prometheus"`
	Namespaces  []string                     `json:"namespaces"`
	Detection   DetectionConfig              `json:"detection"`
	Rules       map[string]AnomalyRuleConfig `json:"rules"`
	Alerting    AlertingConfig               `json:"alerting"`
	Logging     LoggingConfig                `json:"logging"`
}

// LoadPrometheusConfig load cấu hình từ file JSON (backward compatibility)
func LoadPrometheusConfig(filename string) (*DefaultConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", filename, err)
	}

	var config DefaultConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", filename, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	return &config, nil
}

// Validate kiểm tra tính hợp lệ của cấu hình
func (c *DefaultConfig) Validate() error {
	if c.Application.HubbleServer == "" {
		c.Application.HubbleServer = "localhost:4245"
	}
	if c.Application.PrometheusPort == "" {
		c.Application.PrometheusPort = "8080"
	}
	if c.Application.DefaultNamespace == "" {
		c.Application.DefaultNamespace = "default"
	}

	if c.Prometheus.URL == "" {
		return fmt.Errorf("prometheus URL cannot be empty")
	}
	if c.Prometheus.TimeoutSeconds <= 0 {
		c.Prometheus.TimeoutSeconds = 10
	}
	if c.Prometheus.RetryAttempts <= 0 {
		c.Prometheus.RetryAttempts = 3
	}
	if c.Prometheus.RetryDelaySeconds <= 0 {
		c.Prometheus.RetryDelaySeconds = 5
	}

	if c.Detection.BaselineMultiplier <= 0 {
		c.Detection.BaselineMultiplier = 3.0
	}
	if c.Detection.BaselineWindowMinutes <= 0 {
		c.Detection.BaselineWindowMinutes = 5
	}
	if c.Detection.CheckIntervalSeconds <= 0 {
		c.Detection.CheckIntervalSeconds = 30
	}
	if len(c.Detection.Namespaces) == 0 {
		c.Detection.Namespaces = []string{"default"}
	}

	for ruleName, rule := range c.Rules {
		if rule.Severity == "" {
			rule.Severity = "MEDIUM"
		}
		c.Rules[ruleName] = rule
	}

	if c.Alerting.MaxAlertsPerMinute <= 0 {
		c.Alerting.MaxAlertsPerMinute = 10
	}
	if c.Alerting.AlertCooldownSeconds <= 0 {
		c.Alerting.AlertCooldownSeconds = 60
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "INFO"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}

	return nil
}

// GetDefaultConfig trả về cấu hình mặc định
func GetDefaultConfig() *DefaultConfig {
	return &DefaultConfig{
		Application: ApplicationConfig{
			HubbleServer:     "localhost:4245",
			PrometheusPort:   "8080",
			DefaultNamespace: "default",
		},
		Prometheus: PrometheusConfig{
			URL:               "http://localhost:9090",
			TimeoutSeconds:    10,
			RetryAttempts:     3,
			RetryDelaySeconds: 5,
		},
		Detection: DetectionConfig{
			BaselineMultiplier:    3.0,
			BaselineWindowMinutes: 1,
			CheckIntervalSeconds:  30,
			Namespaces:            []string{"default", "kube-system"},
		},
		Rules: map[string]AnomalyRuleConfig{
			"traffic_spike": {
				Enabled:             true,
				Severity:            "CRITICAL",
				Description:         "Phát hiện traffic spike có thể là DDoS",
				ThresholdMultiplier: &[]float64{3.0}[0],
			},
			"new_destination": {
				Enabled:     true,
				Severity:    "HIGH",
				Description: "Phát hiện kết nối đến destination mới",
			},
			"tcp_reset_surge": {
				Enabled:            true,
				Severity:           "HIGH",
				Description:        "Phát hiện surge TCP resets",
				ThresholdPerMinute: &[]int{10}[0],
			},
			"tcp_drop_surge": {
				Enabled:            true,
				Severity:           "HIGH",
				Description:        "Phát hiện surge TCP drops",
				ThresholdPerMinute: &[]int{10}[0],
			},
		},
		Alerting: AlertingConfig{
			Enabled:              true,
			MaxAlertsPerMinute:   10,
			AlertCooldownSeconds: 60,
			Channels: struct {
				Log      bool `json:"log"`
				Webhook  bool `json:"webhook"`
				Email    bool `json:"email"`
				Telegram bool `json:"telegram"`
			}{
				Log:      true,
				Webhook:  false,
				Email:    false,
				Telegram: true,
			},
			Telegram: TelegramConfig{
				BotToken:  "",
				ChatID:    "",
				ParseMode: "Markdown",
				Enabled:   false,
			},
		},
		Logging: LoggingConfig{
			Level:    "INFO",
			Format:   "json",
			FilePath: "/var/log/anomaly-detector.log",
		},
	}
}

// SaveConfig lưu cấu hình ra file
func (c *DefaultConfig) SaveConfig(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %v", filename, err)
	}

	return nil
}

// GetRuleConfig lấy cấu hình của một rule
func (c *DefaultConfig) GetRuleConfig(ruleName string) (AnomalyRuleConfig, bool) {
	rule, exists := c.Rules[ruleName]
	return rule, exists
}

// IsRuleEnabled kiểm tra xem rule có được enable không
func (c *DefaultConfig) IsRuleEnabled(ruleName string) bool {
	rule, exists := c.Rules[ruleName]
	return exists && rule.Enabled
}

// GetNamespaceList trả về danh sách namespaces
func (c *DefaultConfig) GetNamespaceList() []string {
	return c.Detection.Namespaces
}

// AddNamespace thêm namespace mới
func (c *DefaultConfig) AddNamespace(namespace string) {
	for _, ns := range c.Detection.Namespaces {
		if ns == namespace {
			return
		}
	}
	c.Detection.Namespaces = append(c.Detection.Namespaces, namespace)
}

// RemoveNamespace xóa namespace
func (c *DefaultConfig) RemoveNamespace(namespace string) {
	for i, ns := range c.Detection.Namespaces {
		if ns == namespace {
			c.Detection.Namespaces = append(c.Detection.Namespaces[:i], c.Detection.Namespaces[i+1:]...)
			break
		}
	}
}
