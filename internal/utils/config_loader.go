package utils

import (
	"context"
	"fmt"
	"os"
	"strings"

	"hubble-anomaly-detector/internal/client"
	"hubble-anomaly-detector/internal/model"
	"hubble-anomaly-detector/internal/rules"
	"hubble-anomaly-detector/internal/rules/builtin"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// AnomalyDetectionConfig represents the complete configuration structure
type AnomalyDetectionConfig struct {
	Application ApplicationYAMLConfig `yaml:"application"`
	Prometheus  PrometheusYAMLConfig  `yaml:"prometheus"`
	Namespaces  []string              `yaml:"namespaces"`
	Detection   DetectionYAMLConfig   `yaml:"detection"`
	Rules       []model.Rule          `yaml:"rules"`
	Alerting    AlertingYAMLConfig    `yaml:"alerting"`
	Logging     LoggingYAMLConfig     `yaml:"logging"`
}

// ApplicationYAMLConfig cấu hình ứng dụng từ YAML
type ApplicationYAMLConfig struct {
	HubbleServer        string `yaml:"hubble_server"`
	PrometheusExportURL string `yaml:"prometheus_export_url"` // Port để expose metrics
	DefaultNamespace    string `yaml:"default_namespace"`
	AutoStart           bool   `yaml:"auto_start"`
}

// PrometheusYAMLConfig cấu hình Prometheus từ YAML
type PrometheusYAMLConfig struct {
	URL               string `yaml:"url"`
	TimeoutSeconds    int    `yaml:"timeout_seconds"`
	RetryAttempts     int    `yaml:"retry_attempts"`
	RetryDelaySeconds int    `yaml:"retry_delay_seconds"`
}

// DetectionYAMLConfig cấu hình detection từ YAML
type DetectionYAMLConfig struct {
	BaselineMultiplier    float64 `yaml:"baseline_multiplier"`
	BaselineWindowMinutes int     `yaml:"baseline_window_minutes"`
	CheckIntervalSeconds  int     `yaml:"check_interval_seconds"`
}

// AlertingYAMLConfig cấu hình alerting từ YAML
type AlertingYAMLConfig struct {
	Enabled              bool                   `yaml:"enabled"`
	MaxAlertsPerMinute   int                    `yaml:"max_alerts_per_minute"`
	AlertCooldownSeconds int                    `yaml:"alert_cooldown_seconds"`
	Channels             AlertChannelsYAML      `yaml:"channels"`
	Telegram             TelegramYAMLConfig     `yaml:"telegram"`
	Alertmanager         AlertmanagerYAMLConfig `yaml:"alertmanager,omitempty"`
}

// AlertmanagerYAMLConfig cấu hình Alertmanager từ YAML
type AlertmanagerYAMLConfig struct {
	Enabled        bool                    `yaml:"enabled"`
	URL            string                  `yaml:"url"`
	ResolveTimeout string                  `yaml:"resolve_timeout"`
	Route          AlertmanagerRouteConfig `yaml:"route"`
	TelegramConfig TelegramYAMLConfig      `yaml:"telegram_config"`
}

// AlertmanagerRouteConfig cấu hình route của Alertmanager
type AlertmanagerRouteConfig struct {
	Receiver       string   `yaml:"receiver"`
	GroupBy        []string `yaml:"group_by"`
	RepeatInterval string   `yaml:"repeat_interval"`
	GroupWait      string   `yaml:"group_wait"`
	GroupInterval  string   `yaml:"group_interval"`
}

// AlertChannelsYAML cấu hình channels từ YAML
type AlertChannelsYAML struct {
	Log      bool `yaml:"log"`
	Webhook  bool `yaml:"webhook"`
	Email    bool `yaml:"email"`
	Telegram bool `yaml:"telegram"`
}

// TelegramYAMLConfig cấu hình Telegram từ YAML
type TelegramYAMLConfig struct {
	BotToken        string `yaml:"bot_token"`
	ChatID          string `yaml:"chat_id"`
	ParseMode       string `yaml:"parse_mode"`
	Enabled         bool   `yaml:"enabled"`
	MessageTemplate string `yaml:"message_template,omitempty"`
}

// LoggingYAMLConfig cấu hình logging từ YAML
type LoggingYAMLConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	FilePath string `yaml:"file_path"`
}

