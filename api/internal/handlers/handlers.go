package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"hubble-guard/api/internal/storage"
	"hubble-guard/internal/client"
	"hubble-guard/internal/model"
	"hubble-guard/internal/utils"

	"github.com/gorilla/websocket"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prommodel "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type FlowBroadcaster struct {
	mu           sync.RWMutex
	clients      map[*websocket.Conn]bool
	hubbleClient *client.HubbleGRPCClient
	store        *storage.Storage
	config       *utils.AnomalyDetectionConfig
	logger       *logrus.Logger

	running bool
}

var (
	globalBroadcaster *FlowBroadcaster
	broadcasterOnce   sync.Once
)

func GetFlowBroadcaster() *FlowBroadcaster {
	return globalBroadcaster
}

func InitFlowBroadcaster(
	hubbleClient *client.HubbleGRPCClient,
	store *storage.Storage,
	config *utils.AnomalyDetectionConfig,
	logger *logrus.Logger,
	_ *client.PrometheusMetrics,
) {
	broadcasterOnce.Do(func() {
		globalBroadcaster = &FlowBroadcaster{
			clients:      make(map[*websocket.Conn]bool),
			hubbleClient: hubbleClient,
			store:        store,
			config:       config,
			logger:       logger,
		}

		if hubbleClient != nil {
			globalBroadcaster.Start()
		} else {
			logger.Warn("Hubble client is nil → Broadcaster will not start")
		}
	})
}

func (fb *FlowBroadcaster) Start() {
	fb.mu.Lock()
	if fb.running || fb.hubbleClient == nil {
		fb.mu.Unlock()
		return
	}
	fb.running = true
	fb.mu.Unlock()

	fb.logger.Infof(" Starting global Hubble Flow Stream Broadcaster")

	go fb.run()
}

func (fb *FlowBroadcaster) run() {
	var namespaces []string
	if len(fb.config.Namespaces) > 0 {
		namespaces = fb.config.Namespaces
	} else if fb.config.Application.DefaultNamespace != "" {
		namespaces = []string{fb.config.Application.DefaultNamespace}
	}

	for {
		fb.mu.RLock()
		hc := fb.hubbleClient
		fb.mu.RUnlock()

		if hc == nil {
			fb.logger.Warn("❌ Hubble client became nil → stopping broadcaster loop")
			return
		}

		fb.logger.Infof("🔌 Opening Hubble gRPC Stream for namespaces: %v", namespaces)

		err := hc.StreamFlowsWithMetrics(
			context.Background(),
			namespaces,
			func(ns string) {},
			func(flow *model.Flow) {
				sf := convertModelFlowToStorageFlow(flow)
				fb.store.AddFlow(sf)
				fb.broadcast(sf)
			},
		)

		if err != nil && err != context.Canceled {
			fb.logger.Errorf("⚠️ Hubble stream error: %v → retry in 2s", err)
			time.Sleep(2 * time.Second)
			continue
		}

		fb.logger.Warnf("Hubble stream ended (err=%v). Broadcaster exiting.", err)
		return
	}
}

func (fb *FlowBroadcaster) broadcast(flow storage.Flow) {
	fb.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(fb.clients))
	for c := range fb.clients {
		clients = append(clients, c)
	}
	fb.mu.RUnlock()

	for _, conn := range clients {
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := conn.WriteJSON(flow); err != nil {
			fb.logger.Debugf("🛑 WS send error → removing client: %v", err)
			fb.RemoveClient(conn)
		}
	}
}

func (fb *FlowBroadcaster) AddClient(conn *websocket.Conn) {
	fb.mu.Lock()
	fb.clients[conn] = true
	n := len(fb.clients)
	fb.mu.Unlock()

	fb.logger.Infof("🟢 WebSocket client added → total: %d", n)
}

func (fb *FlowBroadcaster) RemoveClient(conn *websocket.Conn) {
	fb.mu.Lock()
	if _, exists := fb.clients[conn]; exists {
		delete(fb.clients, conn)
		fb.mu.Unlock()
		conn.Close()
	} else {
		fb.mu.Unlock()
	}

	fb.logger.Infof("🔴 WebSocket client removed → total: %d", len(fb.clients))
}

type Handlers struct {
	store      *storage.Storage
	config     *utils.AnomalyDetectionConfig
	logger     *logrus.Logger
	upgrader   websocket.Upgrader
	promClient *client.PrometheusClient
}

