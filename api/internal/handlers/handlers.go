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

		err := hc.StreamFlowsWithMetricsOnly(
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
	store    *storage.Storage
	config   *utils.AnomalyDetectionConfig
	logger   *logrus.Logger
	upgrader websocket.Upgrader
}

func NewHandlers(store *storage.Storage, config *utils.AnomalyDetectionConfig, logger *logrus.Logger) *Handlers {
	return &Handlers{
		store:  store,
		config: config,
		logger: logger,
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

func (h *Handlers) GetFlowStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.GetFlowStats())
}

func (h *Handlers) GetAlerts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	severity := r.URL.Query().Get("severity")
	ns := r.URL.Query().Get("namespace")
	search := r.URL.Query().Get("search")

	alerts := h.store.GetAlerts(limit, severity, ns, search)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": alerts,
		"total": len(alerts),
	})
}

func (h *Handlers) GetAlert(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	a := h.store.GetAlertByID(id)
	if a == nil {
		writeError(w, http.StatusNotFound, "Alert not found")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handlers) GetAlertsTimeline(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid start time")
			return
		}
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid end time")
			return
		}
	}

	writeJSON(w, http.StatusOK, h.store.GetAlertsTimeline(start, end))
}

func (h *Handlers) StreamAlerts(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Errorf("WS upgrade error: %v", err)
		return
	}
	defer conn.Close()

	severity := r.URL.Query().Get("severity")
	ns := r.URL.Query().Get("namespace")
	typ := r.URL.Query().Get("type")

	sub := &storage.AlertSubscriber{
		ID:      generateID(),
		Channel: make(chan storage.Alert, 100),
		Filter: storage.AlertFilter{
			Severity:  severity,
			Namespace: ns,
			Type:      typ,
		},
		LastSeen: time.Now(),
	}

	h.store.SubscribeAlerts(sub)
	defer h.store.UnsubscribeAlerts(sub)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			conn.WriteMessage(websocket.PingMessage, []byte{})
		}
	}()

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case a := <-sub.Channel:
			if err := conn.WriteJSON(a); err != nil {
				h.logger.Errorf("WS write error: %v", err)
				return
			}
		}
	}
}

func (h *Handlers) GetRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.GetRules())
}

func (h *Handlers) GetRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	rule := h.store.GetRuleByID(id)
	if rule == nil {
		writeError(w, http.StatusNotFound, "Rule not found")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handlers) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var updates storage.Rule
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if !h.store.UpdateRule(id, updates) {
		writeError(w, http.StatusNotFound, "Rule not found")
		return
	}

	writeJSON(w, http.StatusOK, h.store.GetRuleByID(id))
}

func (h *Handlers) GetRulesStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.GetRulesStats())
}

func (h *Handlers) GetMetricsStats(w http.ResponseWriter, r *http.Request) {
	fs := h.store.GetFlowStats()
	ac := len(h.store.GetAlerts(1000, "", "", ""))
	rs := h.store.GetRulesStats()
	cr := len(h.store.GetAlerts(1000, "CRITICAL", "", ""))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"totalFlows":     fs.TotalFlows,
		"totalAlerts":    ac,
		"activeRules":    rs.Enabled,
		"criticalAlerts": cr,
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