// LoadAnomalyDetectionConfig loads configuration from YAML file
func LoadAnomalyDetectionConfig(filename string) (*AnomalyDetectionConfig, error) {
	// Default to anomaly_detection.yaml if filename is empty
	if filename == "" {
		filename = "configs/anomaly_detection.yaml"
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", filename, err)
	}

	var config AnomalyDetectionConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config file %s: %v", filename, err)
	}

	// Validate and set defaults
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *AnomalyDetectionConfig) Validate() error {
	// Application defaults
	if c.Application.HubbleServer == "" {
		c.Application.HubbleServer = "localhost:4245"
	}
	if c.Application.PrometheusExportURL == "" {
		// Extract port from URL, default to 8080
		c.Application.PrometheusExportURL = "8080"
	}
	if c.Application.DefaultNamespace == "" {
		c.Application.DefaultNamespace = "default"
	}

	// Prometheus defaults
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

	// Namespaces defaults
	if len(c.Namespaces) == 0 {
		c.Namespaces = []string{"default"}
	}

	// Detection defaults
	if c.Detection.BaselineMultiplier <= 0 {
		c.Detection.BaselineMultiplier = 3.0
	}
	if c.Detection.BaselineWindowMinutes <= 0 {
		c.Detection.BaselineWindowMinutes = 1
	}
	if c.Detection.CheckIntervalSeconds <= 0 {
		c.Detection.CheckIntervalSeconds = 10
	}

	// Alerting defaults
	if c.Alerting.MaxAlertsPerMinute <= 0 {
		c.Alerting.MaxAlertsPerMinute = 10
	}
	if c.Alerting.AlertCooldownSeconds <= 0 {
		c.Alerting.AlertCooldownSeconds = 60
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "INFO"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}

	return nil
}

// ToPrometheusAnomalyConfig converts YAML config to PrometheusAnomalyConfig format
// for backward compatibility
func (c *AnomalyDetectionConfig) ToPrometheusAnomalyConfig() *PrometheusAnomalyConfig {
	// Convert rules to map format
	rulesMap := make(map[string]AnomalyRuleConfig)
	for _, rule := range c.Rules {
		ruleConfig := AnomalyRuleConfig{
			Enabled:     rule.Enabled,
			Severity:    rule.Severity,
			Description: rule.Description,
		}

		// Extract thresholds
		if thresholds, ok := rule.Thresholds["multiplier"].(float64); ok {
			multiplier := thresholds
			ruleConfig.ThresholdMultiplier = &multiplier
		}
		if thresholds, ok := rule.Thresholds["per_minute"].(int); ok {
			perMinute := thresholds
			ruleConfig.ThresholdPerMinute = &perMinute
		}

		rulesMap[rule.Name] = ruleConfig
	}

	// Extract port from PrometheusExportURL
	exportPort := c.Application.PrometheusExportURL
	if strings.Contains(exportPort, ":") {
		parts := strings.Split(exportPort, ":")
		if len(parts) > 0 {
			exportPort = parts[len(parts)-1]
		}
	}

	return &PrometheusAnomalyConfig{
		Application: ApplicationConfig{
			HubbleServer:     c.Application.HubbleServer,
			PrometheusPort:   exportPort,
			DefaultNamespace: c.Application.DefaultNamespace,
		},
		Prometheus: PrometheusConfig{
			URL:               c.Prometheus.URL,
			TimeoutSeconds:    c.Prometheus.TimeoutSeconds,
			RetryAttempts:     c.Prometheus.RetryAttempts,
			RetryDelaySeconds: c.Prometheus.RetryDelaySeconds,
		},
		Detection: DetectionConfig{
			BaselineMultiplier:    c.Detection.BaselineMultiplier,
			BaselineWindowMinutes: c.Detection.BaselineWindowMinutes,
			CheckIntervalSeconds:  c.Detection.CheckIntervalSeconds,
			Namespaces:            c.Namespaces,
		},
		Rules: rulesMap,
		Alerting: AlertingConfig{
			Enabled:              c.Alerting.Enabled,
			MaxAlertsPerMinute:   c.Alerting.MaxAlertsPerMinute,
			AlertCooldownSeconds: c.Alerting.AlertCooldownSeconds,
			Channels: struct {
				Log      bool `json:"log"`
				Webhook  bool `json:"webhook"`
				Email    bool `json:"email"`
				Telegram bool `json:"telegram"`
			}{
				Log:      c.Alerting.Channels.Log,
				Webhook:  c.Alerting.Channels.Webhook,
				Email:    c.Alerting.Channels.Email,
				Telegram: c.Alerting.Channels.Telegram,
			},
			Telegram: TelegramConfig{
				BotToken:  c.Alerting.Telegram.BotToken,
				ChatID:    c.Alerting.Telegram.ChatID,
				ParseMode: c.Alerting.Telegram.ParseMode,
				Enabled:   c.Alerting.Telegram.Enabled,
			},
		},
		Logging: LoggingConfig{
			Level:    c.Logging.Level,
			Format:   c.Logging.Format,
			FilePath: c.Logging.FilePath,
		},
	}
}

