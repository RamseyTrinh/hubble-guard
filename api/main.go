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

	"hubble-guard/api/internal/handlers"
	"hubble-guard/api/internal/storage"
	"hubble-guard/internal/client"
	"hubble-guard/internal/utils"

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

	// Create Hubble client for global streaming (used only by FlowBroadcaster)
	var hubbleClient *client.HubbleGRPCClient
	hubbleClient, err = client.NewHubbleGRPCClientWithMetrics(config.Application.HubbleServer, sharedMetrics)
	if err != nil {
		logger.Warnf("Failed to create Hubble client for WebSocket streaming: %v", err)
		logger.Warn("WebSocket flow streaming will not be available")
		hubbleClient = nil
	} else {
		// Test connection once
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := hubbleClient.TestConnection(ctx); err != nil {
			logger.Warnf("Hubble connection test failed: %v", err)
			logger.Warn("WebSocket flow streaming will not be available")
			hubbleClient.Close()
			hubbleClient = nil
		}
		cancel()
	}

	// Initialize flow broadcaster (single gRPC stream for all WebSocket clients)
	// FlowBroadcaster sẽ giữ hubbleClient và tự quản lý vòng đời stream.
	handlers.InitFlowBroadcaster(hubbleClient, store, config, logger, sharedMetrics)

	// Create Prometheus client for metrics queries
	var promClient *client.PrometheusClient
	promClient, err = client.NewPrometheusClient(config.Prometheus.URL)
	if err != nil {
		logger.Warnf("Failed to create Prometheus client: %v", err)
		logger.Warn("Prometheus metrics queries will not be available")
		promClient = nil
	} else {
		logger.Infof("Prometheus client connected to %s", config.Prometheus.URL)
	}

	// Create HTTP handlers (Handlers KHÔNG giữ hubbleClient nữa)
	h := handlers.NewHandlers(store, config, logger, promClient)

	// Setup router
	router := mux.NewRouter()

	// CORS middleware
	router.Use(corsMiddleware)

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Flows endpoints
	api.HandleFunc("/flows/stats", h.GetFlowStats).Methods("GET")
	api.HandleFunc("/stream/flows", h.StreamFlows).Methods("GET")
	api.HandleFunc("/flows", h.GetFlows).Methods("GET")

	// Alerts endpoints
	api.HandleFunc("/alerts/timeline", h.GetAlertsTimeline).Methods("GET")
	api.HandleFunc("/stream/alerts", h.StreamAlerts).Methods("GET")
	api.HandleFunc("/alerts", h.GetAlerts).Methods("GET")
	api.HandleFunc("/alerts/{id}", h.GetAlert).Methods("GET")

	// Rules endpoints
	api.HandleFunc("/rules/stats", h.GetRulesStats).Methods("GET")
	api.HandleFunc("/rules", h.GetRules).Methods("GET")
	api.HandleFunc("/rules/{id}", h.GetRule).Methods("GET")
	api.HandleFunc("/rules/{id}", h.UpdateRule).Methods("PUT")

	// Metrics endpoints
	api.HandleFunc("/metrics/prometheus/stats", h.GetPrometheusStats).Methods("GET")
	api.HandleFunc("/metrics/prometheus/test", h.TestPrometheusConnection).Methods("GET")
	api.HandleFunc("/metrics/prometheus/dropped-flows/timeseries", h.GetDroppedFlowsTimeSeries).Methods("GET")
	api.HandleFunc("/metrics/prometheus/alert-types", h.GetAlertTypesStats).Methods("GET")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}).Methods("GET", "OPTIONS")

	// Start background task to sync rules from config
	go syncRulesFromConfig(store, config, logger)

	// Start server
	addr := fmt.Sprintf(":%s", *port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
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

		allowedOrigins := []string{
			"http://localhost:5000",
			"http://localhost:3000",
			"http://127.0.0.1:5000",
			"http://127.0.0.1:3000",
		}

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

		if allowOrigin != "*" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

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
