package ai

import (
	"context"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
)

// UnderstandEngineering is the existing caller boundary; the previous sheet-loop
// implementation is removed. Candidate writing is owned by the job service.
func (p *Provider) UnderstandEngineering(ctx context.Context, w bom.Workbook, trace func(string, string, string) error, options ...EngineeringToolHooks) (engineering.EngineeringProposal, error) {
	var hooks EngineeringToolHooks
	if len(options) > 0 {
		hooks = options[0]
	}
	result, err := p.ExtractKickoff(ctx, w, w, trace, hooks)
	return engineering.EngineeringProposal{Summary: "分步固定格式候选", Questions: result.Missing, Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}}, err
}
