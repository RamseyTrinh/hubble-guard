package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hubble-anomaly-detector/api/internal/handlers"
	"hubble-anomaly-detector/api/internal/storage"
	"hubble-anomaly-detector/internal/client"
	"hubble-anomaly-detector/internal/model"
	"hubble-anomaly-detector/internal/utils"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		configFile = flag.String("config", "configs/anomaly_detection.yaml", "Configuration file path (YAML)")
		port       = flag.String("port", "5001", "API server port")
	)
	flag.Parse()

	// Load configuration
	config, err := utils.LoadAnomalyDetectionConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := utils.NewLogger(config.Logging.Level)
	logger.SetLevel(logrus.InfoLevel)

	// Create in-memory storage
	store := storage.NewStorage(logger)

	// Create shared Prometheus metrics instance (singleton to avoid duplicate registration)
	sharedMetrics := client.NewPrometheusMetrics()

	// Create Hubble client for WebSocket streaming
	hubbleClient, err := client.NewHubbleGRPCClientWithMetrics(config.Application.HubbleServer, sharedMetrics)
	if err != nil {
		logger.Warnf("Failed to create Hubble client for WebSocket: %v", err)
		logger.Warnf("WebSocket flow streaming will not be available")
		hubbleClient = nil
	} else {
		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := hubbleClient.TestConnection(ctx); err != nil {
			logger.Warnf("Hubble connection test failed: %v", err)
			logger.Warnf("WebSocket flow streaming will not be available")
			hubbleClient.Close()
			hubbleClient = nil
		}
		cancel()
	}

	// Create handlers
	h := handlers.NewHandlers(store, config, logger, hubbleClient)

	// Setup router
	router := mux.NewRouter()

	// CORS middleware
	router.Use(corsMiddleware)

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Flows endpoints (specific routes must come before parameterized routes)
	api.HandleFunc("/flows/stats", h.GetFlowStats).Methods("GET")
	api.HandleFunc("/stream/flows", h.StreamFlows).Methods("GET")
	api.HandleFunc("/flows", h.GetFlows).Methods("GET")
	api.HandleFunc("/flows/{id}", h.GetFlow).Methods("GET")

	// Alerts endpoints (specific routes must come before parameterized routes)
	api.HandleFunc("/alerts/timeline", h.GetAlertsTimeline).Methods("GET")
	api.HandleFunc("/stream/alerts", h.StreamAlerts).Methods("GET")
	api.HandleFunc("/alerts", h.GetAlerts).Methods("GET")
	api.HandleFunc("/alerts/{id}", h.GetAlert).Methods("GET")

	// Rules endpoints (specific routes must come before parameterized routes)
	api.HandleFunc("/rules/stats", h.GetRulesStats).Methods("GET")
	api.HandleFunc("/rules", h.GetRules).Methods("GET")
	api.HandleFunc("/rules/{id}", h.GetRule).Methods("GET")
	api.HandleFunc("/rules/{id}", h.UpdateRule).Methods("PUT")

	// Metrics endpoints
	api.HandleFunc("/metrics/stats", h.GetMetricsStats).Methods("GET")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET", "OPTIONS")

	// Start background task to sync rules from config
	go syncRulesFromConfig(store, config, logger)

	// Start background task to stream flows from Hubble
	go streamFlowsFromHubble(store, config, logger, sharedMetrics)

	// Start server
	addr := fmt.Sprintf(":%s", *port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		// Disable timeouts for WebSocket connections
		ReadHeaderTimeout: 15 * time.Second,
	}

	logger.Infof("API server starting on port %s", *port)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Info("Shutting down API server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Errorf("Server shutdown error: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Server failed: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow specific origins for development
		allowedOrigins := []string{
			"http://localhost:5000",
			"http://localhost:3000",
			"http://127.0.0.1:5000",
			"http://127.0.0.1:3000",
		}

		// If origin is in allowed list, use it; otherwise use *
		allowOrigin := "*"
		if origin != "" {
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					allowOrigin = origin
					break
				}
			}
		}

		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Only set credentials if using specific origin
		if allowOrigin != "*" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func syncRulesFromConfig(store *storage.Storage, config *utils.AnomalyDetectionConfig, logger *logrus.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		rules := make([]storage.Rule, 0, len(config.Rules))
		for _, rule := range config.Rules {
			rules = append(rules, storage.Rule{
				ID:          rule.Name,
				Name:        rule.Name,
				Enabled:     rule.Enabled,
				Severity:    rule.Severity,
				Description: rule.Description,
				Type:        rule.Type,
				Thresholds:  rule.Thresholds,
			})
		}
		store.SetRules(rules)
		logger.Debugf("Synced %d rules from config", len(rules))
	}
}

