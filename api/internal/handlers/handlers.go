package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"hubble-anomaly-detector/api/internal/storage"
	"hubble-anomaly-detector/internal/client"
	"hubble-anomaly-detector/internal/model"
	"hubble-anomaly-detector/internal/utils"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type Handlers struct {
	store        *storage.Storage
	config       *utils.AnomalyDetectionConfig
	logger       *logrus.Logger
	upgrader     websocket.Upgrader
	hubbleClient *client.HubbleGRPCClient
}

func NewHandlers(store *storage.Storage, config *utils.AnomalyDetectionConfig, logger *logrus.Logger, hubbleClient *client.HubbleGRPCClient) *Handlers {
	return &Handlers{
		store:        store,
		config:       config,
		logger:       logger,
		hubbleClient: hubbleClient,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for development
				origin := r.Header.Get("Origin")
				logger.Debugf("WebSocket origin check: %s", origin)
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// Flows handlers
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

	flows, total := h.store.GetFlows(page, limit, namespace, verdict, search)

	response := map[string]interface{}{
		"items": flows,
		"total": total,
		"page":  page,
		"limit": limit,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handlers) GetFlow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	flow := h.store.GetFlowByID(id)
	if flow == nil {
		writeError(w, http.StatusNotFound, "Flow not found")
		return
	}

	writeJSON(w, http.StatusOK, flow)
}

func (h *Handlers) GetFlowStats(w http.ResponseWriter, r *http.Request) {
	stats := h.store.GetFlowStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handlers) StreamFlows(w http.ResponseWriter, r *http.Request) {
	h.logger.Infof("WebSocket connection attempt from %s", r.RemoteAddr)

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Errorf("WebSocket upgrade error: %v", err)
		return
	}

	h.logger.Infof("WebSocket connection established from %s", r.RemoteAddr)
	defer func() {
		h.logger.Debugf("WebSocket connection closed for %s", r.RemoteAddr)
		conn.Close()
	}()

	// Set initial deadlines
	conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Set pong handler to reset read deadline
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(24 * time.Hour))
		return nil
	})

	// Send initial connection message to test the connection
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(map[string]string{"type": "connected", "message": "WebSocket connection established"}); err != nil {
		h.logger.Errorf("Failed to send initial message: %v", err)
		return
	}
	h.logger.Debugf("Sent initial connection message")

	// Channel to signal connection close
	done := make(chan struct{})
	once := &sync.Once{}

	// Helper to safely close done channel
	closeDone := func() {
		once.Do(func() {
			close(done)
		})
	}

	// Send ping to keep connection alive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	go func() {
		defer closeDone()
		for {
			select {
			case <-pingTicker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					h.logger.Debugf("Ping failed: %v", err)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Read messages in background to detect connection close (non-blocking)
	// This goroutine will exit when client closes connection
	go func() {
		defer closeDone()
		for {
			// Don't set read deadline - let it block until client sends something or closes
			_, _, err := conn.ReadMessage()
			if err != nil {
				// Connection closed - this is expected when client disconnects
				return
			}
			// If we receive a message (unlikely from client), just continue
		}
	}()

	// Stream flows directly from Hubble gRPC (like hubble-ui does)
	// This avoids storing flows in memory and provides real-time streaming
	if h.hubbleClient == nil {
		h.logger.Errorf("Hubble client not available for WebSocket streaming")
		conn.WriteJSON(map[string]string{"type": "error", "message": "Hubble client not available"})
		return
	}

	// Get namespace filter from query params
	namespace := r.URL.Query().Get("namespace")
	var namespaces []string
	if namespace != "" {
		namespaces = []string{namespace}
	} else if len(h.config.Namespaces) > 0 {
		namespaces = h.config.Namespaces
	} else if h.config.Application.DefaultNamespace != "" {
		namespaces = []string{h.config.Application.DefaultNamespace}
	}

	// Create context for this WebSocket connection
	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()

	// Use channel to safely pass flows between goroutines
	flowChan := make(chan storage.Flow, 100) // Buffer up to 100 flows

	// Stream flows from Hubble gRPC directly to WebSocket
	go func() {
		defer close(flowChan)
		err := h.hubbleClient.StreamFlowsWithMetricsOnly(streamCtx, namespaces, func(ns string) {
			// Flow counter callback (optional)
		}, func(flow *model.Flow) {
			// Convert model.Flow to storage.Flow
			sf := convertModelFlowToStorageFlow(flow)

			// Also store in storage for REST API queries
			h.store.AddFlow(sf)

			// Send to channel for WebSocket
			select {
			case <-done:
				return
			case flowChan <- sf:
				// Successfully sent to channel
			default:
				// Channel full, skip this flow (avoid blocking)
				h.logger.Debugf("Flow channel full, dropping flow")
			}
		})

		if err != nil && err != context.Canceled {
			h.logger.Errorf("Hubble stream error: %v", err)
		}
	}()

	// Throttle: batch flows and send every 100ms
	throttleTicker := time.NewTicker(100 * time.Millisecond)
	defer throttleTicker.Stop()

	flowBuffer := make([]storage.Flow, 0, 10) // Buffer up to 10 flows per batch

	// Send buffered flows with throttling
	for {
		select {
		case <-done:
			h.logger.Debugf("WebSocket connection closed")
			return
		case flow := <-flowChan:
			// Add to buffer
			flowBuffer = append(flowBuffer, flow)
			// If buffer is full, send immediately
			if len(flowBuffer) >= 10 {
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				flowsToSend := make([]storage.Flow, len(flowBuffer))
				copy(flowsToSend, flowBuffer)
				flowBuffer = flowBuffer[:0] // Clear buffer

				if err := conn.WriteJSON(flowsToSend); err != nil {
					h.logger.Debugf("WebSocket write error: %v", err)
					streamCancel() // Cancel Hubble stream
					return
				}
				h.logger.Debugf("Sent batch of %d flows", len(flowsToSend))
			}
		case <-throttleTicker.C:
			// Send buffered flows periodically
			if len(flowBuffer) > 0 {
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				flowsToSend := make([]storage.Flow, len(flowBuffer))
				copy(flowsToSend, flowBuffer)
				flowBuffer = flowBuffer[:0] // Clear buffer

				if err := conn.WriteJSON(flowsToSend); err != nil {
					h.logger.Debugf("WebSocket write error: %v", err)
					streamCancel() // Cancel Hubble stream
					return
				}
				h.logger.Debugf("Sent batch of %d flows", len(flowsToSend))
			}
		}
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

// Alerts handlers
func (h *Handlers) GetAlerts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	severity := r.URL.Query().Get("severity")
	namespace := r.URL.Query().Get("namespace")
	search := r.URL.Query().Get("search")

	alerts := h.store.GetAlerts(limit, severity, namespace, search)

	response := map[string]interface{}{
		"items": alerts,
		"total": len(alerts),
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handlers) GetAlert(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	alert := h.store.GetAlertByID(id)
	if alert == nil {
		writeError(w, http.StatusNotFound, "Alert not found")
		return
	}

	writeJSON(w, http.StatusOK, alert)
}

func (h *Handlers) GetAlertsTimeline(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid start time format")
			return
		}
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid end time format")
			return
		}
	}

	alerts := h.store.GetAlertsTimeline(start, end)
	writeJSON(w, http.StatusOK, alerts)
}

