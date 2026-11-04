package ai

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
)

// The model proposes identities and relations, not inventory values. All claims
// point back to source cells; their full-row checks run against the local workbook.
type KickoffCellRef struct {
	Sheet string `json:"sheet"`
	Cell  string `json:"cell"`
}
type KickoffEntityAssessment struct {
	Region   string           `json:"region"`
	Kind     string           `json:"kind"`
	Reason   string           `json:"reason"`
	Evidence []KickoffCellRef `json:"evidence"`
}
type KickoffRelationSource struct {
	Sheet    string `json:"sheet"`
	Column   int    `json:"column"`
	StartRow int    `json:"start_row"`
	EndRow   int    `json:"end_row"`
}
type KickoffRelationTarget struct {
	Sheet       string `json:"sheet"`
	KeyColumns  []int  `json:"key_columns"`
	ValueColumn int    `json:"value_column"`
	StartRow    int    `json:"start_row"`
	EndRow      int    `json:"end_row"`
}
type KickoffRelationAlias struct {
	From     KickoffCellRef   `json:"from"`
	To       KickoffCellRef   `json:"to"`
	Reason   string           `json:"reason"`
	Evidence []KickoffCellRef `json:"evidence"`
}
type KickoffRelation struct {
	ID       string                  `json:"id"`
	Kind     string                  `json:"kind"`
	Source   KickoffRelationSource   `json:"source"`
	Targets  []KickoffRelationTarget `json:"targets"`
	Aliases  []KickoffRelationAlias  `json:"aliases,omitempty"`
	Reason   string                  `json:"reason"`
	Evidence []KickoffCellRef        `json:"evidence"`
}
type KickoffRelationshipPlan struct {
	Entities  []KickoffEntityAssessment `json:"entities"`
	Relations []KickoffRelation         `json:"relations"`
	Missing   []string                  `json:"missing_inputs"`
}
type kickoffRelationMatch struct {
	Cell      bom.Cell
	DataStart int
	Evidence  []KickoffCellRef
	Reason    string
}
type kickoffRelationRow struct {
	Source  KickoffCellRef   `json:"source"`
	State   string           `json:"state"`
	Targets []KickoffCellRef `json:"targets"`
}
type kickoffRelationReport struct {
	ID         string               `json:"id"`
	Total      int                  `json:"total"`
	Matched    int                  `json:"matched"`
	Missing    int                  `json:"missing"`
	Ambiguous  int                  `json:"ambiguous"`
	Examples   []kickoffRelationRow `json:"examples"`
	Exceptions []kickoffRelationRow `json:"exceptions"`
}
type kickoffRelationIndex map[string]map[KickoffCellRef]kickoffRelationMatch

