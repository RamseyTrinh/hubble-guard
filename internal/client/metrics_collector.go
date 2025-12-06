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
	FlowTotal         *prometheus.CounterVec
	FlowByVerdict     *prometheus.CounterVec
	FlowByProtocol    *prometheus.CounterVec
	FlowByNamespace   *prometheus.CounterVec
	FlowBySource      *prometheus.CounterVec
	FlowByDestination *prometheus.CounterVec

	// TCP metrics
	TCPConnections *prometheus.CounterVec
	TCPFlags       *prometheus.CounterVec
	TCPBytes       *prometheus.CounterVec

	// UDP metrics
	UDPPackets *prometheus.CounterVec
	UDPBytes   *prometheus.CounterVec

	// L7 metrics
	L7Requests *prometheus.CounterVec
	L7ByType   *prometheus.CounterVec

	// Error metrics
	FlowErrors       *prometheus.CounterVec
	ConnectionErrors *prometheus.CounterVec

	// Performance metrics
	FlowProcessingTime *prometheus.HistogramVec
	ActiveConnections  *prometheus.GaugeVec

	// Anomaly detection metrics
	BaselineTrafficRate    *prometheus.GaugeVec
	TrafficSpikeMultiplier *prometheus.GaugeVec
	NewDestinations        *prometheus.CounterVec
	ErrorResponseRate      *prometheus.CounterVec
	TCPResetRate           *prometheus.CounterVec
	TCPDropRate            *prometheus.CounterVec
	PortScanDistinctPorts  *prometheus.GaugeVec
	NamespaceAccess        *prometheus.CounterVec // Cross-namespace access tracking
	SuspiciousOutbound     *prometheus.CounterVec // Suspicious outbound connections tracking
	// Source-Destination traffic tracking (for unusual traffic detection)
	SourceDestTraffic *prometheus.CounterVec

	// Alert metrics
	AlertCounter *prometheus.CounterVec // Total alerts detected

	// Port scan tracking
	portScanTracker *portScanTracker
}

type portScanEntry struct {
	ports map[uint16]time.Time
}

type portScanTracker struct {
	entries map[string]*portScanEntry
	mu      sync.RWMutex
	window  time.Duration
	// Track which metrics need to be updated
	metricKeys map[string]string // key -> "sourceIP:destIP:namespace"
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

	// Always update timestamp for this port (even if it already exists)
	// This ensures the port stays in the window if it's accessed again
	entry.ports[port] = now

	// Don't cleanup here - let getDistinctPortCount handle cleanup
	// This ensures all ports added in quick succession are counted correctly
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

	// Count ports within window and cleanup old ones
	count := 0
	portsToDelete := []uint16{}
	for port, timestamp := range entry.ports {
		if now.Sub(timestamp) <= pst.window {
			count++
		} else {
			portsToDelete = append(portsToDelete, port)
		}
	}

	// Cleanup old ports
	for _, port := range portsToDelete {
		delete(entry.ports, port)
	}

	// Delete entry if no ports left
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

		FlowBySource: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_flows_by_source_total",
				Help: "Total number of flows by source",
			},
			[]string{"source_ip", "source_port", "namespace"},
		),

		FlowByDestination: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_flows_by_destination_total",
				Help: "Total number of flows by destination",
			},
			[]string{"destination_ip", "destination_port", "namespace"},
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

		UDPPackets: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_udp_packets_total",
				Help: "Total number of UDP packets",
			},
			[]string{"namespace", "source_ip", "destination_ip"},
		),

		UDPBytes: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_udp_bytes_total",
				Help: "Total number of UDP bytes",
			},
			[]string{"namespace", "direction"},
		),

		L7Requests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_l7_requests_total",
				Help: "Total number of L7 requests",
			},
			[]string{"type", "namespace"},
		),

		L7ByType: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_l7_by_type_total",
				Help: "Total number of L7 requests by type",
			},
			[]string{"l7_type", "namespace"},
		),

		FlowErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_flow_errors_total",
				Help: "Total number of flow errors",
			},
			[]string{"error_type", "namespace"},
		),

		ConnectionErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_connection_errors_total",
				Help: "Total number of connection errors",
			},
			[]string{"error_type"},
		),

		FlowProcessingTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hubble_flow_processing_duration_seconds",
				Help:    "Time spent processing flows",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"namespace"},
		),

		ActiveConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hubble_active_connections",
				Help: "Number of active connections",
			},
			[]string{"namespace", "protocol"},
		),

		BaselineTrafficRate: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hubble_baseline_traffic_rate",
				Help: "Baseline traffic rate for anomaly detection",
			},
			[]string{"namespace"},
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

		TCPResetRate: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hubble_tcp_resets_total",
				Help: "Total number of TCP resets",
			},
			[]string{"namespace", "source_ip", "destination_ip"},
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
				Name: "hubble_namespace_access_total",
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