func NewHandlers(store *storage.Storage, config *utils.AnomalyDetectionConfig, logger *logrus.Logger, promClient *client.PrometheusClient) *Handlers {
	return &Handlers{
		store:      store,
		config:     config,
		logger:     logger,
		promClient: promClient,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Handlers) GetFlows(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	namespace := r.URL.Query().Get("namespace")
	verdict := r.URL.Query().Get("verdict")
	search := r.URL.Query().Get("search")

	items, total := h.store.GetFlows(page, limit, namespace, verdict, search)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *Handlers) StreamFlows(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Errorf("❌ WS upgrade failed: %v", err)
		return
	}

	h.logger.Infof("🔌 WS client connected: %s", r.RemoteAddr)

	bc := GetFlowBroadcaster()
	if bc == nil {
		h.logger.Error("❌ FlowBroadcaster not initialized")
		conn.WriteJSON(map[string]string{"error": "Broadcaster not available"})
		conn.Close()
		return
	}

	bc.AddClient(conn)

	conn.SetCloseHandler(func(code int, text string) error {
		bc.RemoveClient(conn)
		h.logger.Infof("🔴 WS client closed: %s", r.RemoteAddr)
		return nil
	})

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				bc.RemoveClient(conn)
				return
			}
		}
	}()

	// Block until client is removed
	for {
		time.Sleep(10 * time.Second)
		if _, ok := bc.clients[conn]; !ok {
			return
		}
	}
}

func convertModelFlowToStorageFlow(mf *model.Flow) storage.Flow {
	sf := storage.Flow{
		Timestamp: time.Now(),
		Verdict:   mf.Verdict.String(),
	}

	if mf.Time != nil {
		sf.Timestamp = *mf.Time
	}

	if mf.Source != nil {
		sf.Source = &storage.Endpoint{
			Name:      mf.Source.PodName,
			Namespace: mf.Source.Namespace,
			Identity:  mf.Source.Namespace + "/" + mf.Source.PodName,
		}
		sf.Namespace = mf.Source.Namespace
	}

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

	if mf.IP != nil {
		sf.SourceIP = mf.IP.Source
		sf.DestinationIP = mf.IP.Destination
	}

	if mf.L4 != nil {
		if mf.L4.TCP != nil {
			sf.DestinationPort = mf.L4.TCP.DestinationPort
			if mf.L4.TCP.Flags != nil {
				sf.TCPFlags = mf.L4.TCP.Flags.String()
			}
		}
		if mf.L4.UDP != nil {
			sf.DestinationPort = mf.L4.UDP.DestinationPort
		}
	}

	if mf.Source != nil && mf.Source.Namespace != "" {
		sf.TrafficDirection = "egress"
	} else if mf.Destination != nil {
		sf.TrafficDirection = "ingress"
	}

	return sf
}