// GetRuleConfigByName gets rule config by name
func (c *AnomalyDetectionConfig) GetRuleConfigByName(name string) (*model.Rule, bool) {
	for i := range c.Rules {
		if c.Rules[i].Name == name {
			return &c.Rules[i], true
		}
	}
	return nil, false
}

// IsRuleEnabled checks if a rule is enabled
func (c *AnomalyDetectionConfig) IsRuleEnabled(name string) bool {
	rule, exists := c.GetRuleConfigByName(name)
	return exists && rule.Enabled
}

// RegisterBuiltinRulesFromYAML registers rules from YAML config
func RegisterBuiltinRulesFromYAML(engine *rules.Engine, yamlConfig *AnomalyDetectionConfig, logger *logrus.Logger, promClient *client.PrometheusClient) {
	for _, ruleConfig := range yamlConfig.Rules {
		if !ruleConfig.Enabled {
			continue
		}

		// Handle different rule types
		switch ruleConfig.Name {
		case "traffic_spike":
			if promClient != nil {
				threshold := 3.0
				if thresholds, ok := ruleConfig.Thresholds["multiplier"].(float64); ok {
					threshold = thresholds
				} else if thresholds, ok := ruleConfig.Thresholds["multiplier"].(int); ok {
					threshold = float64(thresholds)
				}

				promRule := builtin.NewDDoSRulePrometheus(ruleConfig.Enabled, ruleConfig.Severity, threshold, promClient, logger)
				promRule.SetNamespaces(yamlConfig.Namespaces)
				promRule.SetAlertEmitter(func(alert *model.Alert) {
					engine.EmitAlert(*alert)
				})
				engine.RegisterRule(promRule)
				ctx := context.Background()
				go promRule.Start(ctx)
				logger.Infof("Registered rule: %s (threshold: %.2fx)", ruleConfig.Name, threshold)
			}

		case "new_destination":
			// TODO: Implement Prometheus-based new destination rule
			logger.Debugf("Rule %s is configured but not yet implemented with Prometheus", ruleConfig.Name)

		case "tcp_reset_surge":
			// TODO: Implement Prometheus-based TCP reset rule
			logger.Debugf("Rule %s is configured but not yet implemented with Prometheus", ruleConfig.Name)

		case "tcp_drop_surge":
			// TODO: Implement Prometheus-based TCP drop rule
			logger.Debugf("Rule %s is configured but not yet implemented with Prometheus", ruleConfig.Name)

		case "high_bandwidth":
			// TODO: Implement Prometheus-based bandwidth rule
			logger.Debugf("Rule %s is configured but not yet implemented with Prometheus", ruleConfig.Name)

		case "unusual_port_scan":
			// TODO: Implement Prometheus-based port scan rule
			logger.Debugf("Rule %s is configured but not yet implemented with Prometheus", ruleConfig.Name)

		default:
			logger.Warnf("Unknown rule type: %s", ruleConfig.Name)
		}
	}
}

// RegisterBuiltinRules registers rules from JSON config (backward compatibility)
func RegisterBuiltinRules(engine *rules.Engine, config *PrometheusAnomalyConfig, logger *logrus.Logger, promClient *client.PrometheusClient) {
	// DDoS rule - query from Prometheus
	if ruleConfig, exists := config.GetRuleConfig("traffic_spike"); exists && promClient != nil {
		threshold := 3.0
		if ruleConfig.ThresholdMultiplier != nil {
			threshold = *ruleConfig.ThresholdMultiplier
		}
		// Use Prometheus-based rule
		promRule := builtin.NewDDoSRulePrometheus(ruleConfig.Enabled, ruleConfig.Severity, threshold, promClient, logger)
		promRule.SetNamespaces(config.Detection.Namespaces)
		promRule.SetAlertEmitter(func(alert *model.Alert) {
			engine.EmitAlert(*alert)
		})
		if promRule.IsEnabled() {
			engine.RegisterRule(promRule)
			// Start periodic checking in background
			ctx := context.Background()
			go promRule.Start(ctx)
		}
	}

	// Other rules from JSON config...
	// (Keep existing logic for backward compatibility)
}
