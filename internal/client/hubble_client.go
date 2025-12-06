package client

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"hubble-guard/internal/model"

	"github.com/cilium/cilium/api/v1/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type HubbleGRPCClient struct {
	conn    *grpc.ClientConn
	server  string
	metrics *PrometheusMetrics
}

func NewHubbleGRPCClient(server string) (*HubbleGRPCClient, error) {
	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Hubble server: %v", err)
	}

	return &HubbleGRPCClient{
		conn:    conn,
		server:  server,
		metrics: NewPrometheusMetrics(),
	}, nil
}

func NewHubbleGRPCClientWithMetrics(server string, metrics *PrometheusMetrics) (*HubbleGRPCClient, error) {
	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Hubble server: %v", err)
	}

	return &HubbleGRPCClient{
		conn:    conn,
		server:  server,
		metrics: metrics,
	}, nil
}

func (c *HubbleGRPCClient) Close() error {
	return c.conn.Close()
}

func (c *HubbleGRPCClient) TestConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	state := c.conn.GetState()
	if state.String() == "READY" {
		fmt.Printf("Successfully connected to Hubble relay at %s\n", c.server)
		return nil
	}

	ready := c.conn.WaitForStateChange(ctx, state)
	if !ready {
		return fmt.Errorf("connection test failed: timeout waiting for connection")
	}

	finalState := c.conn.GetState()
	if finalState.String() == "READY" {
		fmt.Printf("Successfully connected to Hubble relay at %s\n", c.server)
		return nil
	}

	return fmt.Errorf("connection test failed: connection state is %s", finalState.String())
}

