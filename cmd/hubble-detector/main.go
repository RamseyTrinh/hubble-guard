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
	"hubble-anomaly-detector/internal/model"
	"hubble-anomaly-detector/internal/pipeline"
	"hubble-anomaly-detector/internal/rules"
	"hubble-anomaly-detector/internal/utils"

	"github.com/sirupsen/logrus"
)

func main() {
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
	config, err := utils.LoadAnomalyDetectionConfig(*configFile)
	if err != nil {
		fmt.Printf("Failed to load YAML config %s: %v\n", *configFile, err)
		fmt.Println("Using default configuration...")
		config = utils.GetDefaultAnomalyDetectionConfig()
	} else {
		fmt.Printf("✅ Loaded configuration from %s\n", *configFile)
	}

	hubbleServer := config.Application.HubbleServer
	prometheusPort := config.GetPrometheusPort()
	namespace := config.Application.DefaultNamespace

	fmt.Println("Hubble Anomaly Detector")
	fmt.Printf("Connecting to Hubble relay at: %s\n", hubbleServer)
	fmt.Printf("Prometheus export URL: %s\n", prometheusPort)
	fmt.Printf("Prometheus query URL: %s\n", config.Prometheus.URL)
	if len(config.Namespaces) > 0 {
		fmt.Printf("Monitoring namespaces: %s\n", strings.Join(config.Namespaces, ", "))
	} else {
		fmt.Printf("Using namespace: %s\n", namespace)
	}
	fmt.Println("")

	logger := utils.NewLogger(config.Logging.Level)
	logger.SetLevel(logrus.InfoLevel)

	exporter, err := alert.StartPrometheusExporterWithCustomRegistry(prometheusPort, logger)
	if err != nil {
		fmt.Printf("Failed to create Prometheus exporter: %v\n", err)
		os.Exit(1)
	}

	exporterCtx, exporterCancel := context.WithCancel(context.Background())
	go func() {
		if err := exporter.Start(exporterCtx); err != nil {
			logger.Errorf("Prometheus exporter error: %v", err)
		}
	}()

	hubbleClient, err := client.NewHubbleGRPCClientWithMetrics(hubbleServer, exporter.GetMetrics())
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		exporterCancel()
		os.Exit(1)
	}
	defer hubbleClient.Close()
	defer exporterCancel()

	promClient, err := client.NewPrometheusClient(config.Prometheus.URL)
	if err != nil {
		fmt.Printf("Warning: Failed to create Prometheus client: %v\n", err)
		fmt.Println("Rules will not be able to query Prometheus...")
		promClient = nil
	}

	engine := rules.NewEngine(logger)

	rules.SetGlobalMetrics(exporter.GetMetrics())

	utils.RegisterBuiltinRulesFromYAML(engine, config, logger, promClient)

	registerAlertNotifiers(engine, config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := hubbleClient.TestConnection(ctx); err != nil {
		fmt.Printf("Warning: Connection test failed: %v\n", err)
		fmt.Println("Continuing anyway...")
	}
	cancel()

	var namespaces []string
	if len(config.Namespaces) > 0 {
		namespaces = config.Namespaces
	} else {
		namespaces = []string{namespace}
	}

	streamFlowsToPrometheus(hubbleClient, namespaces, engine, logger, config)

}

func registerAlertNotifiers(engine *rules.Engine, config *utils.AnomalyDetectionConfig, logger *logrus.Logger) {
	if config.Alerting.Channels.Log {
		logNotifier := alert.NewLogAlertNotifier(logger)
		engine.RegisterNotifier(logNotifier)
	}

	if config.Alerting.Channels.Telegram && config.Alerting.Telegram.Enabled {
		messageTemplate := config.Alerting.Telegram.MessageTemplate

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

func streamFlowsToPrometheus(hubbleClient *client.HubbleGRPCClient, namespaces []string, engine *rules.Engine, logger *logrus.Logger, config *utils.AnomalyDetectionConfig) {
	fmt.Println("\n =============================================== ANOMALY DETECTION ===============================================\n")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-sigChan
		fmt.Println("\nStopping flow collection...")
		cancel()
	}()

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

	fmt.Println(" Flow collection started!")
	fmt.Println("")

	// Create processor to evaluate flows with rules
	processor := pipeline.NewProcessor(engine)

	err := hubbleClient.StreamFlowsWithMetricsOnly(ctx, namespaces, func(ns string) {
	}, func(flow *model.Flow) {
		// Process flow through engine to evaluate rules
		if err := processor.Process(ctx, flow); err != nil {
			logger.Errorf("Failed to process flow: %v", err)
		}
	})
	if err != nil {
		if err == context.Canceled {
			fmt.Println("Flow collection stopped by user")
		} else {
			fmt.Printf("Flow streaming failed: %v\n", err)
		}
	}
}

func testTelegramNotification(configFile string) {
	config, err := utils.LoadAnomalyDetectionConfig(configFile)
	if err != nil {
		fmt.Printf("Failed to load config file %s: %v\n", configFile, err)
		fmt.Println("Using default configuration...")
		config = utils.GetDefaultAnomalyDetectionConfig()
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
