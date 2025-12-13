package client

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"hubble-guard/internal/model"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PrometheusMetrics struct {
	// Flow metrics
	FlowTotal       *prometheus.CounterVec
	FlowByVerdict   *prometheus.CounterVec
	FlowByProtocol  *prometheus.CounterVec
	FlowByNamespace *prometheus.CounterVec
	// TCP metrics
	TCPConnections *prometheus.CounterVec
	TCPFlags       *prometheus.CounterVec
	TCPBytes       *prometheus.CounterVec

	ConnectionErrors       *prometheus.CounterVec
	TrafficSpikeMultiplier *prometheus.GaugeVec
	NewDestinations        *prometheus.CounterVec
	ErrorResponseRate      *prometheus.CounterVec
	TCPDropRate            *prometheus.CounterVec
	PortScanDistinctPorts  *prometheus.GaugeVec
	NamespaceAccess        *prometheus.CounterVec
	SuspiciousOutbound     *prometheus.CounterVec
	SourceDestTraffic      *prometheus.CounterVec
	AlertCounter           *prometheus.CounterVec
	portScanTracker        *portScanTracker
}

type portScanEntry struct {
	ports map[uint16]time.Time
}

type portScanTracker struct {
	entries    map[string]*portScanEntry
	mu         sync.RWMutex
	window     time.Duration
	metricKeys map[string]string
}

func newPortScanTracker() *portScanTracker {
	return &portScanTracker{
		entries:    make(map[string]*portScanEntry),
		metricKeys: make(map[string]string),
		window:     10 * time.Second,
	}
}

func (pst *portScanTracker) addPort(sourceIP, destIP string, port uint16) {
	key := fmt.Sprintf("%s:%s", sourceIP, destIP)
	now := time.Now()

	pst.mu.Lock()
	defer pst.mu.Unlock()

	entry, exists := pst.entries[key]
	if !exists {
		entry = &portScanEntry{
			ports: make(map[uint16]time.Time),
		}
		pst.entries[key] = entry
	}

	entry.ports[port] = now
}

func (pst *portScanTracker) getDistinctPortCount(sourceIP, destIP string) int {
	key := fmt.Sprintf("%s:%s", sourceIP, destIP)
	now := time.Now()

	pst.mu.Lock()
	defer pst.mu.Unlock()

	entry, exists := pst.entries[key]
	if !exists {
		return 0
	}

	count := 0
	portsToDelete := []uint16{}
	for port, timestamp := range entry.ports {
		if now.Sub(timestamp) <= pst.window {
			count++
		} else {
			portsToDelete = append(portsToDelete, port)
		}
	}

	for _, port := range portsToDelete {
		delete(entry.ports, port)
	}

	if len(entry.ports) == 0 {
		delete(pst.entries, key)
	}

	return count
}

func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		FlowTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_flows_total",
				Help: "Total number of flows processed",
			},
			[]string{"namespace"},
		),

		FlowByVerdict: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_flows_by_verdict_total",
				Help: "Total number of flows by verdict",
			},
			[]string{"verdict", "namespace"},
		),

		FlowByProtocol: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_flows_by_protocol_total",
				Help: "Total number of flows by protocol",
			},
			[]string{"protocol", "namespace"},
		),

		FlowByNamespace: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_flows_by_namespace_total",
				Help: "Total number of flows by namespace",
			},
			[]string{"namespace"},
		),

		TCPConnections: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_tcp_connections_total",
				Help: "Total number of TCP connections",
			},
			[]string{"namespace", "source_ip", "destination_ip"},
		),

		TCPFlags: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_tcp_flags_total",
				Help: "Total number of TCP flags",
			},
			[]string{"flag", "namespace"},
		),

		TCPBytes: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_tcp_bytes_total",
				Help: "Total number of TCP bytes",
			},
			[]string{"namespace", "direction"},
		),

		ConnectionErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_connection_errors_total",
				Help: "Total number of connection errors",
			},
			[]string{"error_type"},
		),

		TrafficSpikeMultiplier: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hubble_traffic_spike_multiplier",
				Help: "Current traffic rate as multiplier of baseline",
			},
			[]string{"namespace"},
		),

		NewDestinations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_new_destinations_total",
				Help: "Total number of new destination connections",
			},
			[]string{"source_ip", "destination_ip", "namespace"},
		),

		ErrorResponseRate: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_error_responses_total",
				Help: "Total number of error responses",
			},
			[]string{"namespace", "error_type"},
		),

		TCPDropRate: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_tcp_drops_total",
				Help: "Total number of TCP drops",
			},
			[]string{"namespace", "source_ip", "destination_ip"},
		),

		PortScanDistinctPorts: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "portscan_distinct_ports_10s",
				Help: "Number of distinct destination ports in the last 10 seconds per source-dest pair",
			},
			[]string{"source_ip", "dest_ip", "namespace"},
		),

		NamespaceAccess: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "namespace_access_total",
				Help: "Total number of cross-namespace access attempts",
			},
			[]string{"source_namespace", "dest_namespace", "dest_service", "dest_pod"},
		),

		SuspiciousOutbound: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_suspicious_outbound_total",
				Help: "Total number of suspicious outbound connections to suspicious ports",
			},
			[]string{"namespace", "destination_port"},
		),

		SourceDestTraffic: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "source_dest_traffic_total",
				Help: "Total traffic between source and destination pods",
			},
			[]string{"namespace", "source_pod", "dest_pod", "dest_service"},
		),

		AlertCounter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_guard_alerts_total",
				Help: "Total alerts detected",
			},
			[]string{"namespace", "severity", "type"},
		),

		portScanTracker: newPortScanTracker(),
	}
}

