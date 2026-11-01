package engineering

import (
	"keel.local/dev/internal/bom"
	"strings"
)

// ResolveSourceConstants restores a literal only when the model cited a cell
// whose sanitized value exactly matches its proposal. Classification constants
// remain semantic suggestions, while customer identities never leave local IR.
func ResolveSourceConstants(original, sanitized bom.Workbook, p EngineeringProposal) EngineeringProposal {
	raw := map[string]bom.Cell{}
	safe := map[string]bom.Cell{}
	for _, s := range original.Sheets {
		for _, c := range s.Cells {
			raw[s.Name+"!"+c.Ref] = c
		}
	}
	for _, s := range sanitized.Sheets {
		for _, c := range s.Cells {
			safe[s.Name+"!"+c.Ref] = c
		}
	}
	p.Regions = append([]RegionMapping{}, p.Regions...)
	for i := range p.Regions {
		r := &p.Regions[i]
		r.Constants = append([]ConstantBinding{}, r.Constants...)
		for j := range r.Constants {
			c := &r.Constants[j]
			switch c.Field {
			case "equipment_type", "identity_kind", "relation_type", "mount_face", "role", "medium", "length_state", "power_basis", "cooling_type", "work_type", "fabric_kind", "allocation_kind":
				continue
			}
			found, ambiguous := false, false
			value := ""
			for _, ref := range c.EvidenceCells {
				a, aok := raw[r.Sheet+"!"+ref]
				b, bok := safe[r.Sheet+"!"+ref]
				if !aok || !bok || a.Sensitive || b.Sensitive || strings.TrimSpace(b.Value) != strings.TrimSpace(c.Value) {
					continue
				}
				if found && value != a.Value {
					ambiguous = true
				}
				value = a.Value
				found = true
			}
			if found && !ambiguous {
				c.Value = value
			}
		}
	}
	return p
}
