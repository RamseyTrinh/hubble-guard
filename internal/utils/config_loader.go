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

type AnomalyDetectionConfig struct {
	Application ApplicationYAMLConfig `yaml:"application"`
	Prometheus  PrometheusYAMLConfig  `yaml:"prometheus"`
	Namespaces  []string              `yaml:"namespaces"`
	Detection   DetectionYAMLConfig   `yaml:"detection"`
	Rules       []model.Rule          `yaml:"rules"`
	Alerting    AlertingYAMLConfig    `yaml:"alerting"`
	Logging     LoggingYAMLConfig     `yaml:"logging"`
}

type ApplicationYAMLConfig struct {
	HubbleServer        string `yaml:"hubble_server"`
	PrometheusExportURL string `yaml:"prometheus_export_url"`
	DefaultNamespace    string `yaml:"default_namespace"`
	AutoStart           bool   `yaml:"auto_start"`
}

type PrometheusYAMLConfig struct {
	URL               string `yaml:"url"`
	TimeoutSeconds    int    `yaml:"timeout_seconds"`
	RetryAttempts     int    `yaml:"retry_attempts"`
	RetryDelaySeconds int    `yaml:"retry_delay_seconds"`
}

type DetectionYAMLConfig struct {
	BaselineMultiplier    float64 `yaml:"baseline_multiplier"`
	BaselineWindowMinutes int     `yaml:"baseline_window_minutes"`
	CheckIntervalSeconds  int     `yaml:"check_interval_seconds"`
}

type AlertingYAMLConfig struct {
	Enabled              bool                   `yaml:"enabled"`
	MaxAlertsPerMinute   int                    `yaml:"max_alerts_per_minute"`
	AlertCooldownSeconds int                    `yaml:"alert_cooldown_seconds"`
	Channels             AlertChannelsYAML      `yaml:"channels"`
	Telegram             TelegramYAMLConfig     `yaml:"telegram"`
	Alertmanager         AlertmanagerYAMLConfig `yaml:"alertmanager,omitempty"`
}

type AlertmanagerYAMLConfig struct {
	Enabled        bool                    `yaml:"enabled"`
	URL            string                  `yaml:"url"`
	ResolveTimeout string                  `yaml:"resolve_timeout"`
	Route          AlertmanagerRouteConfig `yaml:"route"`
	TelegramConfig TelegramYAMLConfig      `yaml:"telegram_config"`
}

type AlertmanagerRouteConfig struct {
	Receiver       string   `yaml:"receiver"`
	GroupBy        []string `yaml:"group_by"`
	RepeatInterval string   `yaml:"repeat_interval"`
	GroupWait      string   `yaml:"group_wait"`
	GroupInterval  string   `yaml:"group_interval"`
}

type AlertChannelsYAML struct {
	Log      bool `yaml:"log"`
	Webhook  bool `yaml:"webhook"`
	Email    bool `yaml:"email"`
	Telegram bool `yaml:"telegram"`
}

type TelegramYAMLConfig struct {
	BotToken        string `yaml:"bot_token"`
	ChatID          string `yaml:"chat_id"`
	ParseMode       string `yaml:"parse_mode"`
	Enabled         bool   `yaml:"enabled"`
	MessageTemplate string `yaml:"message_template,omitempty"`
}

type LoggingYAMLConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	FilePath string `yaml:"file_path"`
}

func LoadAnomalyDetectionConfig(filename string) (*AnomalyDetectionConfig, error) {
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

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	return &config, nil
}