func (h *Handlers) GetPrometheusStats(w http.ResponseWriter, r *http.Request) {
	if h.promClient == nil {
		writeError(w, http.StatusServiceUnavailable, "Prometheus client not available")
		return
	}

	ctx := r.Context()
	timeout := time.Duration(h.config.Prometheus.TimeoutSeconds) * time.Second

	response := map[string]interface{}{
		"totalFlows":     float64(0),
		"totalAlerts":    float64(0),
		"criticalAlerts": float64(0),
		"tcpConnections": float64(0),
	}
	errors := make(map[string]string)

	// Query total flows - try sum() first, fallback to direct query
	query := "sum(hubble_flows_total)"
	h.logger.Debugf("Querying Prometheus: %s", query)
	if result, err := h.promClient.Query(ctx, query, timeout); err == nil {
		var totalFlowsValue float64
		found := false

		// Handle Vector result
		if vector, ok := result.(prommodel.Vector); ok {
			if len(vector) > 0 {
				totalFlowsValue = float64(vector[0].Value)
				found = true
				h.logger.Debugf("Total flows result (Vector): %v", vector[0].Value)
			}
		} else if scalar, ok := result.(*prommodel.Scalar); ok {
			// Handle Scalar result
			totalFlowsValue = float64(scalar.Value)
			found = true
			h.logger.Debugf("Total flows result (Scalar): %v", scalar.Value)
		}

		if found {
			response["totalFlows"] = totalFlowsValue
		} else {
			// Fallback: try direct query without sum
			h.logger.Debugf("Query '%s' returned empty, trying direct query", query)
			directQuery := "hubble_flows_total"
			if directResult, directErr := h.promClient.Query(ctx, directQuery, timeout); directErr == nil {
				if directVector, ok := directResult.(prommodel.Vector); ok && len(directVector) > 0 {
					// Sum manually
					var sum float64
					for _, sample := range directVector {
						sum += float64(sample.Value)
					}
					response["totalFlows"] = sum
					h.logger.Debugf("Total flows (summed from direct query): %v", sum)
				} else {
					h.logger.Debugf("Direct query '%s' also returned empty", directQuery)
					errors["totalFlows"] = "No data returned"
				}
			} else {
				h.logger.Warnf("Direct query '%s' failed: %v", directQuery, directErr)
				errors["totalFlows"] = "No data returned"
			}
		}
	} else {
		h.logger.Errorf("Failed to query '%s': %v", query, err)
		errors["totalFlows"] = err.Error()
	}

	// Query total alerts
	query = "sum(hubble_guard_alerts_total)"
	h.logger.Debugf("Querying Prometheus: %s", query)
	if result, err := h.promClient.Query(ctx, query, timeout); err == nil {
		var totalAlertsValue float64
		found := false

		if vector, ok := result.(prommodel.Vector); ok {
			if len(vector) > 0 {
				totalAlertsValue = float64(vector[0].Value)
				found = true
				h.logger.Debugf("Total alerts result: %v", vector[0].Value)
			}
		} else if scalar, ok := result.(*prommodel.Scalar); ok {
			totalAlertsValue = float64(scalar.Value)
			found = true
			h.logger.Debugf("Total alerts result (Scalar): %v", scalar.Value)
		}

		if found {
			response["totalAlerts"] = totalAlertsValue
		} else {
			// Fallback: try direct query
			directQuery := "hubble_guard_alerts_total"
			if directResult, directErr := h.promClient.Query(ctx, directQuery, timeout); directErr == nil {
				if directVector, ok := directResult.(prommodel.Vector); ok && len(directVector) > 0 {
					var sum float64
					for _, sample := range directVector {
						sum += float64(sample.Value)
					}
					response["totalAlerts"] = sum
					h.logger.Debugf("Total alerts (summed): %v", sum)
				} else {
					errors["totalAlerts"] = "No data returned"
				}
			} else {
				errors["totalAlerts"] = "No data returned"
			}
		}
	} else {
		h.logger.Errorf("Failed to query '%s': %v", query, err)
		errors["totalAlerts"] = err.Error()
	}

	// Query critical alerts
	query = `sum(hubble_guard_alerts_total{severity="CRITICAL"})`
	h.logger.Debugf("Querying Prometheus: %s", query)
	if result, err := h.promClient.Query(ctx, query, timeout); err == nil {
		var criticalAlertsValue float64
		found := false

		if vector, ok := result.(prommodel.Vector); ok {
			if len(vector) > 0 {
				criticalAlertsValue = float64(vector[0].Value)
				found = true
				h.logger.Debugf("Critical alerts result: %v", vector[0].Value)
			}
		} else if scalar, ok := result.(*prommodel.Scalar); ok {
			criticalAlertsValue = float64(scalar.Value)
			found = true
			h.logger.Debugf("Critical alerts result (Scalar): %v", scalar.Value)
		}

		if found {
			response["criticalAlerts"] = criticalAlertsValue
		} else {
			// Fallback: try direct query with filter
			directQuery := `hubble_guard_alerts_total{severity="CRITICAL"}`
			if directResult, directErr := h.promClient.Query(ctx, directQuery, timeout); directErr == nil {
				if directVector, ok := directResult.(prommodel.Vector); ok && len(directVector) > 0 {
					var sum float64
					for _, sample := range directVector {
						sum += float64(sample.Value)
					}
					response["criticalAlerts"] = sum
					h.logger.Debugf("Critical alerts (summed): %v", sum)
				} else {
					errors["criticalAlerts"] = "No data returned"
				}
			} else {
				errors["criticalAlerts"] = "No data returned"
			}
		}
	} else {
		h.logger.Errorf("Failed to query '%s': %v", query, err)
		errors["criticalAlerts"] = err.Error()
	}

	// Query TCP connections total
	query = "sum(hubble_tcp_connections_total)"
	h.logger.Debugf("Querying Prometheus: %s", query)
	if result, err := h.promClient.Query(ctx, query, timeout); err == nil {
		var tcpConnectionsValue float64
		found := false

		if vector, ok := result.(prommodel.Vector); ok {
			if len(vector) > 0 {
				tcpConnectionsValue = float64(vector[0].Value)
				found = true
				h.logger.Debugf("TCP connections result: %v", vector[0].Value)
			}
		} else if scalar, ok := result.(*prommodel.Scalar); ok {
			tcpConnectionsValue = float64(scalar.Value)
			found = true
			h.logger.Debugf("TCP connections result (Scalar): %v", scalar.Value)
		}

		if found {
			response["tcpConnections"] = tcpConnectionsValue
		} else {
			// Fallback: try direct query
			directQuery := "hubble_tcp_connections_total"
			if directResult, directErr := h.promClient.Query(ctx, directQuery, timeout); directErr == nil {
				if directVector, ok := directResult.(prommodel.Vector); ok && len(directVector) > 0 {
					var sum float64
					for _, sample := range directVector {
						sum += float64(sample.Value)
					}
					response["tcpConnections"] = sum
					h.logger.Debugf("TCP connections (summed): %v", sum)
				} else {
					errors["tcpConnections"] = "No data returned"
				}
			} else {
				errors["tcpConnections"] = "No data returned"
			}
		}
	} else {
		h.logger.Errorf("Failed to query '%s': %v", query, err)
		errors["tcpConnections"] = err.Error()
	}

	// Include errors in response if any (for debugging)
	if len(errors) > 0 {
		response["_errors"] = errors
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handlers) GetDroppedFlowsTimeSeries(w http.ResponseWriter, r *http.Request) {
	if h.promClient == nil {
		writeError(w, http.StatusServiceUnavailable, "Prometheus client not available")
		return
	}

	ctx := r.Context()
	timeout := time.Duration(h.config.Prometheus.TimeoutSeconds) * time.Second

	// Parse query parameters
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	stepStr := r.URL.Query().Get("step")

	// Default: last 1 hour, 15 second steps
	end := time.Now()
	start := end.Add(-1 * time.Hour)
	step := 15 * time.Second

	if startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = parsed
		}
	}
	if endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = parsed
		}
	}
	if stepStr != "" {
		if parsed, err := time.ParseDuration(stepStr); err == nil {
			step = parsed
		}
	}

	// Query dropped flows time-series
	query := `sum(hubble_flows_by_verdict_total{verdict="DROPPED"}) by (namespace)`

	// Use QueryRange for time-series data
	rangeQuery := v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}

	h.logger.Debugf("Querying Prometheus range: %s from %v to %v, step: %v", query, start, end, step)

	result, err := h.promClient.QueryRange(ctx, query, rangeQuery, timeout)
	if err != nil {
		h.logger.Errorf("Failed to query dropped flows time-series: %v", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to query Prometheus: %v", err))
		return
	}

	// Convert result to JSON format
	timeSeriesData := []map[string]interface{}{}

	if matrix, ok := result.(prommodel.Matrix); ok {
		for _, series := range matrix {
			// Get namespace label
			namespace := "default"
			if ns, exists := series.Metric["namespace"]; exists {
				namespace = string(ns)
			}

			// Convert samples to time-series points
			points := []map[string]interface{}{}
			for _, sample := range series.Values {
				points = append(points, map[string]interface{}{
					"timestamp": sample.Timestamp.Unix(),
					"value":     float64(sample.Value),
				})
			}

			timeSeriesData = append(timeSeriesData, map[string]interface{}{
				"namespace": namespace,
				"metric":    series.Metric,
				"values":    points,
			})
		}
	} else {
		h.logger.Warnf("Unexpected result type for time-series: %T", result)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"query": query,
		"start": start.Unix(),
		"end":   end.Unix(),
		"step":  step.Seconds(),
		"data":  timeSeriesData,
	})
}