func (c *HubbleGRPCClient) printFlow(flowCount int, response *observer.GetFlowsResponse) {
	flow := response.GetFlow()
	if flow == nil {
		return
	}

	timeStr := flow.GetTime().AsTime().Format("2006-01-02 15:04:05.000")
	verdict := flow.GetVerdict().String()
	flowType := flow.GetType().String()

	sourceIP := "unknown"
	destIP := "unknown"
	if flow.GetIP() != nil {
		sourceIP = flow.GetIP().GetSource()
		destIP = flow.GetIP().GetDestination()
	}

	sourcePort := "unknown"
	destPort := "unknown"
	protocol := "unknown"
	bytes := uint64(0)
	if flow.GetL4() != nil {
		if tcp := flow.GetL4().GetTCP(); tcp != nil {
			protocol = "TCP"
			sourcePort = fmt.Sprintf("%d", tcp.GetSourcePort())
			destPort = fmt.Sprintf("%d", tcp.GetDestinationPort())
		} else if udp := flow.GetL4().GetUDP(); udp != nil {
			protocol = "UDP"
			sourcePort = fmt.Sprintf("%d", udp.GetSourcePort())
			destPort = fmt.Sprintf("%d", udp.GetDestinationPort())
		}
	}

	sourceInfo := fmt.Sprintf("%s:%s", sourceIP, sourcePort)
	sourceLabels := ""
	if source := flow.GetSource(); source != nil {
		if source.GetNamespace() != "" {
			sourceInfo += fmt.Sprintf(" (%s/%s)", source.GetNamespace(), source.GetPodName())
		}
		if source.GetID() != 0 {
			sourceInfo += fmt.Sprintf(" [ID:%d]", source.GetID())
		}
		if labels := source.GetLabels(); len(labels) > 0 {
			sourceLabels = strings.Join(labels, ", ")
		}
	}

	destInfo := fmt.Sprintf("%s:%s", destIP, destPort)
	destLabels := ""
	if destination := flow.GetDestination(); destination != nil {
		if destination.GetNamespace() != "" {
			destInfo += fmt.Sprintf(" (%s/%s)", destination.GetNamespace(), destination.GetPodName())
		}
		if destination.GetID() != 0 {
			destInfo += fmt.Sprintf(" [ID:%d]", destination.GetID())
		}
		if labels := destination.GetLabels(); len(labels) > 0 {
			destLabels = strings.Join(labels, ", ")
		}
	}

	tcpFlags := ""
	if flow.GetL4() != nil {
		if tcp := flow.GetL4().GetTCP(); tcp != nil && tcp.GetFlags() != nil {
			flags := []string{}
			if tcp.GetFlags().GetSYN() {
				flags = append(flags, "SYN")
			}
			if tcp.GetFlags().GetACK() {
				flags = append(flags, "ACK")
			}
			if tcp.GetFlags().GetFIN() {
				flags = append(flags, "FIN")
			}
			if tcp.GetFlags().GetRST() {
				flags = append(flags, "RST")
			}
			if tcp.GetFlags().GetPSH() {
				flags = append(flags, "PSH")
			}
			if tcp.GetFlags().GetURG() {
				flags = append(flags, "URG")
			}
			if len(flags) > 0 {
				tcpFlags = strings.Join(flags, ", ")
			}
		}
	}

	l7Info := ""
	if flow.GetL7() != nil {
		l7Type := flow.GetL7().GetType().String()
		l7Info = fmt.Sprintf("L7 Type: %s", l7Type)

		if http := flow.GetL7().GetHttp(); http != nil {
			l7Info += fmt.Sprintf(" | Method: %s | URL: %s | Status: %d",
				http.GetMethod(), http.GetUrl(), http.GetCode())
		}

		if dns := flow.GetL7().GetDns(); dns != nil {
			l7Info += fmt.Sprintf(" | Query: %s", dns.GetQuery())
		}

		if kafka := flow.GetL7().GetKafka(); kafka != nil {
			l7Info += fmt.Sprintf(" | Topic: %s | API Key: %s",
				kafka.GetTopic(), kafka.GetApiKey())
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("FLOW #%d - %s\n", flowCount, timeStr)
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	fmt.Printf(" Flow Type: %s\n", flowType)
	fmt.Printf("  Verdict: %s\n", verdict)
	fmt.Printf(" Protocol: %s\n", protocol)

	fmt.Printf(" Source: %s\n", sourceInfo)
	fmt.Printf(" Destination: %s\n", destInfo)

	if bytes > 0 {
		fmt.Printf(" Bytes: %d\n", bytes)
	}

	if tcpFlags != "" {
		fmt.Printf(" TCP Flags: %s\n", tcpFlags)
	}

	if l7Info != "" {
		fmt.Printf(" %s\n", l7Info)
	}

	if sourceLabels != "" {
		fmt.Printf(" Source Labels: %s\n", sourceLabels)
	}
	if destLabels != "" {
		fmt.Printf(" Destination Labels: %s\n", destLabels)
	}

	if flow.GetNodeName() != "" {
		fmt.Printf(" Node: %s\n", flow.GetNodeName())
	}

	if flow.GetTraceContext() != nil {
		fmt.Printf(" Trace Context: %s\n", flow.GetTraceContext())
	}

	if flow.GetEventType() != nil {
		fmt.Printf(" Event Type: %s\n", flow.GetEventType().String())
	}

	if flow.GetIsReply() != nil {
		fmt.Printf(" Is Reply: %t\n", flow.GetIsReply().GetValue())
	}

	fmt.Printf("%s\n", strings.Repeat("-", 80))
}

// Record lại các metrics cần thiết cho anomaly detection
func (c *HubbleGRPCClient) recordAnomalyDetectionMetrics(flow *model.Flow) {
	if flow == nil || c.metrics == nil {
		return
	}

	namespace := "unknown"
	if flow.Source != nil && flow.Source.Namespace != "" {
		namespace = flow.Source.Namespace
	} else if flow.Destination != nil && flow.Destination.Namespace != "" {
		namespace = flow.Destination.Namespace
	}

	if flow.Verdict == model.Verdict_DROPPED {
		if flow.IP != nil {
			c.metrics.RecordTCPDrop(namespace, flow.IP.Source, flow.IP.Destination)
		}
	}

	if flow.IP != nil {
		c.metrics.RecordNewDestination(flow.IP.Source, flow.IP.Destination, namespace)
	}

	if flow.IP != nil && flow.L4 != nil {
		var destPort uint16
		if flow.L4.TCP != nil && flow.L4.TCP.DestinationPort > 0 {
			destPort = uint16(flow.L4.TCP.DestinationPort)
		} else if flow.L4.UDP != nil && flow.L4.UDP.DestinationPort > 0 {
			destPort = uint16(flow.L4.UDP.DestinationPort)
		}
		if destPort > 0 {
			c.metrics.UpdatePortScanDistinctPorts(flow.IP.Source, flow.IP.Destination, namespace, destPort)
		}
	}

	// Record source-destination traffic for all flows
	c.recordSourceDestTraffic(flow, namespace)
}

// recordSourceDestTraffic records traffic between source and destination pods
func (c *HubbleGRPCClient) recordSourceDestTraffic(flow *model.Flow, namespace string) {
	if flow.Source == nil || flow.Destination == nil {
		return
	}

	sourcePod := flow.Source.PodName
	destPod := flow.Destination.PodName

	if sourcePod == "" || destPod == "" {
		return
	}

	destService := extractServiceName(destPod)

	c.metrics.RecordSourceDestTraffic(namespace, sourcePod, destPod, destService)
}

// extractServiceName extracts service name from pod name (e.g., demo-api-xxx -> demo-api)
func extractServiceName(podName string) string {
	if podName == "" {
		return ""
	}
	// Remove trailing hash suffixes (e.g., demo-api-5f7b8c9d4f-abc12 -> demo-api)
	parts := strings.Split(podName, "-")
	if len(parts) <= 2 {
		return podName
	}
	// Typical pod name: service-deployment-hash-hash
	// We want: service or service-name
	// Heuristic: remove last 2 parts if they look like hashes
	if len(parts) >= 3 {
		// Check if last parts look like k8s generated hashes (alphanumeric, 5+ chars)
		lastPart := parts[len(parts)-1]
		secondLastPart := parts[len(parts)-2]
		if len(lastPart) >= 5 && len(secondLastPart) >= 5 {
			// Likely a deployment pod name, remove last 2 parts
			return strings.Join(parts[:len(parts)-2], "-")
		}
	}
	return podName
}

func (c *HubbleGRPCClient) StreamFlowsWithMetricsOnly(ctx context.Context, namespaces interface{}, flowCounter func(string), flowProcessor func(*model.Flow)) error {
	fmt.Println("Starting to stream flows from Hubble relay with metrics")

	var nsList []string
	switch v := namespaces.(type) {
	case string:
		if v != "" {
			nsList = []string{v}
			fmt.Printf("Filtering flows for namespace: %s\n", v)
		}
	case []string:
		nsList = v
		if len(v) > 0 {
			fmt.Printf("Filtering flows for namespaces: %s\n", strings.Join(v, ", "))
		}
	default:
	}

	fmt.Println(strings.Repeat("=", 80))

	client := observer.NewObserverClient(c.conn)

	req := &observer.GetFlowsRequest{
		Follow: true,
	}

	if len(nsList) > 0 {
		var filters []*observer.FlowFilter
		for _, ns := range nsList {
			filters = append(filters,
				&observer.FlowFilter{
					SourceLabel: []string{"k8s:io.kubernetes.pod.namespace=" + ns},
				},
				&observer.FlowFilter{
					DestinationLabel: []string{"k8s:io.kubernetes.pod.namespace=" + ns},
				},
			)
		}
		req.Whitelist = filters
	}

	stream, err := client.GetFlows(ctx, req)
	if err != nil {
		if c.metrics != nil {
			c.metrics.RecordConnectionError("stream_start_failed")
		}
		return fmt.Errorf("failed to start flow streaming: %v", err)
	}

	flowCount := 0
	lastLogTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n Stopped streaming flows")
			return nil
		default:
			response, err := stream.Recv()
			if err == io.EOF {
				fmt.Println(" Stream ended")
				return nil
			}
			if err != nil {
				if c.metrics != nil {
					c.metrics.RecordConnectionError("stream_receive_failed")
				}
				return fmt.Errorf("failed to receive flow: %v", err)
			}

			flowCount++

			flow := c.convertHubbleFlow(response.GetFlow())
			if flow != nil {
				if c.metrics != nil {
					c.metrics.RecordFlow(flow)
					c.recordAnomalyDetectionMetrics(flow)
				}

				if flowCounter != nil {
					flowNamespace := "unknown"
					if flow.Source != nil && flow.Source.Namespace != "" {
						flowNamespace = flow.Source.Namespace
					} else if flow.Destination != nil && flow.Destination.Namespace != "" {
						flowNamespace = flow.Destination.Namespace
					}
					flowCounter(flowNamespace)
				}

				if flowProcessor != nil {
					flowProcessor(flow)
				}
			}

			if time.Since(lastLogTime) >= 10*time.Second {
				fmt.Printf(" Processed %d flows in the last 10 seconds\n", flowCount)
				lastLogTime = time.Now()
				flowCount = 0
			}
		}
	}
}

