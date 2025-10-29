package main

import (
	"context"

	"hubble-anomaly-detector/internal/client"
	"hubble-anomaly-detector/internal/model"
	"hubble-anomaly-detector/internal/rules"
	"hubble-anomaly-detector/internal/rules/builtin"
	"hubble-anomaly-detector/internal/utils"

	"github.com/sirupsen/logrus"
)

// registerBuiltinRulesFromYAML registers rules from YAML config
func registerBuiltinRulesFromYAML(engine *rules.Engine, yamlConfig *utils.AnomalyDetectionConfig, logger *logrus.Logger, promClient *client.PrometheusClient) {
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
				promRule.SetNamespaces(yamlConfig.Detection.Namespaces)
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

// registerBuiltinRules registers rules from JSON config (backward compatibility)
func registerBuiltinRules(engine *rules.Engine, config *utils.PrometheusAnomalyConfig, logger *logrus.Logger, promClient *client.PrometheusClient) {
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

}
