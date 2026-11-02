package ai

import (
	"encoding/json"
	"fmt"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"strings"
)

type engineeringAction struct {
	Action   string                           `json:"action"`
	Sheet    string                           `json:"sheet,omitempty"`
	Bounds   *engineering.Bounds              `json:"bounds,omitempty"`
	Cell     string                           `json:"cell,omitempty"`
	Proposal *engineering.EngineeringProposal `json:"proposal,omitempty"`
	Rule     *engineering.CalculationRule     `json:"rule,omitempty"`
	RuleRef  *engineering.VersionReference    `json:"rule_ref,omitempty"`
	Cells    []engineering.FormulaCell        `json:"cells,omitempty"`
	Inputs   []engineering.NamedValue         `json:"inputs,omitempty"`
}

func mergeEngineeringProposal(base, part engineering.EngineeringProposal, sheet string) engineering.EngineeringProposal {
	out := engineering.EngineeringProposal{Summary: base.Summary, Regions: []engineering.RegionMapping{}, Modules: append([]engineering.ModuleDetection{}, base.Modules...), Questions: append([]string{}, base.Questions...)}
	if sheet != "" {
		out.Summary += sheet + ": " + part.Summary + "\n"
	} else if out.Summary == "" {
		out.Summary = part.Summary
	}
	for _, r := range base.Regions {
		if sheet == "" || r.Sheet != sheet {
			out.Regions = append(out.Regions, r)
		}
	}
	out.Regions = append(out.Regions, part.Regions...)
	for _, m := range part.Modules {
		found := false
		for i := range out.Modules {
			old := &out.Modules[i]
			if old.ModuleID == m.ModuleID {
				found = true
				old.Applicable = old.Applicable || m.Applicable
				old.Sheets = appendUnique(old.Sheets, m.Sheets...)
				if !strings.Contains(old.Reason, m.Reason) {
					old.Reason += "; " + m.Reason
				}
				break
			}
		}
		if !found {
			out.Modules = append(out.Modules, m)
		}
	}
	out.Questions = appendUnique(out.Questions, part.Questions...)
	return out
}

func appendUnique(values []string, more ...string) []string {
	for _, v := range more {
		found := false
		for _, old := range values {
			if old == v {
				found = true
				break
			}
		}
		if !found {
			values = append(values, v)
		}
	}
	return values
}

// Structural blocks are hints only. The agent still chooses semantics and mappings.
// Samples bound the prompt while extraction always reads every mapped source row.