func (h *Handlers) StreamAlerts(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Errorf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	severity := r.URL.Query().Get("severity")
	namespace := r.URL.Query().Get("namespace")
	alertType := r.URL.Query().Get("type")

	sub := &storage.AlertSubscriber{
		ID:      generateID(),
		Channel: make(chan storage.Alert, 100),
		Filter: storage.AlertFilter{
			Severity:  severity,
			Namespace: namespace,
			Type:      alertType,
		},
		LastSeen: time.Now(),
	}

	h.store.SubscribeAlerts(sub)
	defer h.store.UnsubscribeAlerts(sub)

	// Send ping to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}()

	// Read messages (for pong)
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Send alerts
	for {
		select {
		case alert := <-sub.Channel:
			if err := conn.WriteJSON(alert); err != nil {
				h.logger.Errorf("WebSocket write error: %v", err)
				return
			}
		case <-ticker.C:
			// Keep connection alive
		}
	}
}

// Rules handlers
func (h *Handlers) GetRules(w http.ResponseWriter, r *http.Request) {
	rules := h.store.GetRules()
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handlers) GetRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	rule := h.store.GetRuleByID(id)
	if rule == nil {
		writeError(w, http.StatusNotFound, "Rule not found")
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

func (h *Handlers) UpdateRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var updates storage.Rule
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if !h.store.UpdateRule(id, updates) {
		writeError(w, http.StatusNotFound, "Rule not found")
		return
	}

	rule := h.store.GetRuleByID(id)
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handlers) GetRulesStats(w http.ResponseWriter, r *http.Request) {
	stats := h.store.GetRulesStats()
	writeJSON(w, http.StatusOK, stats)
}

// Metrics handlers
func (h *Handlers) GetMetricsStats(w http.ResponseWriter, r *http.Request) {
	flowStats := h.store.GetFlowStats()
	alertCount := len(h.store.GetAlerts(1000, "", "", ""))
	rulesStats := h.store.GetRulesStats()

	criticalAlerts := len(h.store.GetAlerts(1000, "CRITICAL", "", ""))

	stats := map[string]interface{}{
		"totalFlows":     flowStats.TotalFlows,
		"totalAlerts":    alertCount,
		"activeRules":    rulesStats.Enabled,
		"criticalAlerts": criticalAlerts,
	}

	writeJSON(w, http.StatusOK, stats)
}

// Helper functions
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

func setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