func kickoffCellIndex(w bom.Workbook) map[KickoffCellRef]bom.Cell {
	out := map[KickoffCellRef]bom.Cell{}
	for _, s := range w.Sheets {
		for _, c := range s.Cells {
			c.Sheet = s.Name
			out[KickoffCellRef{s.Name, c.Ref}] = c
		}
	}
	return out
}
func kickoffRelationKey(v string) string {
	return strings.ToLower(strings.Join(strings.Fields(v), " "))
}
func kickoffEvidenceExists(cells map[KickoffCellRef]bom.Cell, refs []KickoffCellRef) bool {
	if len(refs) == 0 {
		return false
	}
	for _, r := range refs {
		c, ok := cells[r]
		if !ok || c.Sensitive || bom.Sensitive(c.Value) || strings.TrimSpace(c.Value) == "" {
			return false
		}
	}
	return true
}
func validateKickoffRelations(w bom.Workbook, plan KickoffRelationshipPlan) error {
	cells := kickoffCellIndex(w)
	ids := map[string]bool{}
	if len(plan.Relations) > 128 {
		return fmt.Errorf("RELATION_BATCH_TOO_LARGE")
	}
	for _, r := range plan.Relations {
		if r.ID == "" || ids[r.ID] || strings.TrimSpace(r.Reason) == "" || !kickoffEvidenceExists(cells, r.Evidence) {
			return fmt.Errorf("RELATION_EVIDENCE_REQUIRED: %s", r.ID)
		}
		ids[r.ID] = true
		switch r.Kind {
		case "device_identity", "placement", "cable_spec", "model_spec":
		default:
			return fmt.Errorf("RELATION_KIND_INVALID: %s", r.ID)
		}
		if !kickoffBounds(w, r.Source.Sheet, engineering.Bounds{StartRow: r.Source.StartRow, EndRow: r.Source.EndRow, StartColumn: r.Source.Column, EndColumn: r.Source.Column}) || len(r.Targets) == 0 {
			return fmt.Errorf("RELATION_RANGE_INVALID: %s", r.ID)
		}
		for _, t := range r.Targets {
			if len(t.KeyColumns) == 0 {
				return fmt.Errorf("RELATION_KEYS_REQUIRED: %s", r.ID)
			}
			for _, k := range t.KeyColumns {
				if !kickoffBounds(w, t.Sheet, engineering.Bounds{StartRow: t.StartRow, EndRow: t.EndRow, StartColumn: min(k, t.ValueColumn), EndColumn: max(k, t.ValueColumn)}) {
					return fmt.Errorf("RELATION_TARGET_INVALID: %s", r.ID)
				}
			}
		}
		for _, a := range r.Aliases {
			if a.Reason == "" || len(a.Evidence) < 2 || !kickoffEvidenceExists(cells, append(append([]KickoffCellRef{}, a.Evidence...), a.From, a.To)) {
				return fmt.Errorf("ALIAS_CORROBORATION_REQUIRED: %s", r.ID)
			}
			c := cells[a.From]
			if c.Sheet != r.Source.Sheet || c.Column != r.Source.Column || c.Row < r.Source.StartRow || c.Row > r.Source.EndRow {
				return fmt.Errorf("ALIAS_SOURCE_INVALID: %s", r.ID)
			}
			target := cells[a.To]
			found := false
			for _, t := range r.Targets {
				for _, k := range t.KeyColumns {
					if target.Sheet == t.Sheet && target.Column == k && target.Row >= t.StartRow && target.Row <= t.EndRow {
						found = true
					}
				}
			}
			if !found {
				return fmt.Errorf("ALIAS_TARGET_INVALID: %s", r.ID)
			}
		}
	}
	return nil
}

