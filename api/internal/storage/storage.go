package storage

import (
	"fmt"
	"sync"
	"time"

	"hubble-anomaly-detector/internal/model"

	"github.com/sirupsen/logrus"
)

type Storage struct {
	mu          sync.RWMutex
	alerts      []Alert
	flows       []Flow
	rules       []Rule
	maxAlerts   int
	maxFlows    int
	logger      *logrus.Logger
	alertSubs   map[*AlertSubscriber]bool
	alertSubsMu sync.RWMutex
	flowSubs    map[*FlowSubscriber]bool
	flowSubsMu  sync.RWMutex
}

type Alert struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Severity  string                 `json:"severity"`
	Namespace string                 `json:"namespace"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	FlowData  *model.Flow            `json:"flow_data,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type Flow struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	Source           *Endpoint `json:"source,omitempty"`
	Destination      *Endpoint `json:"destination,omitempty"`
	Verdict          string    `json:"verdict"`
	Namespace        string    `json:"namespace"`
	Port             uint32    `json:"port,omitempty"`
	SourceIP         string    `json:"source_ip,omitempty"`
	DestinationIP    string    `json:"destination_ip,omitempty"`
	DestinationPort  uint32    `json:"destination_port,omitempty"`
	TrafficDirection string    `json:"traffic_direction,omitempty"` // egress, ingress
	TCPFlags         string    `json:"tcp_flags,omitempty"`
}

type Endpoint struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	IP        string `json:"ip,omitempty"`
	Port      uint32 `json:"port,omitempty"`
	Identity  string `json:"identity,omitempty"` // namespace/pod_name format
}