func (m *PrometheusMetrics) RecordFlow(flow *model.Flow) {
	if flow == nil {
		return
	}

	namespace := "unknown"
	if flow.Source != nil && flow.Source.Namespace != "" {
		namespace = flow.Source.Namespace
	} else if flow.Destination != nil && flow.Destination.Namespace != "" {
		namespace = flow.Destination.Namespace
	}

	m.FlowTotal.WithLabelValues(namespace).Inc()

	verdict := flow.Verdict.String()
	m.FlowByVerdict.WithLabelValues(verdict, namespace).Inc()

	m.FlowByNamespace.WithLabelValues(namespace).Inc()

	if flow.L4 != nil {
		if flow.L4.TCP != nil {
			protocol := "tcp"
			m.FlowByProtocol.WithLabelValues(protocol, namespace).Inc()

			if flow.IP != nil {
				sourceIP := flow.IP.Source
				destIP := flow.IP.Destination
				m.TCPConnections.WithLabelValues(namespace, sourceIP, destIP).Inc()

				if flow.L4.TCP.Flags != nil {
					flags := flow.L4.TCP.Flags
					if flags.SYN {
						m.TCPFlags.WithLabelValues("SYN", namespace).Inc()
					}
					if flags.ACK {
						m.TCPFlags.WithLabelValues("ACK", namespace).Inc()
					}
					if flags.FIN {
						m.TCPFlags.WithLabelValues("FIN", namespace).Inc()
					}
					if flags.RST {
						m.TCPFlags.WithLabelValues("RST", namespace).Inc()
					}
					if flags.PSH {
						m.TCPFlags.WithLabelValues("PSH", namespace).Inc()
					}
					if flags.URG {
						m.TCPFlags.WithLabelValues("URG", namespace).Inc()
					}
				}

				if flow.L4.TCP.Bytes > 0 {
					m.TCPBytes.WithLabelValues(namespace, "outbound").Add(float64(flow.L4.TCP.Bytes))
				}
			}
		} else if flow.L4.UDP != nil {
			protocol := "udp"
			m.FlowByProtocol.WithLabelValues(protocol, namespace).Inc()
		}
	}

	if flow.Source != nil && flow.Destination != nil {
		sourceNS := flow.Source.Namespace
		destNS := flow.Destination.Namespace

		if sourceNS != "" && destNS != "" {
		}

		if sourceNS != "" && destNS != "" && sourceNS != destNS {
			destService := flow.Destination.ServiceName
			if destService == "" {
				destService = "unknown"
			}
			destPod := flow.Destination.PodName
			if destPod == "" {
				destPod = "unknown"
			}
			m.NamespaceAccess.WithLabelValues(sourceNS, destNS, destService, destPod).Inc()
		}
	}

	if flow.L4 != nil {
		var destPort int
		if flow.L4.TCP != nil {
			destPort = int(flow.L4.TCP.DestinationPort)
		} else if flow.L4.UDP != nil {
			destPort = int(flow.L4.UDP.DestinationPort)
		}

		suspiciousPorts := map[int]bool{
			22:   false, // SSH - may be suspicious depending on context
			23:   true,  // Telnet - suspicious
			135:  true,  // RPC - suspicious
			445:  true,  // SMB - suspicious
			1433: true,  // SQL Server - suspicious
			3306: true,  // MySQL - suspicious
			5432: true,  // PostgreSQL - suspicious
		}

		if suspiciousPorts[destPort] {
			namespace := "unknown"
			if flow.Source != nil && flow.Source.Namespace != "" {
				namespace = flow.Source.Namespace
			} else if flow.Destination != nil && flow.Destination.Namespace != "" {
				namespace = flow.Destination.Namespace
			}
			m.SuspiciousOutbound.WithLabelValues(namespace, fmt.Sprintf("%d", destPort)).Inc()
		}
	}
}

