package engineering

import "strings"

func RegionModule(kind string) string {
	switch kind {
	case "equipment", "assembly":
		return "equipment"
	case "site", "room", "rack", "placement":
		return "facility_placement"
	case "port", "connection", "cable":
		return "connectivity_cabling"
	case "power":
		return "power_cooling"
	case "material", "workload":
		return "material_workload"
	}
	if strings.HasPrefix(kind, "network_") {
		return "network_design"
	}
	return "workbook_intelligence"
}

// BuildImportPlan selects registered capabilities from evidenced regions and typed
// outputs. It never infers network intent from equipment or physical connections.
func BuildImportPlan(project, source, revision string, sequence int, proposal EngineeringProposal, model PhysicalModel) ImportPlan {
	r := DefaultRegistry()
	plan := ImportPlan{ID: StableID(source, "import-plan", revision), ProjectID: project, SourceID: source, Version: sequence, Summary: proposal.Summary, Modules: []ModuleInstance{}}
	counts := map[string]int{"equipment": len(model.Equipment) + len(model.Assemblies), "facility_placement": len(model.Facilities) + len(model.Placements), "connectivity_cabling": len(model.Ports) + len(model.Connections) + len(model.Cables) + len(model.Optics), "power_cooling": len(model.PowerPlans), "material_workload": len(model.Materials) + len(model.Workloads)}
	for _, p := range model.NetworkPlans {
		if NetworkHasContent(p) {
			counts["network_design"]++
		}
	}
	regions := map[string]int{}
	for _, region := range proposal.Regions {
		regions[RegionModule(region.Kind)]++
	}
	detections := map[string]ModuleDetection{}
	for _, d := range proposal.Modules {
		detections[d.ModuleID] = d
	}
	outputs := map[string]bool{"source": true, "facts": len(proposal.Regions) > 0, "plan": true, "equipment": len(model.Equipment) > 0, "assemblies": len(model.Assemblies) > 0, "facility": len(model.Facilities) > 0, "placement": len(model.Placements) > 0, "connectivity": len(model.Ports) > 0 || len(model.Connections) > 0, "cabling": len(model.Cables) > 0, "power": len(model.PowerPlans) > 0, "material": len(model.Materials) > 0, "workload": len(model.Workloads) > 0, "network": counts["network_design"] > 0}
	for _, d := range r.Definitions() {
		m := ModuleInstance{ID: StableID(source, "module", d.ID), DefinitionID: d.ID, DefinitionVersion: d.Version, State: Detected, ApplicabilityReason: "尚未确认是否适用", HeadRevisionID: &revision, Inputs: []InputBinding{}}
		detection, detected := detections[d.ID]
		if detected {
			m.ApplicabilityReason = detection.Reason
			if !detection.Applicable {
				m.State = NotApplicable
			} else {
				m.State = NeedsInput
			}
		}
		if regions[d.ID] > 0 || counts[d.ID] > 0 {
			m.State = NeedsInput
			if !detected || !detection.Applicable {
				m.ApplicabilityReason = "已有来源区域或人工工程模型"
			}
		}
		if counts[d.ID] > 0 {
			m.State = InReview
		}
		if d.ID == "workbook_intelligence" {
			m.State = Ready
			m.ApplicabilityReason = "已保存不可变原始资料"
			if len(proposal.Regions) > 0 {
				m.State = InReview
			}
		}
		if d.ID == "validation_release" {
			m.State = Ready
			m.ApplicabilityReason = "按域校验、审核和发布"
		}
		for _, input := range d.Inputs {
			digest := Digest([]any{revision, input.Name})
			binding := InputBinding{Name: input.Name, Output: input.Name, Required: input.Required, Available: outputs[input.Name], RevisionID: &revision, Digest: &digest}
			if input.Name == "facts" {
				binding.Available = regions[d.ID] > 0 || counts[d.ID] > 0
			}
			for _, dep := range d.Dependencies {
				if dep.Output == input.Name {
					producer := StableID(source, "module", dep.ModuleID)
					binding.ProducerInstanceID = &producer
					break
				}
			}
			// Network designs can consume a published physical revision from a different source.
			if d.ID == "network_design" && input.Name == "connectivity" && len(model.NetworkPlans) > 0 && model.NetworkPlans[0].PhysicalRevisionID != "" {
				physical := model.NetworkPlans[0].PhysicalRevisionID
				binding.RevisionID = &physical
				binding.ProducerInstanceID = nil
				binding.Available = true
				value := Digest(physical)
				binding.Digest = &value
			}
			if input.Name == "facts" {
				binding.ProducerInstanceID = nil
			}
			if binding.ProducerInstanceID != nil && binding.Available {
				binding.ProducerInstanceID = nil
			} // resolved immutable output, not an unexecuted producer
			m.Inputs = append(m.Inputs, binding)
		}
		plan.Modules = append(plan.Modules, m)
	}
	return plan
}

// EffectivePlan applies persisted review/job states without creating a second
// mutable authority for approvals and publication receipts.
func EffectivePlan(plan ImportPlan, targets []TargetStatus, job *EngineeringJob) PlanView {
	r := DefaultRegistry()
	view := PlanView{Plan: plan, Definitions: r.Definitions(), Batches: [][]ID{}, Unavailable: []PlanBlock{}}
	for i := range view.Plan.Modules {
		m := &view.Plan.Modules[i]
		d, _ := r.Definition(m.DefinitionID)
		states := []ModuleState{}
		for _, target := range targets {
			for _, declared := range d.PublishTargets {
				if declared == target.Target && target.State != NeedsInput && target.State != NotApplicable {
					states = append(states, target.State)
				}
			}
		}
		if len(states) > 0 {
			m.State = Published
			// Least complete relevant output determines the module summary; each target still retains its own independent status.
			priority := map[ModuleState]int{Published: 0, Approved: 1, InReview: 2, Ready: 3, NeedsInput: 4, Detected: 5, Processing: 6, Stale: 7, Blocked: 8}
			for _, state := range states {
				if priority[state] > priority[m.State] {
					m.State = state
				}
			}
		}
		if job != nil && (job.State == "pending" || job.State == "running") && m.DefinitionID == "workbook_intelligence" {
			m.State = Processing
		}
		if job != nil && job.State == "failed" && m.DefinitionID == "workbook_intelligence" {
			m.State = Blocked
		}
	}
	batches, blocked, err := r.PlanBatches(view.Plan)
	if err == nil {
		view.Batches = batches
	}
	for i := range view.Plan.Modules {
		m := &view.Plan.Modules[i]
		if inputs := blocked[m.ID]; len(inputs) > 0 && m.State != NotApplicable {
			view.Unavailable = append(view.Unavailable, PlanBlock{m.ID, inputs})
			if m.State == Detected || m.State == Ready || m.State == InReview {
				m.State = NeedsInput
			}
		}
	}
	return view
}
