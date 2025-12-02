package model

import "time"

type Rule struct {
	Name        string                 `yaml:"name" json:"name"`
	Enabled     bool                   `yaml:"enabled" json:"enabled"`
	Severity    string                 `yaml:"severity" json:"severity"`
	Description string                 `yaml:"description" json:"description"`
	Type        string                 `yaml:"type" json:"type"`
	Thresholds  map[string]interface{} `yaml:"thresholds,omitempty" json:"thresholds,omitempty"`
	Duration    time.Duration          `yaml:"duration,omitempty" json:"duration,omitempty"`
	Conditions  []Condition            `yaml:"conditions,omitempty" json:"conditions,omitempty"`
}

type Condition struct {
	Field    string      `yaml:"field" json:"field"`
	Operator string      `yaml:"operator" json:"operator"`
	Value    interface{} `yaml:"value" json:"value"`
}

type RuleConfig struct {
	Enabled             bool     `json:"enabled"`
	Severity            string   `json:"severity"`
	Description         string   `json:"description"`
	ThresholdMultiplier *float64 `json:"threshold_multiplier,omitempty"`
	ThresholdPerMinute  *int     `json:"threshold_per_minute,omitempty"`
}

type Alert struct {
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Namespace string    `json:"namespace"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	FlowData  *Flow     `json:"flow_data,omitempty"`
}

type FlowStats struct {
	TotalFlows       int64
	TotalBytes       int64
	TotalConnections int64
	DroppedPackets   int64
	LastReset        time.Time
	FlowRate         float64
	ByteRate         float64
	ConnectionRate   float64
	DropRate         float64
}
