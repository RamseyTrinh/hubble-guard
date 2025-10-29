package alert

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"hubble-anomaly-detector/internal/client"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// PrometheusExporter quản lý việc expose metrics qua HTTP endpoint
type PrometheusExporter struct {
	server  *http.Server
	metrics *client.PrometheusMetrics
	logger  *logrus.Logger
	port    string
}

// NewPrometheusExporter tạo instance mới của PrometheusExporter
func NewPrometheusExporter(port string, logger *logrus.Logger) *PrometheusExporter {
	metrics := client.NewPrometheusMetrics()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<h1>Hubble Prometheus Exporter</h1>
			<p><a href="/metrics">Metrics</a></p>
			<p><a href="/health">Health Check</a></p>
		`))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	return &PrometheusExporter{
		server:  server,
		metrics: metrics,
		logger:  logger,
		port:    port,
	}
}

// Start khởi động Prometheus exporter server
func (e *PrometheusExporter) Start(ctx context.Context) error {
	e.logger.Infof("Starting Prometheus exporter on port %s", e.port)
	e.logger.Infof("Metrics available at: http://localhost:%s/metrics", e.port)
	e.logger.Infof("Health check available at: http://localhost:%s/health", e.port)

	go func() {
		if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			e.logger.Errorf("Failed to start Prometheus exporter: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	e.logger.Info("Shutting down Prometheus exporter...")
	return e.server.Shutdown(shutdownCtx)
}

// Stop dừng Prometheus exporter server
func (e *PrometheusExporter) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return e.server.Shutdown(ctx)
}

// GetMetrics trả về instance của PrometheusMetrics
func (e *PrometheusExporter) GetMetrics() *client.PrometheusMetrics {
	return e.metrics
}

// PrometheusCollector implements prometheus.Collector interface
type PrometheusCollector struct {
	metrics *client.PrometheusMetrics
}

// NewPrometheusCollector tạo instance mới của PrometheusCollector
func NewPrometheusCollector(metrics *client.PrometheusMetrics) *PrometheusCollector {
	return &PrometheusCollector{
		metrics: metrics,
	}
}

// Describe implements prometheus.Collector
func (c *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.FlowTotal.Describe(ch)
	c.metrics.FlowByVerdict.Describe(ch)
	c.metrics.FlowByProtocol.Describe(ch)
	c.metrics.FlowByNamespace.Describe(ch)
	c.metrics.FlowBySource.Describe(ch)
	c.metrics.FlowByDestination.Describe(ch)
	c.metrics.TCPConnections.Describe(ch)
	c.metrics.TCPFlags.Describe(ch)
	c.metrics.TCPBytes.Describe(ch)
	c.metrics.UDPPackets.Describe(ch)
	c.metrics.UDPBytes.Describe(ch)
	c.metrics.L7Requests.Describe(ch)
	c.metrics.L7ByType.Describe(ch)
	c.metrics.FlowErrors.Describe(ch)
	c.metrics.ConnectionErrors.Describe(ch)
	c.metrics.FlowProcessingTime.Describe(ch)
	c.metrics.ActiveConnections.Describe(ch)
}

// Collect implements prometheus.Collector
func (c *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	c.metrics.FlowTotal.Collect(ch)
	c.metrics.FlowByVerdict.Collect(ch)
	c.metrics.FlowByProtocol.Collect(ch)
	c.metrics.FlowByNamespace.Collect(ch)
	c.metrics.FlowBySource.Collect(ch)
	c.metrics.FlowByDestination.Collect(ch)
	c.metrics.TCPConnections.Collect(ch)
	c.metrics.TCPFlags.Collect(ch)
	c.metrics.TCPBytes.Collect(ch)
	c.metrics.UDPPackets.Collect(ch)
	c.metrics.UDPBytes.Collect(ch)
	c.metrics.L7Requests.Collect(ch)
	c.metrics.L7ByType.Collect(ch)
	c.metrics.FlowErrors.Collect(ch)
	c.metrics.ConnectionErrors.Collect(ch)
	c.metrics.FlowProcessingTime.Collect(ch)
	c.metrics.ActiveConnections.Collect(ch)
}

// RegisterCustomMetrics đăng ký custom metrics với Prometheus registry
func RegisterCustomMetrics(registry prometheus.Registerer, metrics *client.PrometheusMetrics) error {
	collector := NewPrometheusCollector(metrics)
	return registry.Register(collector)
}

// CreateCustomRegistry tạo Prometheus registry tùy chỉnh
func CreateCustomRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()

	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	return registry
}

// StartPrometheusExporterWithCustomRegistry khởi động exporter với custom registry
func StartPrometheusExporterWithCustomRegistry(port string, logger *logrus.Logger) (*PrometheusExporter, error) {
	metrics := client.NewPrometheusMetrics()
	registry := CreateCustomRegistry()

	if err := RegisterCustomMetrics(registry, metrics); err != nil {
		return nil, fmt.Errorf("failed to register custom metrics: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<h1>Hubble Prometheus Exporter</h1>
			<p><a href="/metrics">Metrics</a></p>
			<p><a href="/health">Health Check</a></p>
		`))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	exporter := &PrometheusExporter{
		server:  server,
		metrics: metrics,
		logger:  logger,
		port:    port,
	}

	return exporter, nil
}
