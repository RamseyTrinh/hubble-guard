package pipeline

import (
	"context"

	"hubble-guard/internal/model"
	"hubble-guard/internal/rules"
)

type Processor struct {
	engine *rules.Engine
}

func NewProcessor(engine *rules.Engine) *Processor {
	return &Processor{
		engine: engine,
	}
}

func (p *Processor) Process(ctx context.Context, flow *model.Flow) error {
	if flow == nil {
		return nil
	}

	normalizedFlow := p.normalize(flow)

	_ = p.engine.Evaluate(ctx, normalizedFlow)

	return nil
}

func (p *Processor) normalize(flow *model.Flow) *model.Flow {
	return flow
}
