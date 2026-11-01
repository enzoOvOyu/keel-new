package main

import "keel.local/dev/internal/engineering"

// Include only this cable's physical inputs; unrelated racks do not invalidate it.
func cableInputDigest(m engineering.PhysicalModel, c engineering.CablePlan) string {
	inputs := engineering.EmptyPhysical(m.ProjectID)
	equipment, facilities, ports := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, v := range m.Connections {
		if v.EntityID == c.ConnectionID {
			inputs.Connections = append(inputs.Connections, v)
			ports[v.APortID] = true
			ports[v.ZPortID] = true
			for _, id := range v.PatchPortIDs {
				ports[id] = true
			}
		}
	}
	for _, v := range m.Ports {
		if ports[v.EntityID] {
			inputs.Ports = append(inputs.Ports, v)
			equipment[v.EquipmentID] = true
		}
	}
	for _, v := range m.Placements {
		if equipment[v.EquipmentID] {
			inputs.Placements = append(inputs.Placements, v)
			facilities[v.RackID] = true
		}
	}
	for _, v := range c.Route {
		facilities[v.FromID] = true
		facilities[v.ToID] = true
	}
	for changed := true; changed; {
		changed = false
		for _, v := range m.Facilities {
			if facilities[v.EntityID] && v.ParentID != nil && !facilities[*v.ParentID] {
				facilities[*v.ParentID] = true
				changed = true
			}
		}
	}
	for _, v := range m.Facilities {
		if facilities[v.EntityID] {
			inputs.Facilities = append(inputs.Facilities, v)
		}
	}
	c.Estimated = nil
	c.Calculated = nil
	c.SelectedStock = nil
	c.Confirmed = nil
	c.AsBuilt = nil
	inputs.Cables = []engineering.CablePlan{c}
	return suggestionDigest(inputs, engineering.CablingTarget)
}
func invalidateCableInputs(before engineering.PhysicalModel, x *engineering.Extraction) {
	prior := map[string]engineering.CablePlan{}
	for _, c := range before.Cables {
		prior[c.EntityID] = c
	}
	cleared := map[string]map[string]bool{}
	for i := range x.Model.Cables {
		c := &x.Model.Cables[i]
		old, ok := prior[c.EntityID]
		if !ok {
			continue
		}
		physicalChanged := cableInputDigest(before, old) != cableInputDigest(x.Model, *c)
		lengthChanged := engineering.Digest([]any{old.Estimated, old.Calculated, old.Confirmed}) != engineering.Digest([]any{c.Estimated, c.Calculated, c.Confirmed})
		fields := map[string]bool{}
		if physicalChanged && c.Calculated != nil && engineering.Digest(c.Calculated) == engineering.Digest(old.Calculated) {
			c.Calculated = nil
			fields["length"] = true
			fields["cable_calculated"] = true
		}
		if (physicalChanged || lengthChanged) && c.SelectedStock != nil && c.SelectedStock.CalculationRunID != nil && engineering.Digest(c.SelectedStock) == engineering.Digest(old.SelectedStock) {
			c.SelectedStock = nil
			fields["standard_length"] = true
			fields["cable_selected_stock"] = true
		}
		cleared[c.EntityID] = fields
	}
	evidence := []engineering.SourceEvidence{}
	for _, v := range x.Evidence {
		if v.CalculationRunID != nil && cleared[v.EntityID][v.Field] {
			continue
		}
		evidence = append(evidence, v)
	}
	x.Evidence = evidence
}