func engineeringSheetHistory(history []any, sheet string) []any {
	type sheetAction struct {
		Sheet    string `json:"sheet"`
		Proposal *struct {
			Regions []struct {
				Sheet string `json:"sheet"`
			} `json:"regions"`
		} `json:"proposal"`
	}
	filtered := []any{}
	for _, entry := range history {
		raw, _ := json.Marshal(entry)
		var step struct {
			Action   sheetAction `json:"action"`
			Rejected sheetAction `json:"rejected_action"`
		}
		if json.Unmarshal(raw, &step) != nil {
			continue
		}
		matches := step.Action.Sheet == sheet || step.Rejected.Sheet == sheet
		for _, action := range []sheetAction{step.Action, step.Rejected} {
			proposal := action.Proposal
			if proposal != nil {
				for _, region := range proposal.Regions {
					matches = matches || region.Sheet == sheet
				}
			}
		}
		if matches {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func constrainEngineeringTools(functions []any, attributes []engineering.AttributeDefinition) {
	fields := engineering.BindingFields()
	for _, attribute := range attributes {
		if attribute.ApprovalStatus == "approved" {
			fields = append(fields, fmt.Sprintf("attribute:%s@%d", attribute.ID, attribute.Version))
		}
	}
	modules := []string{}
	for _, module := range engineering.DefaultRegistry().Definitions() {
		modules = append(modules, module.ID)
	}
	for _, entry := range functions {
		function := entry.(map[string]any)["function"].(map[string]any)
		parameters := function["parameters"].(map[string]any)
		definitions, _ := parameters["$defs"].(map[string]any)
		engineeringColumnMapSchema(definitions, fields)
		for _, name := range []string{"Column", "Constant", "CellBinding", "Module"} {
			definition, ok := definitions[name].(map[string]any)
			if !ok {
				continue
			}
			properties := definition["properties"].(map[string]any)
			field := "field"
			allowed := fields
			if name == "Module" {
				field = "module_id"
				allowed = modules
			}
			properties[field].(map[string]any)["enum"] = allowed
		}
	}
}

// Keep source evidence in context until that sheet is understood. This sequences
// inspection, not engineering-domain applicability or dependency execution.

func engineeringSourceTools(functions []any, sheet string, inspected, propose bool) []any {
	selected := []any{}
	for _, tool := range functions {
		function := tool.(map[string]any)["function"].(map[string]any)
		name := function["name"].(string)
		enabled := name == "inspect_sheet" && !inspected
		if inspected {
			switch name {
			case "inspect_region", "read_range", "inspect_formula", "trace_formula_dependencies", "resolve_cross_sheet_references", "propose_engineering_model":
				enabled = true
			}
		}
		if propose {
			enabled = name == "propose_engineering_model"
		}
		if !enabled {
			continue
		}
		properties := function["parameters"].(map[string]any)["properties"].(map[string]any)
		if value, ok := properties["sheet"].(map[string]any); ok {
			value["enum"] = []string{sheet}
		}
		selected = append(selected, tool)
	}
	return selected
}

// Once a complete small sheet has been read, or a read is being repeated with
// no new evidence, require a proposal/explicit uncertainty instead of looping.

func engineeringProposalDue(sheet bom.Sheet, history []any) bool {
	seen := map[string]bool{}
	reads := 0
	for _, entry := range history {
		raw, _ := json.Marshal(entry)
		var step struct {
			Action engineeringAction `json:"action"`
			Result json.RawMessage   `json:"result"`
		}
		if json.Unmarshal(raw, &step) != nil || step.Action.Sheet != sheet.Name {
			continue
		}
		var failure map[string]json.RawMessage
		if json.Unmarshal(step.Result, &failure) == nil && failure["error"] != nil {
			var detail string
			_ = json.Unmarshal(failure["error"], &detail)
			if step.Action.Action == "propose_engineering_model" && (strings.HasPrefix(detail, "FIELD_NOT_SUPPORTED_FOR_REGION") || strings.HasPrefix(detail, "DUPLICATE_FIELD_BINDING")) {
				return true
			}
			continue
		}
		action := step.Action
		switch action.Action {
		case "inspect_sheet", "inspect_region", "read_range", "inspect_formula", "trace_formula_dependencies", "resolve_cross_sheet_references":
			reads++
		}
		switch action.Action {
		case "inspect_sheet":
			var result struct {
				Samples []any `json:"sample_cells"`
			}
			_ = json.Unmarshal(step.Result, &result)
			visible := 0
			for _, cell := range sheet.Cells {
				if !cell.Sensitive {
					visible++
				}
			}
			if visible > 0 && len(result.Samples) == visible {
				return true
			}
		case "read_range", "inspect_region":
			if action.Bounds == nil {
				continue
			}
			var page struct {
				Bounds *engineering.Bounds `json:"returned_bounds"`
			}
			if json.Unmarshal(step.Result, &page) == nil && page.Bounds != nil {
				action.Bounds = page.Bounds
			}
			key := engineering.Digest(struct {
				Sheet  string
				Bounds engineering.Bounds
			}{sheet.Name, *action.Bounds})
			if seen[key] {
				return true
			}
			seen[key] = true
			complete := true
			for _, cell := range sheet.Cells {
				b := action.Bounds
				if !cell.Sensitive && (cell.Row < b.StartRow || cell.Row > b.EndRow || cell.Column < b.StartColumn || cell.Column > b.EndColumn) {
					complete = false
					break
				}
			}
			if complete {
				return true
			}
		}
	}
	// Bounded history keeps the latest six steps. A full window of source reads
	// must produce an interpretation, rather than endlessly replacing evidence.
	return reads >= 6
}

// Match the existing BOM Agent's bounded-page behavior. Requested bounds are
// never mistaken for the extent actually returned to the model.

func engineeringBindingErrors(proposal engineering.EngineeringProposal, attributes []engineering.AttributeDefinition) []any {
	issues := []any{}
	for index, region := range proposal.Regions {
		allowed := map[string]bool{}
		for _, field := range engineering.RegionFields(region.Kind) {
			allowed[field] = true
		}
		for _, attribute := range attributes {
			if attribute.ApprovalStatus == "approved" {
				allowed[fmt.Sprintf("attribute:%s@%d", attribute.ID, attribute.Version)] = true
			}
		}
		seen := map[string]bool{}
		for _, binding := range region.Columns {
			reason := ""
			if !allowed[binding.Field] {
				reason = "unsupported field: use remove_columns to remove this field; setting other fields leaves it unchanged"
			} else if seen[binding.Field] {
				reason = "duplicate field: each object has one binding per field"
			}
			seen[binding.Field] = true
			if reason != "" {
				issues = append(issues, map[string]any{"region_index": index, "kind": region.Kind, "field": binding.Field, "column": binding.Column, "reason": reason})
			}
		}
		for _, binding := range region.Constants {
			reason := ""
			if !allowed[binding.Field] {
				reason = "unsupported constant field"
			} else if seen[binding.Field] {
				reason = "duplicate constant/column field: different source objects need records or separate regions, not repeated constants"
			} else if len(binding.EvidenceCells) == 0 {
				reason = "constant needs existing source evidence cells"
			}
			seen[binding.Field] = true
			if reason != "" {
				issues = append(issues, map[string]any{"region_index": index, "kind": region.Kind, "field": binding.Field, "reason": reason})
			}
		}
	}
	return issues
}

func engineeringRegionCapabilities() map[string][]string {
	out := map[string][]string{}
	for _, kind := range []string{"equipment", "assembly", "site", "room", "rack", "placement", "port", "connection", "cable", "power", "material", "workload"} {
		out[kind] = engineering.RegionFields(kind)
	}
	return out
}

// Reuse human-review source checks, but omit repeated API bookkeeping from model context.

func engineeringProposedKinds(history []any, sheet string) []string {
	for i := len(history) - 1; i >= 0; i-- {
		raw, _ := json.Marshal(history[i])
		var step struct {
			Action engineeringAction `json:"action"`
		}
		if json.Unmarshal(raw, &step) != nil || step.Action.Proposal == nil {
			continue
		}
		kinds := []string{}
		matched := false
		for _, r := range step.Action.Proposal.Regions {
			if r.Sheet == sheet {
				matched = true
				if r.Kind != "other" {
					kinds = appendUnique(kinds, r.Kind)
				}
			}
		}
		if matched {
			return kinds
		}
	}
	return nil
}

func scopeEngineeringProposalTools(selected, functions []any, kinds []string, attributes []engineering.AttributeDefinition) []any {
	fields := []string{}
	for _, kind := range kinds {
		fields = appendUnique(fields, engineering.RegionFields(kind)...)
	}
	for _, a := range attributes {
		if a.ApprovalStatus == "approved" {
			fields = appendUnique(fields, fmt.Sprintf("attribute:%s@%d", a.ID, a.Version))
		}
	}
	out := append([]any{}, selected...)
	for i, tool := range out {
		f := tool.(map[string]any)["function"].(map[string]any)
		if f["name"] != "propose_engineering_model" {
			continue
		}
		raw, _ := json.Marshal(tool)
		var copy map[string]any
		_ = json.Unmarshal(raw, &copy)
		defs := copy["function"].(map[string]any)["parameters"].(map[string]any)["$defs"].(map[string]any)
		for _, name := range []string{"Constant", "CellBinding"} {
			defs[name].(map[string]any)["properties"].(map[string]any)["field"].(map[string]any)["enum"] = fields
		}
		engineeringColumnMapSchema(defs, fields)
		defs["Region"].(map[string]any)["properties"].(map[string]any)["kind"].(map[string]any)["enum"] = appendUnique(append([]string{}, kinds...), "other")
		namespaceEngineeringDefinitions(copy["function"].(map[string]any)["parameters"].(map[string]any), "mapping_")
		out[i] = copy
	}
	for _, tool := range functions {
		if tool.(map[string]any)["function"].(map[string]any)["name"] == "classify_information" {
			out = append(out, tool)
			break
		}
	}
	return out
}

// Some native tool providers combine definitions across tools in one namespace.
// Scoped mapping fields must not collide with the broader classification schema.

func namespaceEngineeringDefinitions(parameters map[string]any, prefix string) {
	var rewrite func(any)
	rewrite = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if ref, ok := v["$ref"].(string); ok && strings.HasPrefix(ref, "#/$defs/") {
				v["$ref"] = "#/$defs/" + prefix + strings.TrimPrefix(ref, "#/$defs/")
			}
			for _, child := range v {
				rewrite(child)
			}
		case []any:
			for _, child := range v {
				rewrite(child)
			}
		}
	}
	rewrite(parameters)
	defs := parameters["$defs"].(map[string]any)
	renamed := map[string]any{}
	for key, value := range defs {
		renamed[prefix+key] = value
	}
	parameters["$defs"] = renamed
}
