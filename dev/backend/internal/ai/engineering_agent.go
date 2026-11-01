package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xuri/excelize/v2"
	"io"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"sort"
	"strings"
	"time"
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

// Increment when source interpretation changes invalidate previously accepted mappings.
const engineeringCheckpointVersion = 2

type EngineeringCheckpoint struct {
	Version         int                             `json:"version"`
	Proposal        engineering.EngineeringProposal `json:"proposal"`
	InspectedSheets []string                        `json:"inspected_sheets"`
	History         []any                           `json:"history"`
	CompletedSheets []string                        `json:"completed_sheets"`
}
type EngineeringToolHooks struct {
	Resume      *EngineeringCheckpoint
	Checkpoint  func(context.Context, EngineeringCheckpoint) error
	Artifact    func(context.Context, string, string, any) error
	Attributes  []engineering.AttributeDefinition
	Names       []excelize.DefinedName
	ProposeRule func(context.Context, engineering.CalculationRule) (engineering.RuleView, error)
	Calculate   func(context.Context, *engineering.VersionReference, []engineering.FormulaCell, []engineering.NamedValue) (engineering.CalculationPage, error)
}

// Only sanitized IR enters the tool loop; this function has no write authority.
func (p *Provider) UnderstandEngineering(ctx context.Context, w bom.Workbook, trace func(string, string, string) error, options ...EngineeringToolHooks) (engineering.EngineeringProposal, error) {
	var hooks EngineeringToolHooks
	if len(options) > 0 {
		hooks = options[0]
	}
	if hooks.Resume != nil && hooks.Resume.Version != engineeringCheckpointVersion {
		hooks.Resume = nil
	}
	var graph []engineering.FormulaEvidence
	var proposal engineering.EngineeringProposal
	completed := map[string]bool{}
	if hooks.Resume != nil {
		proposal = hooks.Resume.Proposal
		validRegions := []engineering.RegionMapping{}
		for _, sheet := range hooks.Resume.CompletedSheets {
			part := engineering.EngineeringProposal{Summary: proposal.Summary, Modules: proposal.Modules, Regions: []engineering.RegionMapping{}}
			for _, region := range proposal.Regions {
				if region.Sheet == sheet {
					part.Regions = append(part.Regions, region)
				}
			}
			if engineering.CheckProposal(w, part) == nil && engineering.CheckSheetCoverage(w, part, sheet) == nil {
				completed[sheet] = true
				validRegions = append(validRegions, part.Regions...)
			}
		}
		proposal.Regions = validRegions
	}
	prompt, e := prompts.ReadFile("prompts/engineering-source-v1.txt")
	if e != nil {
		return proposal, e
	}
	var functions []any
	toolSchema, _ := prompts.ReadFile("prompts/engineering-tools.json")
	if err := json.Unmarshal(toolSchema, &functions); err != nil {
		return proposal, err
	}
	constrainEngineeringTools(functions, hooks.Attributes)
	ir := bom.NewWorkbookIR(w)
	history := []any{}
	if hooks.Resume != nil {
		history = boundedEngineeringHistory(hooks.Resume.History)
	}
	registry := engineering.DefaultRegistry().Definitions()
	sheetOrder := append([]bom.Sheet{}, w.Sheets...)
	score := func(sheet bom.Sheet) int {
		headers := map[string]bool{}
		for _, cell := range sheet.Cells {
			if !cell.Sensitive {
				for _, field := range engineering.HeaderFields(cell.Value) {
					headers[field] = true
				}
			}
		}
		return len(headers)
	}
	scores := map[string]int{}
	for _, sheet := range sheetOrder {
		scores[sheet.Name] = score(sheet)
	}
	sort.SliceStable(sheetOrder, func(i, j int) bool { return scores[sheetOrder[i].Name] > scores[sheetOrder[j].Name] })
	summary := []any{}
	for _, s := range w.Sheets {
		formulas := 0
		for _, c := range s.Cells {
			if c.Formula != "" {
				formulas++
			}
		}
		summary = append(summary, map[string]any{"sheet": s.Name, "cells": len(s.Cells), "merge_count": len(s.Merges), "formulas": formulas, "dimension": s.Dimension})
	}
	inspected := map[string]bool{}
	if hooks.Resume != nil {
		for _, sheet := range hooks.Resume.InspectedSheets {
			inspected[sheet] = true
		}
	}
	persist := func() error {
		if hooks.Checkpoint == nil {
			return nil
		}
		checkpoint := EngineeringCheckpoint{Version: engineeringCheckpointVersion, Proposal: proposal, CompletedSheets: []string{}, InspectedSheets: []string{}, History: boundedEngineeringHistory(history)}
		for _, sheet := range w.Sheets {
			if completed[sheet.Name] {
				checkpoint.CompletedSheets = append(checkpoint.CompletedSheets, sheet.Name)
			}
			if inspected[sheet.Name] {
				checkpoint.InspectedSheets = append(checkpoint.InspectedSheets, sheet.Name)
			}
		}
		return hooks.Checkpoint(ctx, checkpoint)
	}
	queued := []json.RawMessage{}
	budget := max(24, len(w.Sheets)*16)
	rejections := map[string]int{}
	for turn := 0; turn < budget; turn++ {
		pending := []string{}
		for _, sheet := range sheetOrder {
			if len(sheet.Cells) > 0 && !completed[sheet.Name] {
				pending = append(pending, sheet.Name)
			}
		}
		if len(pending) == 0 {
			return proposal, nil
		}
		history = boundedEngineeringHistory(history)
		if len(queued) == 0 {
			active := pending[0]
			sheet, _ := ir.AnalyzeSheet(active)
			regions := []bom.Region{}
			for _, region := range ir.Regions {
				if region.Sheet == active {
					regions = append(regions, region)
				}
			}
			selected := engineeringSourceTools(functions, active, inspected[active], engineeringProposalDue(sheet, history))
			kinds := engineeringProposedKinds(history, active)
			if len(kinds) > 0 {
				selected = scopeEngineeringProposalTools(selected, functions, kinds, hooks.Attributes)
			}
			raw, err := p.engineeringCompletion(ctx, string(prompt), map[string]any{"workbook": summary, "modules": registry, "attribute_definitions": hooks.Attributes, "history": history, "active_sheet": active, "proposed_region_kinds": kinds, "active_sheet_preview": engineeringSheetPreview(sheet, regions), "region_field_capabilities": engineeringRegionCapabilities(), "pending_sheets": pending, "completed_sheets": len(completed), "turns_remaining": budget - turn}, selected, trace, hooks.Artifact)
			if err != nil {
				return proposal, err
			}
			if len(raw) > 0 && raw[0] == '[' {
				if err = json.Unmarshal(raw, &queued); err != nil {
					return proposal, err
				}
			} else {
				queued = append(queued, raw)
			}
		}
		if len(queued) == 0 {
			return proposal, errors.New("EMPTY_TOOL_RESPONSE")
		}
		raw := queued[0]
		queued = queued[1:]
		var err error
		record := func(tool, state string, value any) error {
			if hooks.Artifact != nil {
				return hooks.Artifact(ctx, tool, state, map[string]any{"request": json.RawMessage(raw), "result": value})
			}
			return nil
		}
		if schemaErr := validateEngineeringSchema(ctx, raw); schemaErr != nil {
			queued = nil // Remaining calls were planned without this failure feedback.
			if schemaErr.Error() == "AI_SCHEMA_VALIDATOR_UNAVAILABLE" {
				return proposal, schemaErr
			}
			if e := trace("schema_validation", "rejected", schemaErr.Error()); e != nil {
				return proposal, e
			}
			if e := record("schema_validation", "rejected", map[string]string{"error": schemaErr.Error()}); e != nil {
				return proposal, e
			}
			history = append(history, map[string]any{"rejected_action": json.RawMessage(raw), "error": schemaErr.Error()})
			if e := persist(); e != nil {
				return proposal, e
			}
			key := engineering.Digest(string(raw)) + schemaErr.Error()
			rejections[key]++
			if rejections[key] >= 3 {
				return proposal, errors.New("ENGINEERING_AGENT_NO_PROGRESS")
			}
			continue
		}
		var action engineeringAction
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&action)
		if err == nil && decoder.Decode(new(any)) != io.EOF {
			err = fmt.Errorf("INVALID_TOOL_JSON")
		}
		var result any
		if err == nil {
			switch action.Action {
			case "inspect_workbook":
				result = map[string]any{"sheets": summary, "structural_regions": ir.InspectWorkbook()}
			case "inspect_sheet":
				s, ok := ir.AnalyzeSheet(action.Sheet)
				if !ok {
					err = fmt.Errorf("UNKNOWN_SHEET")
					break
				}
				regions := []bom.Region{}
				for _, r := range ir.Regions {
					if r.Sheet == s.Name {
						regions = append(regions, r)
					}
				}
				result = engineeringSheetPreview(s, regions)
				inspected[s.Name] = true
			case "inspect_region", "read_range":
				if action.Bounds == nil {
					err = fmt.Errorf("BOUNDS_REQUIRED")
				} else {
					result, err = engineeringReadRange(w, action.Sheet, *action.Bounds)
					if err == nil {
						inspected[action.Sheet] = true
					}
				}
			case "inspect_formula":
				s, ok := ir.AnalyzeSheet(action.Sheet)
				if !ok {
					err = fmt.Errorf("UNKNOWN_SHEET")
					break
				}
				found := false
				for _, c := range s.Cells {
					if c.Ref == action.Cell && !c.Sensitive {
						found = true
						result = map[string]any{"cell": c.Ref, "formula": c.Formula, "cached_value": c.Value, "recalculated_value": nil, "compatibility": "unknown"}
						break
					}
				}
				if !found {
					err = fmt.Errorf("CELL_NOT_FOUND")
				}
			case "trace_formula_dependencies", "resolve_cross_sheet_references":
				if graph == nil {
					graph = engineering.FormulaGraph(w, hooks.Names)
				}
				found := []engineering.FormulaEvidence{}
				total := 0
				for _, f := range graph {
					if f.Cell.SheetID == action.Sheet && (action.Cell == "" || f.Cell.Address == action.Cell) {
						total++
						if len(found) < 32 {
							found = append(found, f)
						}
					}
				}
				result = map[string]any{"formulas": found, "total": total, "truncated": total > len(found)}
			case "propose_calculation_rule":
				if action.Rule == nil {
					err = fmt.Errorf("RULE_REQUIRED")
				} else if hooks.ProposeRule == nil {
					err = fmt.Errorf("RULE_REGISTRY_UNAVAILABLE")
				} else {
					result, err = hooks.ProposeRule(ctx, *action.Rule)
				}
			case "run_calculation":
				if hooks.Calculate == nil {
					err = fmt.Errorf("CALC_PROVIDER_UNAVAILABLE")
				} else {
					result, err = hooks.Calculate(ctx, action.RuleRef, action.Cells, action.Inputs)
				}
			case "classify_information", "map_semantic_fields", "validate_model", "analyze_validation_issues", "propose_engineering_model":
				if action.Proposal == nil {
					err = fmt.Errorf("PROPOSAL_REQUIRED")
					break
				}
				err = engineering.CheckProposal(w, *action.Proposal)
				if err == nil && action.Action == "propose_engineering_model" {
					// A commit is bounded to one inspected sheet. Independent sheets survive retries.
					if action.Sheet == "" && len(action.Proposal.Regions) > 0 {
						action.Sheet = action.Proposal.Regions[0].Sheet
					}
					if !inspected[action.Sheet] {
						err = fmt.Errorf("INSPECT_SHEET_BEFORE_PROPOSING")
					}
					for _, r := range action.Proposal.Regions {
						if r.Sheet != action.Sheet {
							err = fmt.Errorf("PROPOSE_ONE_SHEET_AT_A_TIME")
						}
					}
					if err == nil {
						err = engineering.CheckSheetCoverage(w, *action.Proposal, action.Sheet)
					}
				}
				if err == nil {
					candidate := mergeEngineeringProposal(proposal, *action.Proposal, action.Sheet)
					result = map[string]any{"schema_valid": true, "regions": len(candidate.Regions), "formal_write": false}
					if action.Action == "validate_model" || action.Action == "analyze_validation_issues" {
						x, extractErr := engineering.ExtractPhysical(w, candidate, "analysis", "source", "revision", hooks.Attributes...)
						if extractErr != nil {
							err = extractErr
						} else {
							result = map[string]any{"issues": x.Issues, "equipment": len(x.Model.Equipment), "facilities": len(x.Model.Facilities), "placements": len(x.Model.Placements), "mapped_cells": x.MappedCells, "total_cells": x.TotalCells}
						}
					}
					if action.Action == "propose_engineering_model" {
						proposal = candidate
						completed[action.Sheet] = true
						result = map[string]any{"sheet_saved": action.Sheet, "regions": len(action.Proposal.Regions), "completed_sheets": len(completed), "formal_write": false}
						history = nil
					}
				}
			default:
				err = fmt.Errorf("UNKNOWN_ENGINEERING_TOOL")
			}
		}
		state, detail := "succeeded", "已读取脱敏来源IR"
		if err != nil {
			state = "rejected"
			detail = err.Error()
			feedback := map[string]any{"error": detail}
			if action.Proposal != nil {
				feedback["binding_errors"] = engineeringBindingErrors(*action.Proposal, hooks.Attributes)
				feedback["next"] = "Correct all listed bindings together. Omit unsupported columns; they remain in the source. Do not rename their fields or change the region kind just to pass validation."
			}
			if action.Action == "inspect_formula" {
				refs := []string{}
				if sheet, ok := ir.AnalyzeSheet(action.Sheet); ok {
					for _, cell := range sheet.Cells {
						if !cell.Sensitive && cell.Formula != "" && len(refs) < 16 {
							refs = append(refs, cell.Ref)
						}
					}
				}
				feedback["available_formula_cells"] = refs
				feedback["next"] = "Use an actual formula cell address from the source preview or available_formula_cells; do not guess a row beneath the header."
			}
			result = feedback
		}
		if action.Action == "propose_engineering_model" && err == nil {
			detail = fmt.Sprintf("已持久保存 %s 的区域理解（%d/%d Sheet）；未发布正式数据", action.Sheet, len(completed), len(w.Sheets))
		}
		if e := record(action.Action, state, result); e != nil {
			return proposal, e
		}
		if e := trace(action.Action, state, detail); e != nil {
			return proposal, e
		}
		history = boundedEngineeringHistory(append(history, map[string]any{"action": action, "result": result}))
		if e := persist(); e != nil {
			return proposal, e
		}
		if err != nil {
			queued = nil // Give the model a correction turn before executing dependent calls.
			key := action.Action + "/" + action.Sheet + "/" + err.Error()
			rejections[key]++
			if rejections[key] >= 3 {
				return proposal, errors.New("ENGINEERING_AGENT_NO_PROGRESS")
			}
		}
	}
	return proposal, fmt.Errorf("ENGINEERING_AGENT_TURN_LIMIT")
}

