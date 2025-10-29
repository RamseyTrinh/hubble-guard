package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"hubble-anomaly-detector/internal/alert"
	"hubble-anomaly-detector/internal/client"
	"hubble-anomaly-detector/internal/rules"
	"hubble-anomaly-detector/internal/utils"

	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command line flags
	var (
		configFile   = flag.String("config", "configs/anomaly_detection.yaml", "Configuration file path (YAML)")
		showVersion  = flag.Bool("version", false, "Show version information")
		testTelegram = flag.Bool("test-telegram", false, "Send test message to Telegram")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("Hubble Anomaly Detector v1.0.0")
		return
	}

	if *testTelegram {
		fmt.Println("Testing Telegram notification...")
		testTelegramNotification(*configFile)
		return
	}

	// Load configuration from YAML file
	var yamlConfig *utils.AnomalyDetectionConfig
	var config *utils.PrometheusAnomalyConfig

	// Try to load from YAML first
	yamlConfig, err := utils.LoadAnomalyDetectionConfig(*configFile)
	if err != nil {
		// Fallback to default config if YAML fails
		fmt.Printf("Failed to load YAML config %s: %v\n", *configFile, err)
		fmt.Println("Using default configuration...")
		config = utils.GetDefaultPrometheusConfig()
	} else {
		// Convert YAML config to PrometheusAnomalyConfig for compatibility
		config = yamlConfig.ToPrometheusAnomalyConfig()
		fmt.Printf("✅ Loaded configuration from %s\n", *configFile)
	}

	// Get values from config
	var hubbleServer, prometheusPort, namespace string
	if yamlConfig != nil {
		hubbleServer = yamlConfig.Application.HubbleServer
		// Extract port from PrometheusExportURL
		prometheusPort = yamlConfig.Application.PrometheusExportURL
		if strings.Contains(prometheusPort, ":") {
			parts := strings.Split(prometheusPort, ":")
			if len(parts) > 0 {
				prometheusPort = parts[len(parts)-1]
			}
		}
		namespace = yamlConfig.Application.DefaultNamespace
	} else {
		hubbleServer = config.Application.HubbleServer
		prometheusPort = config.Application.PrometheusPort
		namespace = config.Application.DefaultNamespace
	}

	fmt.Println("Hubble Anomaly Detector")
	fmt.Printf("Connecting to Hubble relay at: %s\n", hubbleServer)
	fmt.Printf("Prometheus export URL: %s\n", prometheusPort)
	fmt.Printf("Prometheus query URL: %s\n", config.Prometheus.URL)
	fmt.Printf("Using namespace: %s\n", namespace)
	fmt.Println("")

	// Create logger
	logger := utils.NewLogger(config.Logging.Level)
	logger.SetLevel(logrus.InfoLevel)

	// Create Prometheus exporter
	exporter, err := alert.StartPrometheusExporterWithCustomRegistry(prometheusPort, logger)
	if err != nil {
		fmt.Printf("Failed to create Prometheus exporter: %v\n", err)
		os.Exit(1)
	}

	// Start Prometheus exporter in background
	exporterCtx, exporterCancel := context.WithCancel(context.Background())
	go func() {
		if err := exporter.Start(exporterCtx); err != nil {
			logger.Errorf("Prometheus exporter error: %v", err)
		}
	}()

	// Create gRPC client with metrics
	hubbleClient, err := client.NewHubbleGRPCClientWithMetrics(hubbleServer, exporter.GetMetrics())
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		exporterCancel()
		os.Exit(1)
	}
	defer hubbleClient.Close()
	defer exporterCancel()

	// Create Prometheus client for rules
	promClient, err := client.NewPrometheusClient(config.Prometheus.URL)
	if err != nil {
		fmt.Printf("Warning: Failed to create Prometheus client: %v\n", err)
		fmt.Println("Rules will not be able to query Prometheus...")
		promClient = nil
	}

	// Create rule engine
	engine := rules.NewEngine(logger)

	// Register builtin rules (query from Prometheus)
	if yamlConfig != nil {
		// Use YAML config with rules array
		utils.RegisterBuiltinRulesFromYAML(engine, yamlConfig, logger, promClient)
	} else {
		// Use JSON config (backward compatibility)
		utils.RegisterBuiltinRules(engine, config, logger, promClient)
	}

	// Register alert notifiers
	registerAlertNotifiers(engine, config, yamlConfig, logger)

	// Test connection (optional)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := hubbleClient.TestConnection(ctx); err != nil {
		fmt.Printf("Warning: Connection test failed: %v\n", err)
		fmt.Println("Continuing anyway...")
	}
	cancel()

	// Show menu and handle user choice
	streamFlowsToPrometheus(hubbleClient, namespace, engine, logger, config)

}

