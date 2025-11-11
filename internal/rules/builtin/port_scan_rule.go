package builtin

import (
	"context"
	"fmt"
	"time"

	"hubble-anomaly-detector/internal/model"

	prommodel "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type PortScanRule struct {
	name          string
	enabled       bool
	severity      string
	threshold     float64
	prometheusAPI PrometheusQueryClient
	logger        *logrus.Logger
	interval      time.Duration
	stopChan      chan struct{}
	alertEmitter  func(*model.Alert)
}

func NewPortScanRule(enabled bool, severity string, threshold float64, promClient PrometheusQueryClient, logger *logrus.Logger) *PortScanRule {
	if threshold <= 0 {
		threshold = 10.0
	}
	return &PortScanRule{
		name:          "port_scan",
		enabled:       enabled,
		severity:      severity,
		threshold:     threshold,
		prometheusAPI: promClient,
		logger:        logger,
		interval:      10 * time.Second,
		stopChan:      make(chan struct{}),
	}
}

func (r *PortScanRule) SetAlertEmitter(emitter func(*model.Alert)) {
	r.alertEmitter = emitter
}

func (r *PortScanRule) Name() string {
	return r.name
}

func (r *PortScanRule) IsEnabled() bool {
	return r.enabled
}

func (r *PortScanRule) Start(ctx context.Context) {
	if !r.enabled {
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.Infof("[Port Scan] Starting periodic checks from Prometheus (interval: %v, threshold: %.0f)", r.interval, r.threshold)

	for {
		select {
		case <-ticker.C:
			r.checkFromPrometheus(ctx)
		case <-ctx.Done():
			r.logger.Info("[Port Scan] Stopping periodic checks")
			return
		case <-r.stopChan:
			r.logger.Info("[Port Scan] Rule stopped")
			return
		}
	}
}

func (r *PortScanRule) Stop() {
	close(r.stopChan)
}

func (r *PortScanRule) Evaluate(ctx context.Context, flow *model.Flow) *model.Alert {
	return nil
}

func (r *PortScanRule) checkFromPrometheus(ctx context.Context) {
	query := `portscan_distinct_ports_10s > 0`

	result, err := r.prometheusAPI.Query(ctx, query, 10*time.Second)
	if err != nil {
		r.logger.Errorf("[Port Scan] Failed to query Prometheus: %v", err)
		return
	}

	vector, ok := result.(prommodel.Vector)
	if !ok {
		r.logger.Debugf("[Port Scan] No data from Prometheus")
		return
	}

	for _, sample := range vector {
		sourceIP := ""
		destIP := ""

		if val, exists := sample.Metric["source_ip"]; exists {
			sourceIP = string(val)
		}
		if val, exists := sample.Metric["dest_ip"]; exists {
			destIP = string(val)
		}

		distinctPorts := float64(sample.Value)

		r.logger.Debugf("[Port Scan] source_ip: %s, dest_ip: %s, distinct_ports: %.0f (threshold: %.0f)",
			sourceIP, destIP, distinctPorts, r.threshold)

		if distinctPorts > r.threshold {
			alert := &model.Alert{
				Type:      r.name,
				Severity:  r.severity,
				Message:   fmt.Sprintf("Port scanning detected: %.0f distinct ports in 10 seconds from %s to %s (threshold: %.0f)", distinctPorts, sourceIP, destIP, r.threshold),
				Timestamp: time.Now(),
			}
			r.logger.Warnf("Port Scan Rule Alert: %s", alert.Message)
			if r.alertEmitter != nil {
				r.alertEmitter(alert)
			}
		}
	}
}