// Whole-column association evaluation. It does not use sheet names, model names,
// role strings or substring similarity to decide which records are equipment.
func evaluateKickoffRelations(w bom.Workbook, plan KickoffRelationshipPlan) (kickoffRelationIndex, []kickoffRelationReport, error) {
	if err := validateKickoffRelations(w, plan); err != nil {
		return nil, nil, err
	}
	cells := kickoffCellIndex(w)
	byPosition := map[string]map[[2]int]bom.Cell{}
	for _, s := range w.Sheets {
		byPosition[s.Name] = map[[2]int]bom.Cell{}
		for _, c := range s.Cells {
			c.Sheet = s.Name
			byPosition[s.Name][[2]int{c.Row, c.Column}] = c
		}
	}
	index := kickoffRelationIndex{}
	reports := []kickoffRelationReport{}
	for _, r := range plan.Relations {
		keys := map[string][]kickoffRelationMatch{}
		for _, t := range r.Targets {
			for row := t.StartRow; row <= t.EndRow; row++ {
				v := byPosition[t.Sheet][[2]int{row, t.ValueColumn}]
				if v.Value == "" || v.Sensitive || bom.Sensitive(v.Value) || v.Formula != "" {
					continue
				}
				for _, k := range t.KeyColumns {
					key := byPosition[t.Sheet][[2]int{row, k}]
					if key.Value == "" || key.Sensitive || bom.Sensitive(key.Value) || key.Formula != "" {
						continue
					}
					keys[kickoffRelationKey(key.Value)] = append(keys[kickoffRelationKey(key.Value)], kickoffRelationMatch{Cell: v, DataStart: t.StartRow, Evidence: []KickoffCellRef{{key.Sheet, key.Ref}, {v.Sheet, v.Ref}}, Reason: r.Reason})
				}
			}
		}
		aliases := map[KickoffCellRef][]kickoffRelationMatch{}
		for _, a := range r.Aliases {
			target := cells[a.To]
			for _, t := range r.Targets {
				if target.Sheet != t.Sheet || target.Row < t.StartRow || target.Row > t.EndRow {
					continue
				}
				v := byPosition[t.Sheet][[2]int{target.Row, t.ValueColumn}]
				if v.Value != "" && !v.Sensitive && !bom.Sensitive(v.Value) && v.Formula == "" {
					aliases[a.From] = append(aliases[a.From], kickoffRelationMatch{Cell: v, DataStart: t.StartRow, Evidence: append(append([]KickoffCellRef{}, a.Evidence...), a.From, a.To, KickoffCellRef{v.Sheet, v.Ref}), Reason: a.Reason})
				}
			}
		}
		found := map[KickoffCellRef]kickoffRelationMatch{}
		report := kickoffRelationReport{ID: r.ID, Examples: []kickoffRelationRow{}, Exceptions: []kickoffRelationRow{}}
		for row := r.Source.StartRow; row <= r.Source.EndRow; row++ {
			c := byPosition[r.Source.Sheet][[2]int{row, r.Source.Column}]
			if c.Value == "" || c.Sensitive || bom.Sensitive(c.Value) {
				continue
			}
			ref := KickoffCellRef{c.Sheet, c.Ref}
			report.Total++
			matches := []kickoffRelationMatch{}
			if c.Formula == "" {
				matches = append(matches, keys[kickoffRelationKey(c.Value)]...)
				matches = append(matches, aliases[ref]...)
			}
			unique := map[string]kickoffRelationMatch{}
			for _, m := range matches {
				unique[strings.TrimSpace(m.Cell.Value)] = m
			}
			detail := kickoffRelationRow{Source: ref, Targets: []KickoffCellRef{}}
			for _, m := range unique {
				detail.Targets = append(detail.Targets, KickoffCellRef{m.Cell.Sheet, m.Cell.Ref})
			}
			sort.Slice(detail.Targets, func(i, j int) bool {
				return detail.Targets[i].Sheet+detail.Targets[i].Cell < detail.Targets[j].Sheet+detail.Targets[j].Cell
			})
			switch len(unique) {
			case 0:
				report.Missing++
				detail.State = "unresolved"
			case 1:
				report.Matched++
				detail.State = "matched"
				for _, m := range unique {
					m.Evidence = append([]KickoffCellRef{ref}, m.Evidence...)
					found[ref] = m
				}
			default:
				report.Ambiguous++
				detail.State = "conflict"
			}
			if detail.State == "matched" && len(report.Examples) < 8 {
				report.Examples = append(report.Examples, detail)
			} else if detail.State != "matched" && len(report.Exceptions) < 24 {
				report.Exceptions = append(report.Exceptions, detail)
			}
		}
		index[r.ID] = found
		reports = append(reports, report)
	}
	return index, reports, nil
}

// Search is evidence retrieval, not an identity decision. Returns redacted row
// context and real coordinates, never original labels or credentials to the model.
func kickoffRelatedRecords(original, safe bom.Workbook, ref KickoffCellRef, offset int) (any, error) {
	cells := kickoffCellIndex(original)
	source, ok := cells[ref]
	if !ok || !kickoffEvidenceExists(cells, []KickoffCellRef{ref}) {
		return nil, fmt.Errorf("RELATION_SOURCE_NOT_FOUND")
	}
	if offset < 0 {
		return nil, fmt.Errorf("INVALID_OFFSET")
	}
	key := kickoffRelationKey(source.Value)
	if len(key) < 2 {
		return nil, fmt.Errorf("RELATION_SOURCE_TOO_SHORT")
	}
	hits := []KickoffCellRef{}
	for _, s := range original.Sheets {
		for _, c := range s.Cells {
			if c.Sensitive || bom.Sensitive(c.Value) || c.Value == "" {
				continue
			}
			value := kickoffRelationKey(c.Value)
			if value == key || strings.Contains(value, key) || strings.Contains(key, value) && len(value) >= 4 {
				hits = append(hits, KickoffCellRef{s.Name, c.Ref})
			}
		}
	}
	out := []any{}
	for _, hit := range hits[min(offset, len(hits)):min(offset+12, len(hits))] {
		c := cells[hit]
		s, _ := kickoffSheet(safe, hit.Sheet)
		row := []bom.Cell{}
		for _, v := range s.Cells {
			if v.Row == c.Row {
				row = append(row, v)
			}
		}
		out = append(out, map[string]any{"source": hit, "row_context": engineeringCellPreview(row), "match_is_identity_proof": false})
	}
	return map[string]any{"total": len(hits), "offset": offset, "items": out, "has_more": offset+12 < len(hits)}, nil
}