func (h *Handlers) GetAlertTypesStats(w http.ResponseWriter, r *http.Request) {
	if h.promClient == nil {
		writeError(w, http.StatusServiceUnavailable, "Prometheus client not available")
		return
	}

	ctx := r.Context()
	timeout := time.Duration(h.config.Prometheus.TimeoutSeconds) * time.Second

	// Query alert types - sum by type
	query := `sum(hubble_guard_alerts_total) by (type)`
	h.logger.Debugf("Querying Prometheus for alert types: %s", query)

	result, err := h.promClient.Query(ctx, query, timeout)
	if err != nil {
		h.logger.Errorf("Failed to query alert types: %v", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to query Prometheus: %v", err))
		return
	}

	// Convert result to JSON format
	alertTypes := []map[string]interface{}{}

	if vector, ok := result.(prommodel.Vector); ok {
		for _, sample := range vector {
			alertType := "unknown"
			if typeVal, exists := sample.Metric["type"]; exists {
				alertType = string(typeVal)
			}

			alertTypes = append(alertTypes, map[string]interface{}{
				"type":  alertType,
				"value": float64(sample.Value),
			})
		}
	} else {
		h.logger.Warnf("Unexpected result type for alert types: %T", result)
	}

	// Also query by severity for more details
	severityQuery := `sum(hubble_guard_alerts_total) by (type, severity)`
	h.logger.Debugf("Querying Prometheus for alert types by severity: %s", severityQuery)

	severityResult, err := h.promClient.Query(ctx, severityQuery, timeout)
	severityData := []map[string]interface{}{}

	if err == nil {
		if vector, ok := severityResult.(prommodel.Vector); ok {
			for _, sample := range vector {
				alertType := "unknown"
				severity := "unknown"

				if typeVal, exists := sample.Metric["type"]; exists {
					alertType = string(typeVal)
				}
				if sevVal, exists := sample.Metric["severity"]; exists {
					severity = string(sevVal)
				}

				severityData = append(severityData, map[string]interface{}{
					"type":     alertType,
					"severity": severity,
					"value":    float64(sample.Value),
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"byType":     alertTypes,
		"bySeverity": severityData,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