type Rule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Enabled     bool                   `json:"enabled"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	Thresholds  map[string]interface{} `json:"thresholds,omitempty"`
}

type AlertSubscriber struct {
	ID       string
	Channel  chan Alert
	Filter   AlertFilter
	LastSeen time.Time
}

type AlertFilter struct {
	Severity  string
	Namespace string
	Type      string
}

type FlowSubscriber struct {
	ID       string
	Channel  chan Flow
	LastSeen time.Time
}

func NewStorage(logger *logrus.Logger) *Storage {
	return &Storage{
		alerts:    make([]Alert, 0),
		flows:     make([]Flow, 0),
		rules:     make([]Rule, 0),
		maxAlerts: 10000, // Keep last 10k alerts
		maxFlows:  50000, // Keep last 50k flows
		logger:    logger,
		alertSubs: make(map[*AlertSubscriber]bool),
		flowSubs:  make(map[*FlowSubscriber]bool),
	}
}

// Alert methods
func (s *Storage) AddAlert(alert Alert) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alert.ID = generateID()
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	s.alerts = append(s.alerts, alert)

	// Keep only last maxAlerts
	if len(s.alerts) > s.maxAlerts {
		s.alerts = s.alerts[len(s.alerts)-s.maxAlerts:]
	}

	// Notify subscribers
	s.notifySubscribers(alert)
}

func (s *Storage) GetAlerts(limit int, severity, namespace, search string) []Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Alert, 0)
	count := 0

	// Iterate in reverse to get latest first
	for i := len(s.alerts) - 1; i >= 0 && count < limit; i-- {
		alert := s.alerts[i]

		// Apply filters
		if severity != "" && alert.Severity != severity {
			continue
		}
		if namespace != "" && alert.Namespace != namespace {
			continue
		}
		if search != "" && !contains(alert.Message, search) {
			continue
		}

		result = append(result, alert)
		count++
	}

	return result
}

func (s *Storage) GetAlertByID(id string) *Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.alerts {
		if s.alerts[i].ID == id {
			return &s.alerts[i]
		}
	}
	return nil
}

func (s *Storage) GetAlertsTimeline(start, end time.Time) []Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Alert, 0)
	for i := range s.alerts {
		alert := s.alerts[i]
		if (start.IsZero() || alert.Timestamp.After(start) || alert.Timestamp.Equal(start)) &&
			(end.IsZero() || alert.Timestamp.Before(end) || alert.Timestamp.Equal(end)) {
			result = append(result, alert)
		}
	}
	return result
}

// Flow methods
func (s *Storage) AddFlow(flow Flow) {
	s.mu.Lock()

	flow.ID = generateID()
	if flow.Timestamp.IsZero() {
		flow.Timestamp = time.Now()
	}

	s.flows = append(s.flows, flow)

	// Keep only last maxFlows
	if len(s.flows) > s.maxFlows {
		s.flows = s.flows[len(s.flows)-s.maxFlows:]
	}

	s.mu.Unlock()

	// Notify subscribers
	s.notifyFlowSubscribers(flow)
}

func (s *Storage) SubscribeFlows(sub *FlowSubscriber) {
	s.flowSubsMu.Lock()
	defer s.flowSubsMu.Unlock()
	s.flowSubs[sub] = true
}

func (s *Storage) UnsubscribeFlows(sub *FlowSubscriber) {
	s.flowSubsMu.Lock()
	defer s.flowSubsMu.Unlock()
	delete(s.flowSubs, sub)
	close(sub.Channel)
}

func (s *Storage) notifyFlowSubscribers(flow Flow) {
	s.flowSubsMu.RLock()
	defer s.flowSubsMu.RUnlock()

	for sub := range s.flowSubs {
		select {
		case sub.Channel <- flow:
			sub.LastSeen = time.Now()
		default:
			// Channel full, skip
		}
	}
}

func (s *Storage) GetFlows(page, limit int, namespace, verdict, search string) ([]Flow, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter flows
	filtered := make([]Flow, 0)
	for i := range s.flows {
		flow := s.flows[i]

		if namespace != "" && flow.Namespace != namespace {
			continue
		}
		if verdict != "" && flow.Verdict != verdict {
			continue
		}
		if search != "" {
			matched := false
			if flow.Source != nil && contains(flow.Source.Name, search) {
				matched = true
			}
			if flow.Destination != nil && contains(flow.Destination.Name, search) {
				matched = true
			}
			if !matched {
				continue
			}
		}

		filtered = append(filtered, flow)
	}

	total := len(filtered)

	// Pagination
	start := (page - 1) * limit
	end := start + limit
	if start > len(filtered) {
		return []Flow{}, total
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	// Return in reverse order (latest first)
	result := make([]Flow, 0)
	for i := len(filtered) - 1 - start; i >= 0 && len(result) < limit; i-- {
		if i < len(filtered) {
			result = append(result, filtered[i])
		}
	}

	return result, total
}

func (s *Storage) GetFlowByID(id string) *Flow {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.flows {
		if s.flows[i].ID == id {
			return &s.flows[i]
		}
	}
	return nil
}

func (s *Storage) GetFlowStats() FlowStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := FlowStats{
		TotalFlows: int64(len(s.flows)),
	}

	// Count by verdict
	verdictCounts := make(map[string]int64)
	for i := range s.flows {
		verdictCounts[s.flows[i].Verdict]++
	}
	stats.VerdictCounts = verdictCounts

	// Count by namespace
	namespaceCounts := make(map[string]int64)
	for i := range s.flows {
		namespaceCounts[s.flows[i].Namespace]++
	}
	stats.NamespaceCounts = namespaceCounts

	return stats
}

type FlowStats struct {
	TotalFlows      int64            `json:"total_flows"`
	VerdictCounts   map[string]int64 `json:"verdict_counts"`
	NamespaceCounts map[string]int64 `json:"namespace_counts"`
}

// Rule methods
func (s *Storage) SetRules(rules []Rule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = rules
}

func (s *Storage) GetRules() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Rule, len(s.rules))
	copy(result, s.rules)
	return result
}

func (s *Storage) GetRuleByID(id string) *Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.rules {
		if s.rules[i].ID == id || s.rules[i].Name == id {
			return &s.rules[i]
		}
	}
	return nil
}

func (s *Storage) UpdateRule(id string, updates Rule) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.rules {
		if s.rules[i].ID == id || s.rules[i].Name == id {
			if updates.Enabled != s.rules[i].Enabled {
				s.rules[i].Enabled = updates.Enabled
			}
			if updates.Severity != "" {
				s.rules[i].Severity = updates.Severity
			}
			if updates.Description != "" {
				s.rules[i].Description = updates.Description
			}
			return true
		}
	}
	return false
}

func (s *Storage) GetRulesStats() RulesStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := RulesStats{
		Total:    len(s.rules),
		Enabled:  0,
		Disabled: 0,
	}

	for i := range s.rules {
		if s.rules[i].Enabled {
			stats.Enabled++
		} else {
			stats.Disabled++
		}
	}

	return stats
}

type RulesStats struct {
	Total    int `json:"total"`
	Enabled  int `json:"enabled"`
	Disabled int `json:"disabled"`
}

// Subscriber methods
func (s *Storage) SubscribeAlerts(sub *AlertSubscriber) {
	s.alertSubsMu.Lock()
	defer s.alertSubsMu.Unlock()
	s.alertSubs[sub] = true
}

func (s *Storage) UnsubscribeAlerts(sub *AlertSubscriber) {
	s.alertSubsMu.Lock()
	defer s.alertSubsMu.Unlock()
	delete(s.alertSubs, sub)
	close(sub.Channel)
}

func (s *Storage) notifySubscribers(alert Alert) {
	s.alertSubsMu.RLock()
	defer s.alertSubsMu.RUnlock()

	for sub := range s.alertSubs {
		// Apply filter
		if sub.Filter.Severity != "" && alert.Severity != sub.Filter.Severity {
			continue
		}
		if sub.Filter.Namespace != "" && alert.Namespace != sub.Filter.Namespace {
			continue
		}
		if sub.Filter.Type != "" && alert.Type != sub.Filter.Type {
			continue
		}

		select {
		case sub.Channel <- alert:
			sub.LastSeen = time.Now()
		default:
			// Channel full, skip
		}
	}
}

// Helper functions
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			indexOf(s, substr) >= 0)))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
