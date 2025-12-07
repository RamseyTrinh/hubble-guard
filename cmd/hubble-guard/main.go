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

	"hubble-guard/internal/alert"
	"hubble-guard/internal/client"
	"hubble-guard/internal/model"
	"hubble-guard/internal/pipeline"
	"hubble-guard/internal/rules"
	"hubble-guard/internal/utils"

	"github.com/sirupsen/logrus"
)

func getVersion() string {
	content, err := os.ReadFile("VERSION")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(content))
}

func main() {
	var (
		configFile = flag.String("config", "configs/anomaly_detection.yaml", "Configuration file path (YAML)")
	)
	flag.Parse()

	// Load configuration from YAML file
	config, err := utils.LoadAnomalyDetectionConfig(*configFile)
	if err != nil {
		fmt.Printf("Failed to load YAML config %s: %v\n", *configFile, err)
		fmt.Println("Using default configuration...")
		config = utils.GetDefaultAnomalyDetectionConfig()
	} else {
		fmt.Printf("Loaded configuration from %s\n", *configFile)
	}

	hubbleServer := config.Application.HubbleServer
	prometheusPort := config.GetPrometheusPort()
	namespace := config.Application.DefaultNamespace

	fmt.Printf("Anomaly Detector v%s\n", getVersion())
	fmt.Printf("Connecting to Hubble relay at: %s\n", hubbleServer)
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

	hubbleClient, err := client.NewHubbleGRPCClient(hubbleServer, exporter.GetMetrics())
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
		promClient = nil
	}

	engine := rules.NewEngine(logger)

	rules.SetGlobalMetrics(exporter.GetMetrics())

	utils.RegisterBuiltinRulesFromYAML(engine, config, logger, promClient)

	registerAlertNotifiers(engine, config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := hubbleClient.TestConnection(ctx); err != nil {
		fmt.Printf("Warning: Connection test failed: %v\n", err)
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
	fmt.Println("\n =============================================== ANOMALY DETECTOR v" + getVersion() + " ===============================================\n")

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

	err := hubbleClient.StreamFlowsWithMetrics(ctx, namespaces, func(ns string) {
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