func (c *HubbleGRPCClient) StreamFlowsWithMetrics(ctx context.Context, namespace string) error {
	fmt.Println("Starting to stream flows from Hubble relay with metrics...")
	if namespace != "" {
		fmt.Printf("Filtering flows for namespace: %s\n", namespace)
	}
	fmt.Println(strings.Repeat("=", 80))

	client := observer.NewObserverClient(c.conn)

	req := &observer.GetFlowsRequest{
		Follow: true,
	}

	if namespace != "" {
		req.Whitelist = []*observer.FlowFilter{
			{
				SourceLabel: []string{"k8s:io.kubernetes.pod.namespace=" + namespace},
			},
			{
				DestinationLabel: []string{"k8s:io.kubernetes.pod.namespace=" + namespace},
			},
		}
	}

	stream, err := client.GetFlows(ctx, req)
	if err != nil {
		if c.metrics != nil {
			c.metrics.RecordConnectionError("stream_start_failed")
		}
		return fmt.Errorf("failed to start flow streaming: %v", err)
	}

	flowCount := 0

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n Stopped streaming flows")
			return nil
		default:
			response, err := stream.Recv()
			if err == io.EOF {
				fmt.Println(" Stream ended")
				return nil
			}
			if err != nil {
				if c.metrics != nil {
					c.metrics.RecordConnectionError("stream_receive_failed")
				}
				return fmt.Errorf("failed to receive flow: %v", err)
			}

			flowCount++

			flow := c.convertHubbleFlow(response.GetFlow())
			if flow != nil {
				if c.metrics != nil {
					c.metrics.RecordFlow(flow)
					c.recordAnomalyDetectionMetrics(flow)
				}
			}

			c.printFlow(flowCount, response)
		}
	}
}