func validateKickoffRelationshipBindings(w bom.Workbook, regions []KickoffRegion, plan KickoffRelationshipPlan, task KickoffTaskResult) error {
	kinds := map[string]string{}
	for _, e := range plan.Entities {
		kinds[e.Region] = e.Kind
	}
	relations := map[string]KickoffRelation{}
	for _, r := range plan.Relations {
		relations[r.ID] = r
	}
	for _, m := range task.Mappings {
		table := strings.TrimPrefix(m.Table, "kickoff23/")
		if table == "devices" && kinds[m.Region] != "device" && kinds[m.Region] != "connection" {
			return fmt.Errorf("NOT_ASSESSED_AS_DEVICE: %s", m.Region)
		}
		sheet := ""
		for _, r := range regions {
			if r.ID == m.Region {
				sheet = r.Sheet
			}
		}
		bound := map[string]bool{}
		for _, b := range m.Bindings {
			bound[b.Field] = true
			if table == "connections" && (b.Field == "a_serial" || b.Field == "b_serial") && b.Relationship == "" {
				return fmt.Errorf("ENDPOINT_RELATION_REQUIRED: %s/%s", m.Region, b.Field)
			}
			if table == "devices" && kinds[m.Region] == "connection" && b.Field == "serial" && b.Relationship == "" {
				return fmt.Errorf("ENDPOINT_RELATION_REQUIRED: %s/serial", m.Region)
			}
			if b.Relationship == "" {
				continue
			}
			relation, ok := relations[b.Relationship]
			if !ok || relation.Source.Sheet != sheet || b.Column != relation.Source.Column || m.Bounds.StartRow < relation.Source.StartRow || m.Bounds.EndRow > relation.Source.EndRow {
				return fmt.Errorf("RELATION_SOURCE_MISMATCH: %s", b.Relationship)
			}
			identity := table == "devices" && b.Field == "serial" || b.Field == "a_serial" || b.Field == "b_serial" || table == "placements" && b.Field == "device_id"
			if identity {
				if relation.Kind != "device_identity" {
					return fmt.Errorf("DEVICE_RELATION_REQUIRED: %s", b.Relationship)
				}
				for _, target := range relation.Targets {
					trusted := false
					for _, region := range regions {
						if (kinds[region.ID] == "device" || kinds[region.ID] == "connection") && region.Sheet == target.Sheet && target.StartRow >= region.Bounds.StartRow && target.EndRow <= region.Bounds.EndRow && target.ValueColumn >= region.Bounds.StartColumn && target.ValueColumn <= region.Bounds.EndColumn {
							trusted = true
						}
					}
					if !trusted {
						return fmt.Errorf("RELATION_TARGET_NOT_DEVICE: %s", b.Relationship)
					}
				}
			}
		}
		if table == "devices" && kinds[m.Region] == "connection" && (!bound["serial"] || !bound["label"]) {
			return fmt.Errorf("ENDPOINT_DEVICE_IDENTITY_REQUIRED: %s", m.Region)
		}
		if table == "connections" {
			for _, field := range []string{"a_serial", "b_serial", "a_port", "b_port"} {
				if !bound[field] {
					return fmt.Errorf("ENDPOINT_BINDING_REQUIRED: %s/%s", m.Region, field)
				}
			}
		}
	}
	return nil
}

// Only independently calculated compatible cells enter local relationship joins.
// Safe previews retain opaque labels so text identities never enter model input.
func kickoffVerifiedWorkbook(w bom.Workbook, values map[string]KickoffVerifiedCell, safe bool) bom.Workbook {
	out := w
	out.Sheets = append([]bom.Sheet{}, w.Sheets...)
	for i := range out.Sheets {
		out.Sheets[i].Cells = append([]bom.Cell{}, w.Sheets[i].Cells...)
		for j := range out.Sheets[i].Cells {
			c := &out.Sheets[i].Cells[j]
			v, ok := values[out.Sheets[i].Name+"!"+c.Ref]
			if !ok || c.Formula == "" || c.Sensitive || bom.Sensitive(v.Value) {
				continue
			}
			c.Value = v.Value
			c.Formula = ""
			c.FormulaBacked = true
			if safe {
				if _, err := strconv.ParseFloat(v.Value, 64); err != nil || strings.HasPrefix(w.Sheets[i].Cells[j].Value, "[IDENTITY") {
					c.Value = "[LABEL-" + bom.StableID(v.Value) + "]"
				}
			}
		}
	}
	return out
}