// Retry transient transport failures and ask the model to correct invalid output.
// Neither retry path relaxes the schema or source checks.
func (p *Provider) engineeringCompletion(ctx context.Context, prompt string, input any, functions []any, trace func(string, string, string) error, artifacts ...func(context.Context, string, string, any) error) ([]byte, error) {
	corrections := 0
	for attempt := 0; ; attempt++ {
		raw, err := p.complete(ctx, prompt, input, functions)
		if err == nil {
			return raw, nil
		}
		if (err.Error() == "INVALID_JSON" || err.Error() == "PROVIDER_OUTPUT_TRUNCATED" || err.Error() == "SCHEMA_VALIDATION_FAILED") && corrections < 2 {
			if len(artifacts) > 0 && artifacts[0] != nil {
				if e := artifacts[0](ctx, "provider_output", "rejected", map[string]any{"error": err.Error(), "response": string(p.Raw)}); e != nil {
					return nil, e
				}
			}
			corrections++
			if e := trace("provider_output", "retrying", err.Error()+"；要求模型重新输出精简、合法的工具参数"); e != nil {
				return nil, e
			}
			prompt += "\nYour previous output was rejected: " + err.Error() + ". Call exactly one supplied function with a complete valid JSON object as arguments. Keep it concise; bind columns instead of listing source rows. Do not output commentary or markdown."
			continue
		}
		var upstream *ProviderError
		if !errors.As(err, &upstream) || !upstream.Temporary() || attempt >= 3 {
			_ = trace("provider_request", "failed", err.Error())
			return nil, err
		}
		delay := upstream.RetryAfter
		if delay <= 0 {
			delay = time.Duration(1<<attempt) * 5 * time.Second
		}
		if e := trace("provider_retry", "waiting", fmt.Sprintf("%s；等待 %.0f 秒后重试当前工具调用，已保存结果保留", upstream.Error(), delay.Seconds())); e != nil {
			return nil, e
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
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
func engineeringSheetPreview(s bom.Sheet, regions []bom.Region) any {
	regions = engineeringSourceSpans(regions)
	rows := map[int]bool{}
	last := 0
	for _, r := range regions {
		if r.Start > last+1 || last == 0 {
			for row := r.Start; row <= min(r.End, r.Start+5); row++ {
				rows[row] = true
			}
		}
		last = r.End
	}
	if len(regions) > 0 {
		rows[regions[len(regions)-1].End] = true
	}
	samples := []map[string]any{}
	hints := []map[string]any{}
	for _, c := range s.Cells {
		if rows[c.Row] && !c.Sensitive && len(samples) < 240 {
			samples = append(samples, map[string]any{"cell": c.Ref, "row": c.Row, "column": c.Column, "value": c.Value, "formula": c.Formula})
			if fields := engineering.HeaderFields(c.Value); len(fields) > 0 {
				hints = append(hints, map[string]any{"cell": c.Ref, "header": c.Value, "candidate_fields": fields})
			}
		}
	}
	full := engineering.Bounds{StartRow: 1, StartColumn: 1}
	for _, cell := range s.Cells {
		full.EndRow = max(full.EndRow, cell.Row)
		full.EndColumn = max(full.EndColumn, cell.Column)
	}
	return map[string]any{"sheet": s.Name, "full_sheet_bounds": full, "merges": s.Merges, "regions": regions, "regions_are_occupied_spans_not_semantic_tables": true, "sample_cells": samples, "header_hints": hints, "sample_is_not_full_data": true}
}

func boundedEngineeringHistory(history []any) []any {
	if len(history) > 6 {
		history = history[len(history)-6:]
	}
	for len(history) > 1 {
		raw, _ := json.Marshal(history)
		if len(raw) <= 64<<10 {
			break
		}
		history = history[1:]
	}
	if len(history) == 1 {
		raw, _ := json.Marshal(history[0])
		if len(raw) > 64<<10 {
			if item, ok := history[0].(map[string]any); ok {
				history = []any{map[string]any{"action": item["action"], "result": map[string]any{"error": "TOOL_RESULT_TOO_LARGE", "bytes": len(raw), "next": "Full result preserved as artifact; request a smaller source range."}}}
			}
		}
	}
	return history
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
	return false
}

// Match the existing BOM Agent's bounded-page behavior. Requested bounds are
// never mistaken for the extent actually returned to the model.
func engineeringReadRange(w bom.Workbook, sheet string, requested engineering.Bounds) (any, error) {
	if requested.StartRow < 1 || requested.StartColumn < 1 || requested.EndRow < requested.StartRow || requested.EndColumn < requested.StartColumn {
		return nil, fmt.Errorf("INVALID_RANGE: end_row and end_column are absolute coordinates, not counts; require end_row >= start_row and end_column >= start_column")
	}
	returned := requested
	returned.EndRow = min(requested.EndRow, requested.StartRow+31)
	returned.EndColumn = min(requested.EndColumn, requested.StartColumn+63)
	cells, err := engineering.SourceRange(w, sheet, returned)
	if err != nil {
		return nil, err
	}
	return map[string]any{"requested_bounds": requested, "returned_bounds": returned, "truncated": returned != requested, "cells": engineeringCellPreview(cells), "instruction": "This is a source sample. For the next page use absolute coordinates after returned_bounds; engineering mappings still use the full observed table bounds."}, nil
}

// Return every invalid column in a rejected proposal, not only the first error.
// This is correction feedback, never an automatically accepted/repaired mapping.
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
				reason = "unsupported field: omit this binding"
			} else if seen[binding.Field] {
				reason = "duplicate field: each object has one binding per field"
			}
			seen[binding.Field] = true
			if reason != "" {
				issues = append(issues, map[string]any{"region": index, "kind": region.Kind, "field": binding.Field, "column": binding.Column, "reason": reason})
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
func engineeringCellPreview(cells []bom.Cell) []map[string]any {
	out := make([]map[string]any, 0, len(cells))
	for _, c := range cells {
		v := map[string]any{"cell": c.Ref, "row": c.Row, "column": c.Column, "value": c.Value}
		if c.Formula != "" {
			v["formula"] = c.Formula
		}
		out = append(out, v)
	}
	return out
}

// Undo the IR paging boundary before showing structural spans to the Agent.
// Blank rows still separate spans; semantic tables remain the Agent's decision.
func engineeringSourceSpans(chunks []bom.Region) []bom.Region {
	spans := []bom.Region{}
	for _, r := range chunks {
		if len(spans) > 0 {
			last := &spans[len(spans)-1]
			if last.Sheet == r.Sheet && r.Start <= last.End+1 {
				last.End = max(last.End, r.End)
				last.FirstColumn = min(last.FirstColumn, r.FirstColumn)
				last.LastColumn = max(last.LastColumn, r.LastColumn)
				last.Cells += r.Cells
				a, _ := excelize.CoordinatesToCellName(last.FirstColumn, last.Start)
				z, _ := excelize.CoordinatesToCellName(last.LastColumn, last.End)
				last.Range = a + ":" + z
				continue
			}
		}
		spans = append(spans, r)
	}
	return spans
}

// Scope mapping vocabulary to the Agent's own most recent domain proposal.
// The classification tool remains available so new evidence can change it.
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
		for _, name := range []string{"Column", "Constant", "CellBinding"} {
			defs[name].(map[string]any)["properties"].(map[string]any)["field"].(map[string]any)["enum"] = fields
		}
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