func (c *HubbleGRPCClient) convertHubbleFlow(hubbleFlow *observer.Flow) *model.Flow {
	if hubbleFlow == nil {
		return nil
	}

	flow := &model.Flow{}

	if hubbleFlow.Time != nil {
		t := hubbleFlow.Time.AsTime()
		flow.Time = &t
	}

	switch hubbleFlow.Verdict {
	case observer.Verdict_FORWARDED:
		flow.Verdict = model.Verdict_FORWARDED
	case observer.Verdict_DROPPED:
		flow.Verdict = model.Verdict_DROPPED
	case observer.Verdict_ERROR:
		flow.Verdict = model.Verdict_ERROR
	case observer.Verdict_AUDIT:
		flow.Verdict = model.Verdict_AUDIT
	case observer.Verdict_REDIRECTED:
		flow.Verdict = model.Verdict_REDIRECTED
	case observer.Verdict_TRACED:
		flow.Verdict = model.Verdict_TRACED
	case observer.Verdict_TRANSLATED:
		flow.Verdict = model.Verdict_TRANSLATED
	default:
		flow.Verdict = model.Verdict_VERDICT_UNKNOWN
	}

	// Extract IP addresses
	// GetSource() and GetDestination() should return string directly
	if hubbleFlow.GetIP() != nil {
		sourceIP := hubbleFlow.GetIP().GetSource()
		destIP := hubbleFlow.GetIP().GetDestination()

		// Debug: log if IP is empty
		if sourceIP == "" || destIP == "" {
			fmt.Printf("DEBUG: IP extraction - SourceIP: '%s', DestIP: '%s'\n", sourceIP, destIP)
		}

		// Always set IP (even if empty, to help with debugging)
		flow.IP = &model.IP{
			Source:      sourceIP,
			Destination: destIP,
		}
	} else {
		fmt.Printf("DEBUG: hubbleFlow.GetIP() is nil\n")
	}

	if hubbleFlow.GetL4() != nil {
		flow.L4 = &model.L4{}

		if tcp := hubbleFlow.GetL4().GetTCP(); tcp != nil {
			flow.L4.TCP = &model.TCP{
				SourcePort:      tcp.GetSourcePort(),
				DestinationPort: tcp.GetDestinationPort(),
				Bytes:           0,
			}

			if flags := tcp.GetFlags(); flags != nil {
				flow.L4.TCP.Flags = &model.TCPFlags{
					SYN: flags.GetSYN(),
					ACK: flags.GetACK(),
					FIN: flags.GetFIN(),
					RST: flags.GetRST(),
					PSH: flags.GetPSH(),
					URG: flags.GetURG(),
				}
			}
		}

		if udp := hubbleFlow.GetL4().GetUDP(); udp != nil {
			flow.L4.UDP = &model.UDP{
				SourcePort:      udp.GetSourcePort(),
				DestinationPort: udp.GetDestinationPort(),
				Bytes:           0,
			}
		}
	}

	if hubbleFlow.GetL7() != nil {
		flow.L7 = &model.L7{}

		l7Type := hubbleFlow.GetL7().GetType()
		switch l7Type {
		case 1: // HTTP
			flow.L7.Type = model.L7Type_HTTP
		case 2: // KAFKA
			flow.L7.Type = model.L7Type_KAFKA
		case 3: // DNS
			flow.L7.Type = model.L7Type_DNS
		default:
			flow.L7.Type = model.L7Type_UNKNOWN_L7
		}
	}

	switch hubbleFlow.GetType() {
	case observer.FlowType_L3_L4:
		flow.Type = model.FlowType_L3_L4
	case observer.FlowType_L7:
		flow.Type = model.FlowType_L7
	default:
		flow.Type = model.FlowType_UNKNOWN_TYPE
	}

	if source := hubbleFlow.GetSource(); source != nil {
		labels := make(map[string]string)
		if sourceLabels := source.GetLabels(); sourceLabels != nil {
			for _, label := range sourceLabels {
				parts := strings.SplitN(label, "=", 2)
				if len(parts) == 2 {
					labels[parts[0]] = parts[1]
				}
			}
		}

		// Get pod name - try multiple sources
		podName := source.GetPodName()
		workload := ""
		serviceName := ""

		// Try to extract from labels if pod name is empty
		if podName == "" {
			if name, ok := labels["k8s:io.kubernetes.pod.name"]; ok {
				podName = name
			}
		}
		// Get app label as workload
		if app, ok := labels["k8s:app"]; ok {
			workload = app
		} else if app, ok := labels["app"]; ok {
			workload = app
		}
		// Get service name from labels
		if svc, ok := labels["k8s:io.cilium.k8s.policy.serviceaccount"]; ok {
			serviceName = svc
		}

		flow.Source = &model.Endpoint{
			Namespace:   source.GetNamespace(),
			PodName:     podName,
			ServiceName: serviceName,
			Workload:    workload,
			Labels:      labels,
		}
	}

	if dest := hubbleFlow.GetDestination(); dest != nil {
		labels := make(map[string]string)
		if destLabels := dest.GetLabels(); destLabels != nil {
			for _, label := range destLabels {
				parts := strings.SplitN(label, "=", 2)
				if len(parts) == 2 {
					labels[parts[0]] = parts[1]
				}
			}
		}

		// Get pod name - try multiple sources
		podName := dest.GetPodName()
		workload := ""
		serviceName := ""

		// Try to extract from labels if pod name is empty
		if podName == "" {
			if name, ok := labels["k8s:io.kubernetes.pod.name"]; ok {
				podName = name
			}
		}
		// Get app label as workload/service
		if app, ok := labels["k8s:app"]; ok {
			workload = app
			if serviceName == "" {
				serviceName = app
			}
		} else if app, ok := labels["app"]; ok {
			workload = app
			if serviceName == "" {
				serviceName = app
			}
		}

		flow.Destination = &model.Endpoint{
			Namespace:   dest.GetNamespace(),
			PodName:     podName,
			ServiceName: serviceName,
			Workload:    workload,
			Labels:      labels,
		}
	}

	return flow
}

func (c *HubbleGRPCClient) GetMetrics() *PrometheusMetrics {
	return c.metrics
}
