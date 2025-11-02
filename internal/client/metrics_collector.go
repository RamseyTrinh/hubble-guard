package client

import (
	"fmt"
	"sync"
	"time"

	"hubble-anomaly-detector/internal/model"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics chứa tất cả các metrics được expose
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

	// Port scan tracking
	portScanTracker *portScanTracker
}

// portScanEntry tracks ports seen for a source-dest pair
type portScanEntry struct {
	ports map[uint16]time.Time
}

// portScanTracker tracks distinct ports in 10s windows per source-dest pair
type portScanTracker struct {
	entries map[string]*portScanEntry
	mu      sync.RWMutex
	window  time.Duration
}

// newPortScanTracker creates a new port scan tracker
func newPortScanTracker() *portScanTracker {
	return &portScanTracker{
		entries: make(map[string]*portScanEntry),
		window:  10 * time.Second,
	}
}

// addPort adds a port to the tracker for a source-dest pair
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

	// Add or update port timestamp
	entry.ports[port] = now

	// Cleanup old ports (older than window)
	pst.cleanupOldPorts(key, entry, now)
}

// cleanupOldPorts removes ports older than the window
func (pst *portScanTracker) cleanupOldPorts(key string, entry *portScanEntry, now time.Time) {
	for port, timestamp := range entry.ports {
		if now.Sub(timestamp) > pst.window {
			delete(entry.ports, port)
		}
	}

	// Remove entry if no ports left
	if len(entry.ports) == 0 {
		delete(pst.entries, key)
	}
}

// getDistinctPortCount returns the count of distinct ports for a source-dest pair
func (pst *portScanTracker) getDistinctPortCount(sourceIP, destIP string) int {
	key := fmt.Sprintf("%s:%s", sourceIP, destIP)
	now := time.Now()

	pst.mu.RLock()
	defer pst.mu.RUnlock()

	entry, exists := pst.entries[key]
	if !exists {
		return 0
	}

	// Count valid ports (within window)
	count := 0
	for port, timestamp := range entry.ports {
		if now.Sub(timestamp) <= pst.window {
			count++
		} else {
			// Mark for cleanup (we'll do it in next addPort call)
			delete(entry.ports, port)
		}
	}

	return count
}

// NewPrometheusMetrics tạo instance mới của PrometheusMetrics
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		// Flow metrics
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

		// TCP metrics
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

		// UDP metrics
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

		// L7 metrics
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

		// Error metrics
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

		// Performance metrics
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

		// Anomaly detection metrics
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
			[]string{"source_ip", "dest_ip"},
		),
		portScanTracker: newPortScanTracker(),
	}
}

// RecordFlow ghi lại metrics cho một flow
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
}

// RecordError ghi lại lỗi
func (m *PrometheusMetrics) RecordError(errorType, namespace string) {
	m.FlowErrors.WithLabelValues(errorType, namespace).Inc()
}

// RecordConnectionError ghi lại lỗi kết nối
func (m *PrometheusMetrics) RecordConnectionError(errorType string) {
	m.ConnectionErrors.WithLabelValues(errorType).Inc()
}

// RecordProcessingTime ghi lại thời gian xử lý
func (m *PrometheusMetrics) RecordProcessingTime(namespace string, duration float64) {
	m.FlowProcessingTime.WithLabelValues(namespace).Observe(duration)
}

// UpdateActiveConnections cập nhật số lượng kết nối đang hoạt động
func (m *PrometheusMetrics) UpdateActiveConnections(namespace, protocol string, count float64) {
	m.ActiveConnections.WithLabelValues(namespace, protocol).Set(count)
}

// UpdateBaselineTrafficRate cập nhật baseline traffic rate
func (m *PrometheusMetrics) UpdateBaselineTrafficRate(namespace string, rate float64) {
	m.BaselineTrafficRate.WithLabelValues(namespace).Set(rate)
}

// UpdateTrafficSpikeMultiplier cập nhật traffic spike multiplier
func (m *PrometheusMetrics) UpdateTrafficSpikeMultiplier(namespace string, multiplier float64) {
	m.TrafficSpikeMultiplier.WithLabelValues(namespace).Set(multiplier)
}

// RecordNewDestination ghi lại kết nối đến destination mới
func (m *PrometheusMetrics) RecordNewDestination(sourceIP, destIP, namespace string) {
	m.NewDestinations.WithLabelValues(sourceIP, destIP, namespace).Inc()
}

// RecordErrorResponse ghi lại error response
func (m *PrometheusMetrics) RecordErrorResponse(namespace, errorType string) {
	m.ErrorResponseRate.WithLabelValues(namespace, errorType).Inc()
}

// RecordTCPReset ghi lại TCP reset
func (m *PrometheusMetrics) RecordTCPReset(namespace, sourceIP, destIP string) {
	m.TCPResetRate.WithLabelValues(namespace, sourceIP, destIP).Inc()
}

// RecordTCPDrop ghi lại TCP drop
func (m *PrometheusMetrics) RecordTCPDrop(namespace, sourceIP, destIP string) {
	m.TCPDropRate.WithLabelValues(namespace, sourceIP, destIP).Inc()
}

// UpdatePortScanDistinctPorts updates the distinct ports count for port scan detection
func (m *PrometheusMetrics) UpdatePortScanDistinctPorts(sourceIP, destIP string, port uint16) {
	if m.portScanTracker == nil {
		return
	}

	// Add port to tracker
	m.portScanTracker.addPort(sourceIP, destIP, port)

	// Get current distinct port count
	count := m.portScanTracker.getDistinctPortCount(sourceIP, destIP)

	// Update metric
	m.PortScanDistinctPorts.WithLabelValues(sourceIP, destIP).Set(float64(count))
}
