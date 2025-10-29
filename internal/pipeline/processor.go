package pipeline

import (
	"context"

	"hubble-anomaly-detector/internal/model"
	"hubble-anomaly-detector/internal/rules"
)

// Processor receives flow, normalizes it, evaluates rules, and emits alerts
type Processor struct {
	engine *rules.Engine
}

// NewProcessor creates a new processor instance
func NewProcessor(engine *rules.Engine) *Processor {
	return &Processor{
		engine: engine,
	}
}

// Process receives a flow, normalizes it, evaluates rules and emits alerts
func (p *Processor) Process(ctx context.Context, flow *model.Flow) error {
	if flow == nil {
		return nil
	}

	// Normalize flow (if needed)
	normalizedFlow := p.normalize(flow)

	// Evaluate rules - they will emit alerts themselves
	_ = p.engine.Evaluate(ctx, normalizedFlow)

	return nil
}

// normalize normalizes flow data
func (p *Processor) normalize(flow *model.Flow) *model.Flow {
	// For now, just return the flow as-is
	// Can add normalization logic here if needed
	return flow
}