func (c *AnomalyDetectionConfig) Validate() error {
	if c.Application.HubbleServer == "" {
		c.Application.HubbleServer = "localhost:4245"
	}
	if c.Application.PrometheusExportURL == "" {
		c.Application.PrometheusExportURL = "8080"
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

	if len(c.Namespaces) == 0 {
		c.Namespaces = []string{"default"}
	}

	if c.Detection.BaselineMultiplier <= 0 {
		c.Detection.BaselineMultiplier = 3.0
	}
	if c.Detection.BaselineWindowMinutes <= 0 {
		c.Detection.BaselineWindowMinutes = 1
	}
	if c.Detection.CheckIntervalSeconds <= 0 {
		c.Detection.CheckIntervalSeconds = 10
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

// GetPrometheusPort extracts port from PrometheusExportURL
func (c *AnomalyDetectionConfig) GetPrometheusPort() string {
	exportPort := c.Application.PrometheusExportURL
	if strings.Contains(exportPort, ":") {
		parts := strings.Split(exportPort, ":")
		if len(parts) > 0 {
			exportPort = parts[len(parts)-1]
		}
	}
	return exportPort
}

func (c *AnomalyDetectionConfig) GetRuleConfigByName(name string) (*model.Rule, bool) {
	for i := range c.Rules {
		if c.Rules[i].Name == name {
			return &c.Rules[i], true
		}
	}
	return nil, false
}

func (c *AnomalyDetectionConfig) IsRuleEnabled(name string) bool {
	rule, exists := c.GetRuleConfigByName(name)
	return exists && rule.Enabled
}

// GetDefaultAnomalyDetectionConfig returns a default AnomalyDetectionConfig
func GetDefaultAnomalyDetectionConfig() *AnomalyDetectionConfig {
	return &AnomalyDetectionConfig{
		Application: ApplicationYAMLConfig{
			HubbleServer:        "localhost:4245",
			PrometheusExportURL: "8080",
			DefaultNamespace:    "default",
			AutoStart:           false,
		},
		Prometheus: PrometheusYAMLConfig{
			URL:               "http://localhost:9090",
			TimeoutSeconds:    10,
			RetryAttempts:     3,
			RetryDelaySeconds: 5,
		},
		Namespaces: []string{"default"},
		Detection: DetectionYAMLConfig{
			BaselineMultiplier:    3.0,
			BaselineWindowMinutes: 1,
			CheckIntervalSeconds:  30,
		},
		Rules: []model.Rule{},
		Alerting: AlertingYAMLConfig{
			Enabled:              true,
			MaxAlertsPerMinute:   10,
			AlertCooldownSeconds: 60,
			Channels: AlertChannelsYAML{
				Log:      true,
				Webhook:  false,
				Email:    false,
				Telegram: true,
			},
			Telegram: TelegramYAMLConfig{
				BotToken:  "",
				ChatID:    "",
				ParseMode: "Markdown",
				Enabled:   false,
			},
		},
		Logging: LoggingYAMLConfig{
			Level:    "INFO",
			Format:   "json",
			FilePath: "/var/log/anomaly-detector.log",
		},
	}
}

func RegisterBuiltinRulesFromYAML(engine *rules.Engine, yamlConfig *AnomalyDetectionConfig, logger *logrus.Logger, promClient *client.PrometheusClient) {
	for _, ruleConfig := range yamlConfig.Rules {
		if !ruleConfig.Enabled {
			continue
		}

		switch ruleConfig.Name {
		case "ddos":
			threshold := 3.0
			if thresholds, ok := ruleConfig.Thresholds["multiplier"].(float64); ok {
				threshold = thresholds
			} else if thresholds, ok := ruleConfig.Thresholds["multiplier"].(int); ok {
				threshold = float64(thresholds)
			}

			ddosRule := builtin.NewDDoSRule(ruleConfig.Enabled, ruleConfig.Severity, threshold, logger)
			ddosRule.SetAlertEmitter(func(alert *model.Alert) {
				engine.EmitAlert(*alert)
			})
			engine.RegisterRule(ddosRule)
			logger.Infof("Registered rule: %s (threshold: %.2fx, real-time monitoring)", ruleConfig.Name, threshold)

		case "traffic_spike":
			if promClient != nil {
				threshold := 3.0
				if thresholds, ok := ruleConfig.Thresholds["multiplier"].(float64); ok {
					threshold = thresholds
				} else if thresholds, ok := ruleConfig.Thresholds["multiplier"].(int); ok {
					threshold = float64(thresholds)
				}

				promRule := builtin.NewTrafficSpikeRule(ruleConfig.Enabled, ruleConfig.Severity, threshold, promClient, logger)
				promRule.SetNamespaces(yamlConfig.Namespaces)
				promRule.SetAlertEmitter(func(alert *model.Alert) {
					engine.EmitAlert(*alert)
				})
				engine.RegisterRule(promRule)
				ctx := context.Background()
				go promRule.Start(ctx)
				logger.Infof("Registered rule: %s (threshold: %.2fx)", ruleConfig.Name, threshold)
			}

		case "traffic_death":
			if promClient != nil {
				promRule := builtin.NewTrafficDeathRule(ruleConfig.Enabled, ruleConfig.Severity, promClient, logger)
				promRule.SetNamespaces(yamlConfig.Namespaces)
				promRule.SetAlertEmitter(func(alert *model.Alert) {
					engine.EmitAlert(*alert)
				})
				engine.RegisterRule(promRule)
				ctx := context.Background()
				go promRule.Start(ctx)
				logger.Infof("Registered rule: %s", ruleConfig.Name)
			}

		case "block_connection":
			if promClient != nil {
				threshold := 10.0
				if thresholds, ok := ruleConfig.Thresholds["per_minute"].(float64); ok {
					threshold = thresholds
				} else if thresholds, ok := ruleConfig.Thresholds["per_minute"].(int); ok {
					threshold = float64(thresholds)
				} else if thresholds, ok := ruleConfig.Thresholds["count"].(float64); ok {
					threshold = thresholds
				} else if thresholds, ok := ruleConfig.Thresholds["count"].(int); ok {
					threshold = float64(thresholds)
				}

				blockRule := builtin.NewBlockConnectionRule(ruleConfig.Enabled, ruleConfig.Severity, threshold, promClient, logger)
				blockRule.SetNamespaces(yamlConfig.Namespaces)
				blockRule.SetAlertEmitter(func(alert *model.Alert) {
					engine.EmitAlert(*alert)
				})
				engine.RegisterRule(blockRule)
				ctx := context.Background()
				go blockRule.Start(ctx)
				logger.Infof("Registered rule: %s (threshold: %.0f DROP flows per minute)", ruleConfig.Name, threshold)
			}

		case "port_scan":
			if promClient != nil {
				threshold := 10.0
				if thresholds, ok := ruleConfig.Thresholds["distinct_ports"].(float64); ok {
					threshold = thresholds
				} else if thresholds, ok := ruleConfig.Thresholds["distinct_ports"].(int); ok {
					threshold = float64(thresholds)
				} else if thresholds, ok := ruleConfig.Thresholds["count"].(float64); ok {
					threshold = thresholds
				} else if thresholds, ok := ruleConfig.Thresholds["count"].(int); ok {
					threshold = float64(thresholds)
				}

				portScanRule := builtin.NewPortScanRule(ruleConfig.Enabled, ruleConfig.Severity, threshold, promClient, logger)
				portScanRule.SetNamespaces(yamlConfig.Namespaces)
				portScanRule.SetAlertEmitter(func(alert *model.Alert) {
					engine.EmitAlert(*alert)
				})
				engine.RegisterRule(portScanRule)
				ctx := context.Background()
				go portScanRule.Start(ctx)
				logger.Infof("Registered rule: %s (threshold: %.0f distinct ports in 10s)", ruleConfig.Name, threshold)
			}

		case "suspicious_outbound":
			if promClient != nil {
				threshold := 10.0
				if thresholds, ok := ruleConfig.Thresholds["per_minute"].(float64); ok {
					threshold = thresholds
				} else if thresholds, ok := ruleConfig.Thresholds["per_minute"].(int); ok {
					threshold = float64(thresholds)
				} else if thresholds, ok := ruleConfig.Thresholds["threshold"].(float64); ok {
					threshold = thresholds
				} else if thresholds, ok := ruleConfig.Thresholds["threshold"].(int); ok {
					threshold = float64(thresholds)
				}

				outboundRule := builtin.NewOutboundRule(ruleConfig.Enabled, ruleConfig.Severity, threshold, promClient, logger)
				outboundRule.SetNamespaces(yamlConfig.Namespaces)
				outboundRule.SetAlertEmitter(func(alert *model.Alert) {
					engine.EmitAlert(*alert)
				})
				engine.RegisterRule(outboundRule)
				ctx := context.Background()
				go outboundRule.Start(ctx)
				logger.Infof("Registered rule: %s (threshold: %.0f connections per minute)", ruleConfig.Name, threshold)
			}

		case "namespace_access":
			if promClient != nil {
				var forbiddenNS []string
				if thresholds, ok := ruleConfig.Thresholds["forbidden_namespaces"].([]interface{}); ok {
					for _, ns := range thresholds {
						if nsStr, ok := ns.(string); ok {
							forbiddenNS = append(forbiddenNS, nsStr)
						}
					}
				} else if thresholds, ok := ruleConfig.Thresholds["forbidden_namespaces"].([]string); ok {
					forbiddenNS = thresholds
				}

				if len(forbiddenNS) == 0 {
					for _, condition := range ruleConfig.Conditions {
						if condition.Field == "forbidden_namespaces" {
							if nsList, ok := condition.Value.([]interface{}); ok {
								for _, ns := range nsList {
									if nsStr, ok := ns.(string); ok {
										forbiddenNS = append(forbiddenNS, nsStr)
									}
								}
							} else if nsList, ok := condition.Value.([]string); ok {
								forbiddenNS = nsList
							}
						}
					}
				}

				nsAccessRule := builtin.NewNamespaceAccessRule(ruleConfig.Enabled, ruleConfig.Severity, promClient, logger)
				if len(forbiddenNS) > 0 {
					nsAccessRule.SetForbiddenNamespaces(forbiddenNS)
				} else {
					defaultForbiddenNS := []string{"kube-system", "monitoring", "security"}
					nsAccessRule.SetForbiddenNamespaces(defaultForbiddenNS)
					logger.Warnf("No forbidden namespaces configured for namespace_access rule, using defaults: %v", defaultForbiddenNS)
				}
				nsAccessRule.SetAlertEmitter(func(alert *model.Alert) {
					engine.EmitAlert(*alert)
				})
				engine.RegisterRule(nsAccessRule)
				ctx := context.Background()
				go nsAccessRule.Start(ctx)
				logger.Infof("Registered rule: %s (forbidden namespaces: %v)", ruleConfig.Name, forbiddenNS)
			}

		default:
			logger.Warnf("Unknown rule type: %s", ruleConfig.Name)
		}
	}
}
