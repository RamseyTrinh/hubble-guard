package rules

import (
	"encoding/json"
	"fmt"
	"os"

	"hubble-anomaly-detector/internal/model"

	"gopkg.in/yaml.v3"
)

// LoadRulesFromJSON loads rules from a JSON configuration file
func LoadRulesFromJSON(filename string) ([]model.Rule, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %v", err)
	}

	var rules struct {
		Rules []model.Rule `json:"rules"`
	}

	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse rules file: %v", err)
	}

	return rules.Rules, nil
}

// LoadRulesFromYAML loads rules from a YAML configuration file
func LoadRulesFromYAML(filename string) ([]model.Rule, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %v", err)
	}

	var rules struct {
		Rules []model.Rule `yaml:"rules"`
	}

	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse YAML rules file: %v", err)
	}

	return rules.Rules, nil
}

// LoadRules automatically detects file format and loads rules
func LoadRules(filename string) ([]model.Rule, error) {
	if len(filename) == 0 {
		return nil, fmt.Errorf("rules file path is empty")
	}

	// Try YAML first, then JSON
	if len(filename) >= 5 && filename[len(filename)-5:] == ".yaml" || len(filename) >= 4 && filename[len(filename)-4:] == ".yml" {
		return LoadRulesFromYAML(filename)
	}

	if len(filename) >= 5 && filename[len(filename)-5:] == ".json" {
		return LoadRulesFromJSON(filename)
	}

	// Default: try YAML first, fallback to JSON
	if rules, err := LoadRulesFromYAML(filename); err == nil {
		return rules, nil
	}
	return LoadRulesFromJSON(filename)
}