func (m *PrometheusMetrics) RecordConnectionError(errorType string) {
	m.ConnectionErrors.WithLabelValues(errorType).Inc()
}

func (m *PrometheusMetrics) UpdateTrafficSpikeMultiplier(namespace string, multiplier float64) {
	m.TrafficSpikeMultiplier.WithLabelValues(namespace).Set(multiplier)
}

func (m *PrometheusMetrics) RecordNewDestination(sourceIP, destIP, namespace string) {
	m.NewDestinations.WithLabelValues(sourceIP, destIP, namespace).Inc()
}

func (m *PrometheusMetrics) RecordErrorResponse(namespace, errorType string) {
	m.ErrorResponseRate.WithLabelValues(namespace, errorType).Inc()
}

func (m *PrometheusMetrics) RecordTCPDrop(namespace, sourceIP, destIP string) {
	m.TCPDropRate.WithLabelValues(namespace, sourceIP, destIP).Inc()
}

func (m *PrometheusMetrics) UpdatePortScanDistinctPorts(sourceIP, destIP, namespace string, port uint16) {
	if m.portScanTracker == nil {
		return
	}

	m.portScanTracker.addPort(sourceIP, destIP, port)

	key := fmt.Sprintf("%s:%s", sourceIP, destIP)
	m.portScanTracker.mu.Lock()
	m.portScanTracker.metricKeys[key] = fmt.Sprintf("%s:%s:%s", sourceIP, destIP, namespace)
	m.portScanTracker.mu.Unlock()

	count := m.portScanTracker.getDistinctPortCount(sourceIP, destIP)

	m.PortScanDistinctPorts.WithLabelValues(sourceIP, destIP, namespace).Set(float64(count))
}

func (m *PrometheusMetrics) ResetPortScanMetric(sourceIP, destIP, namespace string) {
	if m.portScanTracker == nil {
		return
	}

	key := fmt.Sprintf("%s:%s", sourceIP, destIP)

	m.portScanTracker.mu.Lock()
	defer m.portScanTracker.mu.Unlock()

	// Clear all tracked ports for this pair
	if entry, exists := m.portScanTracker.entries[key]; exists {
		entry.ports = make(map[uint16]time.Time)
	}

	// Reset the Prometheus metric to 0
	m.PortScanDistinctPorts.WithLabelValues(sourceIP, destIP, namespace).Set(0)
}

// CleanupPortScanMetrics periodically cleans up old port scan metrics
func (m *PrometheusMetrics) CleanupPortScanMetrics() {
	if m.portScanTracker == nil {
		return
	}

	m.portScanTracker.mu.Lock()
	defer m.portScanTracker.mu.Unlock()

	now := time.Now()
	keysToDelete := []string{}

	for key, entry := range m.portScanTracker.entries {
		for port, timestamp := range entry.ports {
			if now.Sub(timestamp) > m.portScanTracker.window {
				delete(entry.ports, port)
			}
		}

		count := 0
		for _, timestamp := range entry.ports {
			if now.Sub(timestamp) <= m.portScanTracker.window {
				count++
			}
		}

		if metricKey, exists := m.portScanTracker.metricKeys[key]; exists {
			parts := strings.Split(metricKey, ":")
			if len(parts) == 3 {
				sourceIP, destIP, namespace := parts[0], parts[1], parts[2]
				if count == 0 {
					m.PortScanDistinctPorts.WithLabelValues(sourceIP, destIP, namespace).Set(0)
					keysToDelete = append(keysToDelete, key)
				} else {
					m.PortScanDistinctPorts.WithLabelValues(sourceIP, destIP, namespace).Set(float64(count))
				}
			}
		}

		if len(entry.ports) == 0 {
			delete(m.portScanTracker.entries, key)
		}
	}

	for _, key := range keysToDelete {
		delete(m.portScanTracker.metricKeys, key)
	}
}

func (m *PrometheusMetrics) RecordAlert(namespace, severity, alertType string) {
	if namespace == "" {
		namespace = "unknown"
	}
	if severity == "" {
		severity = "unknown"
	}
	if alertType == "" {
		alertType = "unknown"
	}
	m.AlertCounter.WithLabelValues(namespace, severity, alertType).Inc()
}

func (m *PrometheusMetrics) RecordSourceDestTraffic(namespace, sourcePod, destPod, destService string) {
	if namespace == "" {
		namespace = "unknown"
	}
	if sourcePod == "" {
		sourcePod = "unknown"
	}
	if destPod == "" {
		destPod = "unknown"
	}
	if destService == "" {
		destService = "unknown"
	}
	m.SourceDestTraffic.WithLabelValues(namespace, sourcePod, destPod, destService).Inc()
}