// Record metrics cho một flow
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

	// Flow total
	m.FlowTotal.WithLabelValues(namespace).Inc()

	// Flow by verdict
	verdict := flow.Verdict.String()
	m.FlowByVerdict.WithLabelValues(verdict, namespace).Inc()

	// Flow by namespace
	m.FlowByNamespace.WithLabelValues(namespace).Inc()

	// Protocol metrics
	if flow.L4 != nil {
		if flow.L4.TCP != nil {
			protocol := "tcp"
			m.FlowByProtocol.WithLabelValues(protocol, namespace).Inc()

			// TCP specific metrics
			if flow.IP != nil {
				sourceIP := flow.IP.Source
				destIP := flow.IP.Destination
				m.TCPConnections.WithLabelValues(namespace, sourceIP, destIP).Inc()

				// TCP flags
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

				// TCP bytes
				if flow.L4.TCP.Bytes > 0 {
					m.TCPBytes.WithLabelValues(namespace, "outbound").Add(float64(flow.L4.TCP.Bytes))
				}
			}
		} else if flow.L4.UDP != nil {
			protocol := "udp"
			m.FlowByProtocol.WithLabelValues(protocol, namespace).Inc()

			// UDP specific metrics
			if flow.IP != nil {
				sourceIP := flow.IP.Source
				destIP := flow.IP.Destination
				m.UDPPackets.WithLabelValues(namespace, sourceIP, destIP).Inc()

				// UDP bytes
				if flow.L4.UDP.Bytes > 0 {
					m.UDPBytes.WithLabelValues(namespace, "outbound").Add(float64(flow.L4.UDP.Bytes))
				}
			}
		}
	}

	// L7 metrics
	if flow.L7 != nil {
		l7Type := flow.L7.Type.String()
		m.L7Requests.WithLabelValues(l7Type, namespace).Inc()
		m.L7ByType.WithLabelValues(l7Type, namespace).Inc()
	}

	// Source and destination metrics
	if flow.IP != nil {
		sourceIP := flow.IP.Source
		destIP := flow.IP.Destination

		sourcePort := "unknown"
		destPort := "unknown"

		if flow.L4 != nil {
			if flow.L4.TCP != nil {
				sourcePort = fmt.Sprintf("%d", flow.L4.TCP.SourcePort)
				destPort = fmt.Sprintf("%d", flow.L4.TCP.DestinationPort)
			} else if flow.L4.UDP != nil {
				sourcePort = fmt.Sprintf("%d", flow.L4.UDP.SourcePort)
				destPort = fmt.Sprintf("%d", flow.L4.UDP.DestinationPort)
			}
		}

		m.FlowBySource.WithLabelValues(sourceIP, sourcePort, namespace).Inc()
		m.FlowByDestination.WithLabelValues(destIP, destPort, namespace).Inc()
	}

	// Error metrics
	if flow.Verdict == model.Verdict_ERROR {
		m.FlowErrors.WithLabelValues("verdict_error", namespace).Inc()
	}

	// Namespace access tracking - track cross-namespace access
	if flow.Source != nil && flow.Destination != nil {
		sourceNS := flow.Source.Namespace
		destNS := flow.Destination.Namespace
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

	// Suspicious outbound tracking - track connections to suspicious ports
	if flow.L4 != nil {
		var destPort int
		if flow.L4.TCP != nil {
			destPort = int(flow.L4.TCP.DestinationPort)
		} else if flow.L4.UDP != nil {
			destPort = int(flow.L4.UDP.DestinationPort)
		}

		// Check if port is suspicious
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

func (m *PrometheusMetrics) RecordError(errorType, namespace string) {
	m.FlowErrors.WithLabelValues(errorType, namespace).Inc()
}

func (m *PrometheusMetrics) RecordConnectionError(errorType string) {
	m.ConnectionErrors.WithLabelValues(errorType).Inc()
}

func (m *PrometheusMetrics) RecordProcessingTime(namespace string, duration float64) {
	m.FlowProcessingTime.WithLabelValues(namespace).Observe(duration)
}

func (m *PrometheusMetrics) UpdateActiveConnections(namespace, protocol string, count float64) {
	m.ActiveConnections.WithLabelValues(namespace, protocol).Set(count)
}

func (m *PrometheusMetrics) UpdateBaselineTrafficRate(namespace string, rate float64) {
	m.BaselineTrafficRate.WithLabelValues(namespace).Set(rate)
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

func (m *PrometheusMetrics) RecordTCPReset(namespace, sourceIP, destIP string) {
	m.TCPResetRate.WithLabelValues(namespace, sourceIP, destIP).Inc()
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

// RecordSourceDestTraffic records traffic between source pod and destination pod/service
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