// registerAlertNotifiers registers alert notifiers with the engine
func registerAlertNotifiers(engine *rules.Engine, config *utils.PrometheusAnomalyConfig, yamlConfig *utils.AnomalyDetectionConfig, logger *logrus.Logger) {
	// Log notifier
	if config.Alerting.Channels.Log {
		logNotifier := alert.NewLogAlertNotifier(logger)
		engine.RegisterNotifier(logNotifier)
	}

	// Telegram notifier
	if config.Alerting.Channels.Telegram && config.Alerting.Telegram.Enabled {
		messageTemplate := ""
		// Try to get message template from YAML config if available
		if yamlConfig != nil && yamlConfig.Alerting.Telegram.MessageTemplate != "" {
			messageTemplate = yamlConfig.Alerting.Telegram.MessageTemplate
		}

		telegramNotifier := alert.NewTelegramNotifierWithTemplate(
			config.Alerting.Telegram.BotToken,
			config.Alerting.Telegram.ChatID,
			config.Alerting.Telegram.ParseMode,
			config.Alerting.Telegram.Enabled,
			messageTemplate,
			logger,
		)
		engine.RegisterNotifier(telegramNotifier)
	}
}

// streamFlowsToPrometheus streams flows and sends metrics to Prometheus
// Rules will query Prometheus separately, not process individual flows
func streamFlowsToPrometheus(hubbleClient *client.HubbleGRPCClient, namespace string, engine *rules.Engine, logger *logrus.Logger, config *utils.PrometheusAnomalyConfig) {
	fmt.Println("\nANOMALY DETECTION MODE (Prometheus-based)")
	fmt.Println("Press Ctrl+C to return to main menu")
	fmt.Println("")
	fmt.Println("📊 Flows will be collected and sent to Prometheus metrics")
	fmt.Println("🔍 Rules will query Prometheus periodically to detect anomalies")
	fmt.Println("")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-sigChan
		fmt.Println("\nStopping flow collection...")
		cancel()
	}()

	// Start alert processor
	go func() {
		alertChannel := engine.GetAlertChannel()
		for {
			select {
			case alert := <-alertChannel:
				timestamp := alert.Timestamp.Format("2006-01-02 15:04:05")
				severityEmoji := "⚠️"
				switch alert.Severity {
				case "CRITICAL":
					severityEmoji = "🔴"
				case "HIGH":
					severityEmoji = "🔴"
				case "MEDIUM":
					severityEmoji = "🟡"
				case "LOW":
					severityEmoji = "🟢"
				}
				fmt.Printf("\n%s [%s] %s - %s\n", severityEmoji, timestamp, alert.Severity, alert.Message)
			case <-ctx.Done():
				return
			}
		}
	}()

	fmt.Println("✅ Rules running in background, querying Prometheus...")
	fmt.Println("✅ Flow collection started!")
	fmt.Println("")

	// Stream flows - ONLY to send metrics to Prometheus, NOT for rule evaluation
	// Rules will query Prometheus separately
	err := hubbleClient.StreamFlowsWithMetricsOnly(ctx, namespace, func(ns string) {
		// Just count flows, rules will query from Prometheus
	}, nil) // No flow processor - rules don't process individual flows
	if err != nil {
		if err == context.Canceled {
			fmt.Println("Flow collection stopped by user")
		} else {
			fmt.Printf("Flow streaming failed: %v\n", err)
		}
	}
}

// testTelegramNotification test gửi thông báo qua Telegram
func testTelegramNotification(configFile string) {
	var config *utils.PrometheusAnomalyConfig

	// Try to load from YAML first
	yamlConfig, err := utils.LoadAnomalyDetectionConfig(configFile)
	if err != nil {
		// Fallback to default config if YAML fails
		fmt.Printf("Failed to load config file %s: %v\n", configFile, err)
		fmt.Println("Using default configuration...")
		config = utils.GetDefaultPrometheusConfig()
	} else {
		config = yamlConfig.ToPrometheusAnomalyConfig()
	}

	logger := utils.NewLogger(config.Logging.Level)
	logger.SetLevel(logrus.InfoLevel)

	telegramNotifier := alert.NewTelegramNotifier(
		config.Alerting.Telegram.BotToken,
		config.Alerting.Telegram.ChatID,
		config.Alerting.Telegram.ParseMode,
		config.Alerting.Telegram.Enabled,
		logger,
	)

	if !telegramNotifier.IsEnabled() {
		fmt.Println("❌ Telegram notifier is disabled in configuration")
		return
	}

	fmt.Println("Sending test message to Telegram...")
	if err := telegramNotifier.SendTestMessage(); err != nil {
		fmt.Printf("❌ Failed to send test message: %v\n", err)
		return
	}

	fmt.Println("✅ Test message sent successfully to Telegram!")
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