func streamFlowsFromHubble(store *storage.Storage, config *utils.AnomalyDetectionConfig, logger *logrus.Logger, sharedMetrics *client.PrometheusMetrics) {
	// Create Hubble client with shared metrics to avoid duplicate registration
	hubbleClient, err := client.NewHubbleGRPCClientWithMetrics(config.Application.HubbleServer, sharedMetrics)
	if err != nil {
		logger.Errorf("Failed to create Hubble client: %v", err)
		return
	}
	defer hubbleClient.Close()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := hubbleClient.TestConnection(ctx); err != nil {
		logger.Warnf("Hubble connection test failed: %v", err)
		logger.Warnf("Flow streaming will not be available")
		cancel()
		return
	}
	cancel()

	// Get namespaces to monitor
	var namespaces []string
	if len(config.Namespaces) > 0 {
		namespaces = config.Namespaces
	} else if config.Application.DefaultNamespace != "" {
		namespaces = []string{config.Application.DefaultNamespace}
	}

	logger.Infof("Starting to stream flows from Hubble (namespaces: %v)", namespaces)

	// Stream flows
	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()

	err = hubbleClient.StreamFlowsWithMetricsOnly(streamCtx, namespaces, func(ns string) {
		// Flow counter callback (optional)
	}, func(flow *model.Flow) {
		// Convert and store flow
		sf := convertModelFlowToStorageFlow(flow)
		store.AddFlow(sf)
		logger.Debugf("Stored flow: %s -> %s", sf.SourceIP, sf.DestinationIP)
	})

	if err != nil && err != context.Canceled {
		logger.Errorf("Flow streaming error: %v", err)
	}
}

// Helper function to convert model.Flow to storage.Flow
func convertModelFlowToStorageFlow(mf *model.Flow) storage.Flow {
	sf := storage.Flow{
		Timestamp: time.Now(),
		Verdict:   mf.Verdict.String(),
	}

	if mf.Time != nil {
		sf.Timestamp = *mf.Time
	}

	// Source endpoint
	if mf.Source != nil {
		sf.Source = &storage.Endpoint{
			Name:      mf.Source.PodName,
			Namespace: mf.Source.Namespace,
			Identity:  mf.Source.Namespace + "/" + mf.Source.PodName,
		}
		sf.Namespace = mf.Source.Namespace
	}

	// Destination endpoint
	if mf.Destination != nil {
		sf.Destination = &storage.Endpoint{
			Name:      mf.Destination.PodName,
			Namespace: mf.Destination.Namespace,
			Identity:  mf.Destination.Namespace + "/" + mf.Destination.PodName,
		}
		if sf.Namespace == "" {
			sf.Namespace = mf.Destination.Namespace
		}
	}

	// IP addresses
	if mf.IP != nil {
		sf.SourceIP = mf.IP.Source
		sf.DestinationIP = mf.IP.Destination
	}

	// Ports and L4 info
	if mf.L4 != nil {
		if mf.L4.TCP != nil {
			sf.DestinationPort = mf.L4.TCP.DestinationPort
			if mf.L4.TCP.Flags != nil {
				sf.TCPFlags = mf.L4.TCP.Flags.String()
			}
		} else if mf.L4.UDP != nil {
			sf.DestinationPort = mf.L4.UDP.DestinationPort
		}
	}

	// L7 info
	if mf.L7 != nil {
		sf.L7Info = mf.L7.Type.String()
	}

	// Traffic direction - determine from source/destination
	if mf.Source != nil && mf.Source.Namespace != "" {
		if mf.Destination != nil && mf.Destination.Namespace == "" {
			sf.TrafficDirection = "egress"
		} else if mf.Destination != nil && mf.Destination.Namespace != "" {
			sf.TrafficDirection = "egress" // Default assumption
		}
	} else if mf.Destination != nil && mf.Destination.Namespace != "" {
		sf.TrafficDirection = "ingress"
	}

	return sf
}
