package ai

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"sort"
	"strings"
)

// Plans describe source coordinates, never generated inventory rows.
type KickoffBinding struct {
	Field          string         `json:"field"`
	Relationship   string         `json:"relationship,omitempty"`
	Classification string         `json:"classification,omitempty"`
	Normalize      string         `json:"normalize,omitempty"`
	Column         int            `json:"column,omitempty"`
	Row            int            `json:"row,omitempty"`
	OffsetRow      int            `json:"offset_row,omitempty"`
	OffsetColumn   int            `json:"offset_column,omitempty"`
	Lookup         *KickoffLookup `json:"lookup,omitempty"`
}
type KickoffLookup struct {
	Sheet       string `json:"sheet"`
	KeyColumn   int    `json:"key_column"`
	ValueColumn int    `json:"value_column"`
	StartRow    int    `json:"start_row"`
	EndRow      int    `json:"end_row"`
}
type KickoffRegion struct {
	ID     string             `json:"id"`
	Sheet  string             `json:"sheet"`
	Bounds engineering.Bounds `json:"bounds"`
	Steps  []int              `json:"steps"`
	Reason string             `json:"reason"`
}
type KickoffMapping struct {
	Region    string             `json:"region"`
	Table     string             `json:"table_id"`
	Bounds    engineering.Bounds `json:"bounds"`
	Layout    string             `json:"layout"`
	KeyColumn int                `json:"key_column,omitempty"`
	Bindings  []KickoffBinding   `json:"bindings"`
}
type KickoffTaskResult struct {
	Failure  string           `json:"failure,omitempty"`
	Mappings []KickoffMapping `json:"mappings"`
	Missing  []string         `json:"missing_inputs"`
}
type KickoffCoverage struct {
	Region string `json:"region"`
	Step   int    `json:"step"`
	State  string `json:"state"`
	Rows   int    `json:"rows"`
	Reason string `json:"reason"`
}
type KickoffVerifiedCell struct {
	Value string `json:"value"`
	RunID string `json:"run_id"`
}
type KickoffCheckpoint struct {
	FormulaVerificationAttempted bool                           `json:"formula_verification_attempted,omitempty"`
	Relationships                *KickoffRelationshipPlan       `json:"relationships,omitempty"`
	Missing                      []string                       `json:"missing_inputs,omitempty"`
	Verified                     map[string]KickoffVerifiedCell `json:"verified_cells,omitempty"`
	Fingerprint                  string                         `json:"fingerprint"`
	Regions                      []KickoffRegion                `json:"regions"`
	Tasks                        map[int]KickoffTaskResult      `json:"tasks"`
	Coverage                     []KickoffCoverage              `json:"coverage"`
}
type KickoffDraft struct {
	TableID string                     `json:"table_id"`
	CSV     string                     `json:"csv"`
	Issues  []engineering.InputProblem `json:"issues"`
}
type KickoffResult struct {
	FlowVersion    string                  `json:"flow_version,omitempty"`
	Candidates     []KickoffAgentCandidate `json:"candidates,omitempty"`
	Regions        []KickoffRegion         `json:"regions,omitempty"`
	SourceCoverage []any                   `json:"source_coverage,omitempty"`
	Drafts         []KickoffDraft          `json:"drafts"`
	Missing        []string                `json:"missing_inputs"`
	Coverage       []KickoffCoverage       `json:"coverage"`
}

func kickoffDecode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("TRAILING_JSON")
	}
	return nil
}
func kickoffSheet(w bom.Workbook, name string) (bom.Sheet, bool) {
	for _, s := range w.Sheets {
		if s.Name == name {
			return s, true
		}
	}
	return bom.Sheet{}, false
}
func kickoffBounds(w bom.Workbook, sheet string, b engineering.Bounds) bool {
	s, ok := kickoffSheet(w, sheet)
	if !ok || b.StartRow < 1 || b.StartColumn < 1 || b.EndRow < b.StartRow || b.EndColumn < b.StartColumn {
		return false
	}
	maxRow, maxCol := 0, 0
	for _, c := range s.Cells {
		maxRow = max(maxRow, c.Row)
		maxCol = max(maxCol, c.Column)
	}
	return b.EndRow <= maxRow && b.EndColumn <= maxCol
}
func kickoffTable(id string) (engineering.InputTable, bool) {
	for _, t := range append(engineering.StepInputCatalog().Tables, KickoffAgentCatalog().Tables...) {
		if t.ID == id {
			return t, true
		}
	}
	return engineering.InputTable{}, false
}

func (p *Provider) ExtractKickoff(ctx context.Context, safe, original bom.Workbook, trace func(string, string, string) error, hooks EngineeringToolHooks, options ...KickoffAgentOptions) (KickoffResult, error) {
	result := KickoffResult{Drafts: []KickoffDraft{}, Missing: []string{}, Coverage: []KickoffCoverage{}}
	catalog := engineering.StepInputCatalog()
	promptPath := "prompts/kickoff-v1.txt"
	selected := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true}
	extended := len(options) > 0
	if extended {
		catalog = KickoffAgentCatalog()
		result.FlowVersion = "termination-v1"
		promptPath = "prompts/kickoff-domains-v1.txt"
		selected = map[int]bool{}
		for _, domain := range options[0].Domains {
			selected[KickoffAgentStep(domain)] = true
		}
	}
	lastStep := 5
	if extended {
		lastStep = 6
	}
	promptSource, _ := prompts.ReadFile(promptPath)
	relationPrompt, _ := prompts.ReadFile("prompts/kickoff-relations-v2.txt")
	confirmedLengthUnit := ""
	if extended {
		confirmedLengthUnit = options[0].CableLengthUnit
		if confirmedLengthUnit == "" {
			confirmedLengthUnit = "m"
		}
	}
	fingerprint := engineering.Digest([]any{confirmedLengthUnit, hooks.SourceHash, p.ConfigID, p.ConfigRevision, p.Model, string(promptSource), string(relationPrompt), catalog.Version})
	cp := KickoffCheckpoint{Fingerprint: fingerprint, Regions: []KickoffRegion{}, Tasks: map[int]KickoffTaskResult{}, Coverage: []KickoffCoverage{}}
	if hooks.Resume != nil && hooks.Resume.Version == engineeringCheckpointVersion && hooks.Resume.Kickoff != nil && hooks.Resume.Kickoff.Fingerprint == fingerprint {
		cp = *hooks.Resume.Kickoff
	}
	var priorRelationshipPlan *KickoffRelationshipPlan
	if hooks.Resume != nil && hooks.Resume.Kickoff != nil && hooks.Resume.Kickoff.Fingerprint != fingerprint {
		// Server resume is scoped to the same immutable source and revision.
		// Reuse only inspected region bounds as hints; re-evaluate identities.
		for _, region := range hooks.Resume.Kickoff.Regions {
			if kickoffBounds(original, region.Sheet, region.Bounds) {
				cp.Regions = append(cp.Regions, region)
			}
		}
		priorRelationshipPlan = hooks.Resume.Kickoff.Relationships
	}
	if cp.Verified == nil {
		cp.Verified = map[string]KickoffVerifiedCell{}
	}
	if cp.Tasks == nil {
		cp.Tasks = map[int]KickoffTaskResult{}
	}
	persist := func() error {
		if hooks.Checkpoint != nil {
			return hooks.Checkpoint(ctx, EngineeringCheckpoint{Version: engineeringCheckpointVersion, Kickoff: &cp})
		}
		return nil
	}
	artifact := func(tool string, value any) error {
		if hooks.Artifact != nil {
			return hooks.Artifact(ctx, tool, "succeeded", value)
		}
		return nil
	}
	original = kickoffVerifiedWorkbook(original, cp.Verified, false)
	safe = kickoffVerifiedWorkbook(safe, cp.Verified, true)
	var graph []engineering.FormulaEvidence
	testedRelations := map[string]bool{}
	testedGraphs := map[string]bool{}
	// One semantic correction, with a bounded number of read tools, per task.
	ask := func(task string, input map[string]any, decode func([]byte) error) error {
		history := []any{}
		correction := false
		toolLimit := 6
		if task == "reason_relationships" {
			toolLimit = 18
		}
		for turn := 0; turn < toolLimit+3; turn++ {
			input["confirmed_cable_length_unit"] = confirmedLengthUnit
			input["history"] = boundedEngineeringHistory(history)
			input["remaining_tool_calls"] = max(0, toolLimit-turn)
			input["must_finish"] = turn >= toolLimit
			prompt, _ := prompts.ReadFile(promptPath)
			callPrompt := string(prompt) + "\nTask: " + task
			if task == "reason_relationships" {
				callPrompt = string(relationPrompt) + "\nTask: " + task
			}
			if turn >= toolLimit {
				callPrompt += "\nNo read/tool calls remain. Return the final route or extraction plan now, with explicit missing_inputs for unresolved facts. Do not request another tool."
			}
			callPrompt += "\nUse the supplied functions for every tool action. Finish by calling submit_result with the final task object in result. The workbook actions are real available tools, not examples of unavailable capabilities."
			raw, err := p.engineeringCompletion(ctx, callPrompt, input, kickoffFunctions(turn >= toolLimit), trace, hooks.Artifact)
			if err != nil {
				return err
			}
			var submitted struct {
				Action string          `json:"action"`
				Result json.RawMessage `json:"result"`
			}
			if json.Unmarshal(raw, &submitted) == nil && submitted.Action == "submit_result" {
				raw = submitted.Result
			}
			var resolved struct {
				Model string `json:"model"`
			}
			_ = json.Unmarshal(p.Raw, &resolved)
			if err = artifact("kickoff_provider_usage", map[string]any{"task": task, "requested_model": p.Model, "resolved_model": resolved.Model, "provider_revision": p.ConfigRevision, "input_tokens": p.InputTokens, "output_tokens": p.OutputTokens, "total_input_tokens": p.TotalInputTokens, "total_output_tokens": p.TotalOutputTokens}); err != nil {
				return err
			}
			if err = artifact("kickoff_"+task+"_response", json.RawMessage(raw)); err != nil {
				return err
			}
			var action struct {
				Action    string                        `json:"action"`
				Ranges    []KickoffGraphRange           `json:"ranges,omitempty"`
				Relations []KickoffRelation             `json:"relations,omitempty"`
				Offset    int                           `json:"offset,omitempty"`
				Sheet     string                        `json:"sheet"`
				Bounds    engineering.Bounds            `json:"bounds"`
				Cell      string                        `json:"cell"`
				Rule      *engineering.CalculationRule  `json:"rule,omitempty"`
				RuleRef   *engineering.VersionReference `json:"rule_ref,omitempty"`
				Cells     []engineering.FormulaCell     `json:"cells,omitempty"`
				Inputs    []engineering.NamedValue      `json:"inputs,omitempty"`
			}
			_ = json.Unmarshal(raw, &action)
			if turn >= toolLimit && action.Action != "" {
				history = append(history, map[string]any{"error": "FINAL_PLAN_REQUIRED", "instruction": "Return the final plan, not another tool request."})
				continue
			}
			if action.Action == "inspect_connection_graph" {
				value, graphErr := inspectKickoffConnectionGraph(original, safe, action.Ranges, action.Offset)
				if graphErr == nil {
					for _, gr := range action.Ranges {
						for _, region := range cp.Regions {
							if gr.Region == region.ID && gr.Sheet == region.Sheet && gr.StartRow >= region.Bounds.StartRow && gr.EndRow == region.Bounds.EndRow {
								testedGraphs[gr.Region] = true
							}
						}
					}
				}
				state := "succeeded"
				if graphErr != nil {
					value = map[string]string{"error": graphErr.Error()}
					state = "failed"
				}
				history = append(history, map[string]any{"action": action, "result": value})
				if e := artifact("kickoff_connection_graph", value); e != nil {
					return e
				}
				if e := trace(action.Action, state, task+" · 全量端接关系与邻接证据"); e != nil {
					return e
				}
				continue
			}
			if action.Action == "verify_workbook_formulas" {
				var verificationErr error
				if !cp.FormulaVerificationAttempted {
					if hooks.VerifyWorkbook == nil {
						verificationErr = errors.New("CALCULATION_NOT_AVAILABLE")
					} else {
						var verified map[string]KickoffVerifiedCell
						verified, verificationErr = hooks.VerifyWorkbook(ctx)
						if verificationErr == nil {
							for key, value := range verified {
								cp.Verified[key] = value
							}
						}
					}
					if verificationErr == nil {
						cp.FormulaVerificationAttempted = true
					}
					if err := persist(); err != nil {
						return err
					}
				}
				original = kickoffVerifiedWorkbook(original, cp.Verified, false)
				safe = kickoffVerifiedWorkbook(safe, cp.Verified, true)
				value := map[string]any{"verified_cells": len(cp.Verified), "original_values_returned": false, "instruction": "Re-evaluate affected relationship hypotheses. Text and numeric results are available locally; use source coordinates, not cached raw identity text."}
				state := "succeeded"
				if verificationErr != nil {
					state = "failed"
					value["error"] = verificationErr.Error()
				}
				history = append(history, map[string]any{"action": action, "result": value})
				if err := artifact("kickoff_formula_verification", value); err != nil {
					return err
				}
				if err := trace("verify_workbook_formulas", state, fmt.Sprintf("%d个公式单元格独立核算；文本值仅在本地使用", len(cp.Verified))); err != nil {
					return err
				}
				continue
			}
			if action.Action == "find_related_records" || action.Action == "evaluate_relationships" {
				var value any
				if action.Action == "find_related_records" {
					value, err = kickoffRelatedRecords(original, safe, KickoffCellRef{action.Sheet, action.Cell}, action.Offset)
				} else {
					var reports []kickoffRelationReport
					_, reports, err = evaluateKickoffRelations(original, KickoffRelationshipPlan{Relations: action.Relations})
					if err == nil {
						value = reports
						for _, relation := range action.Relations {
							testedRelations[engineering.Digest(relation)] = true
						}
					}
				}
				if err != nil {
					value = map[string]string{"error": err.Error()}
				}
				history = append(history, map[string]any{"action": action, "result": value})
				if e := artifact("kickoff_relationship_tool", history[len(history)-1]); e != nil {
					return e
				}
				state := "succeeded"
				if err != nil {
					state = "failed"
				}
				if e := trace(action.Action, state, task+" · 跨表证据核对结果已记录"); e != nil {
					return e
				}
				continue
			}
			if action.Action == "trace_formula_dependencies" {
				if graph == nil {
					graph = engineering.FormulaGraph(safe, nil)
				}
				var value any = map[string]string{"error": "FORMULA_NOT_FOUND"}
				for _, f := range graph {
					if f.Cell.SheetID == action.Sheet && f.Cell.Address == action.Cell {
						value = f
						break
					}
				}
				history = append(history, map[string]any{"action": action, "result": value})
				if err = artifact("kickoff_formula_dependencies", history[len(history)-1]); err != nil {
					return err
				}
				continue
			}
			if action.Action == "run_calculation" || action.Action == "propose_calculation_rule" {
				var value any
				if action.Action == "propose_calculation_rule" {
					if task != "extract_step_3" || hooks.ProposeRule == nil || action.Rule == nil {
						err = errors.New("RULE_CANDIDATE_NOT_ALLOWED")
					} else {
						value, err = hooks.ProposeRule(ctx, *action.Rule)
					}
				} else {
					if hooks.Calculate == nil || len(action.Cells) > 32 {
						err = errors.New("CALCULATION_NOT_AVAILABLE")
					} else {
						var page engineering.CalculationPage
						page, err = hooks.Calculate(ctx, action.RuleRef, action.Cells, action.Inputs)
						if err == nil {
							value = page
							for _, f := range page.Run.Formulas {
								if f.ErrorCode == nil && f.Compatibility == "compatible" && f.RecalculatedValue != nil && f.RecalculatedValue.Decimal != nil {
									cp.Verified[f.Cell.SheetID+"!"+f.Cell.Address] = KickoffVerifiedCell{Value: *f.RecalculatedValue.Decimal, RunID: page.Run.ID}
								}
							}
							if err = persist(); err != nil {
								return err
							}
						}
					}
				}
				if err != nil {
					value = map[string]string{"error": err.Error()}
				}
				history = append(history, map[string]any{"action": action, "result": value})
				if err = artifact("kickoff_calculation_tool", history[len(history)-1]); err != nil {
					return err
				}
				continue
			}
			if action.Action == "read_range" || action.Action == "inspect_region" || action.Action == "inspect_sheet" || action.Action == "inspect_formula" {
				var value any
				if action.Action == "inspect_sheet" {
					if s, ok := kickoffSheet(safe, action.Sheet); ok {
						value = kickoffPreview(s)
					} else {
						err = errors.New("UNKNOWN_SHEET")
					}
				} else if action.Action == "inspect_formula" {
					s, ok := kickoffSheet(safe, action.Sheet)
					if !ok {
						err = errors.New("UNKNOWN_SHEET")
					} else {
						for _, c := range s.Cells {
							if c.Ref == action.Cell {
								value = engineeringCellPreview([]bom.Cell{c})
								break
							}
						}
						if value == nil {
							err = errors.New("UNKNOWN_CELL")
						}
					}
				} else {
					value, err = engineeringReadRange(safe, action.Sheet, action.Bounds)
				}
				if err != nil {
					value = map[string]string{"error": err.Error()}
				}
				history = append(history, map[string]any{"action": action, "result": value})
				if err = artifact("kickoff_tool", history[len(history)-1]); err != nil {
					return err
				}
				if err = trace(action.Action, "succeeded", task+" · "+action.Sheet); err != nil {
					return err
				}
				continue
			}
			if err = decode(raw); err == nil {
				return nil
			}
			if traceErr := trace("kickoff_plan_validation", "failed", task+" · "+strings.SplitN(err.Error(), ":", 2)[0]); traceErr != nil {
				return traceErr
			}
			if correction {
				return err
			}
			correction = true
			input["validation_error"] = err.Error()
			history = append(history, map[string]any{"error": err.Error(), "instruction": "Correct only the rejected mapping; no invented source values."})
		}
		return errors.New("KICKOFF_TASK_TOOL_LIMIT")
	}
	if len(cp.Regions) == 0 {
		sheets := []any{}
		for _, s := range safe.Sheets {
			preview := kickoffPreview(s)
			sheets = append(sheets, preview)
		}
		err := ask("route", map[string]any{"sheets": sheets, "steps": catalog}, func(raw []byte) error {
			var plan struct {
				Regions []KickoffRegion `json:"regions"`
				Missing []string        `json:"missing_inputs,omitempty"`
			}
			if err := kickoffDecode(raw, &plan); err != nil {
				return err
			}
			seen := map[string]bool{}
			covered := map[string]bool{}
			for _, r := range plan.Regions {
				if r.ID == "" || seen[r.ID] || !kickoffBounds(safe, r.Sheet, r.Bounds) || strings.TrimSpace(r.Reason) == "" {
					return errors.New("INVALID_ROUTE_REGION")
				}
				seen[r.ID] = true
				covered[r.Sheet] = true
				for _, step := range r.Steps {
					if step < 1 || step > lastStep {
						return errors.New("INVALID_STEP")
					}
				}
			}
			for _, s := range safe.Sheets {
				if len(s.Cells) > 0 && !covered[s.Name] {
					return fmt.Errorf("UNCLASSIFIED_SHEET: %s", s.Name)
				}
			}
			cp.Regions = plan.Regions
			cp.Missing = plan.Missing
			return nil
		})
		if err != nil {
			if !extended || kickoffAgentFatal(err) {
				return result, err
			}
			cp.Missing = append(cp.Missing, "语义区域路由未完成；保留Parser结构区域供各域独立提取")
			for _, r := range bom.NewWorkbookIR(safe).InspectWorkbook() {
				steps := []int{}
				for step := 1; step <= lastStep; step++ {
					if selected[step] {
						steps = append(steps, step)
					}
				}
				cp.Regions = append(cp.Regions, KickoffRegion{ID: r.ID, Sheet: r.Sheet, Bounds: engineering.Bounds{StartRow: r.Start, EndRow: r.End, StartColumn: r.FirstColumn, EndColumn: r.LastColumn}, Steps: steps, Reason: "Parser结构区域；语义分类未完成"})
			}
		}
		if err = artifact("kickoff_import_plan", cp.Regions); err != nil {
			return result, err
		}
		if err = persist(); err != nil {
			return result, err
		}
	}
	if err := artifact("kickoff_manifest", map[string]any{"fingerprint": fingerprint, "source_hash": hooks.SourceHash, "model_id": p.Model, "provider_id": p.ConfigID, "provider_revision": p.ConfigRevision, "checkpoint_version": engineeringCheckpointVersion}); err != nil {
		return result, err
	}
	// Semantic identity reasoning precedes field extraction. Failed reasoning
	// cannot silently fall back to the old header-only equipment pipeline.
	needRelationships := extended && (selected[1] || selected[2] || selected[3])
	if needRelationships && cp.Relationships == nil {
		var plan KickoffRelationshipPlan
		err := ask("reason_relationships", map[string]any{"regions": cp.Regions, "region_samples": kickoffSamples(safe, cp.Regions), "prior_plan_to_recheck": priorRelationshipPlan, "objective": "判断真实设备，跨表佐证双端连接，再关联机柜/U位和线缆；字段映射不是本任务的结论。"}, func(raw []byte) error {
			if err := kickoffDecode(raw, &plan); err != nil {
				return err
			}
			if len(plan.Entities) == 0 {
				return errors.New("DEVICE_ASSESSMENT_REQUIRED")
			}
			cells := kickoffCellIndex(original)
			seen := map[string]bool{}
			for _, entity := range plan.Entities {
				found := false
				for _, r := range cp.Regions {
					if entity.Region == r.ID {
						found = true
					}
				}
				if !found || seen[entity.Region] || entity.Reason == "" || !kickoffEvidenceExists(cells, entity.Evidence) {
					return errors.New("ENTITY_ASSESSMENT_EVIDENCE_REQUIRED")
				}
				seen[entity.Region] = true
				switch entity.Kind {
				case "device", "component", "material", "aggregate", "unknown", "connection", "placement":
					if entity.Kind == "connection" && !testedGraphs[entity.Region] {
						return fmt.Errorf("CONNECTION_GRAPH_REQUIRED: inspect_connection_graph over all rows of %s before finalizing", entity.Region)
					}
				default:
					return errors.New("ENTITY_ASSESSMENT_KIND_INVALID")
				}
			}
			if err := validateKickoffRelations(original, plan); err != nil {
				return err
			}
			for _, relation := range plan.Relations {
				if !testedRelations[engineering.Digest(relation)] {
					return fmt.Errorf("RELATION_NOT_CHECKED: %s; call evaluate_relationships before finalizing", relation.ID)
				}
			}
			if len(plan.Relations) == 0 && len(plan.Missing) == 0 && len(plan.Entities) == 0 {
				return errors.New("RELATION_OR_EXPLICIT_GAP_REQUIRED")
			}
			return nil
		})
		if err != nil {
			return result, fmt.Errorf("RELATIONSHIP_REASONING_INCOMPLETE: %w", err)
		}
		for _, entity := range plan.Entities {
			if err := trace("entity_assessment", "succeeded", entity.Region+" · "+entity.Kind+" · "+entity.Reason); err != nil {
				return result, err
			}
		}
		for _, relation := range plan.Relations {
			if err := trace("relationship_hypothesis", "succeeded", relation.ID+" · "+relation.Reason); err != nil {
				return result, err
			}
		}
		cp.Relationships = &plan
		if err := artifact("kickoff_relationship_plan", plan); err != nil {
			return result, err
		}
		if err := persist(); err != nil {
			return result, err
		}
		if err := trace("reason_relationships", "succeeded", fmt.Sprintf("%d项对象性质判断，%d项已检验跨表关系，%d项待核对", len(plan.Entities), len(plan.Relations), len(plan.Missing))); err != nil {
			return result, err
		}
	}
	relationshipIndex := kickoffRelationIndex{}
	if cp.Relationships != nil {
		var reports []kickoffRelationReport
		var err error
		relationshipIndex, reports, err = evaluateKickoffRelations(original, *cp.Relationships)
		if err != nil {
			return result, err
		}
		result.Missing = append(result.Missing, cp.Relationships.Missing...)
		for _, r := range reports {
			if r.Missing+r.Ambiguous > 0 {
				result.Missing = append(result.Missing, fmt.Sprintf("关系%s：%d条来源，%d条关联，%d条未匹配，%d条冲突；未匹配属性保留缺口；已明确端接身份不因此丢弃。", r.ID, r.Total, r.Matched, r.Missing, r.Ambiguous))
			}
		}
		if err := artifact("kickoff_relationship_validation", reports); err != nil {
			return result, err
		}
	}
	var unavailable error
	for step := 1; step <= lastStep; step++ {
		if !selected[step] {
			continue
		}
		if cached, ok := cp.Tasks[step]; ok && cached.Failure == "" {
			continue
		}
		regions := []KickoffRegion{}
		for _, r := range cp.Regions {
			for _, s := range r.Steps {
				if s == step {
					regions = append(regions, r)
					break
				}
			}
		}
		task := KickoffTaskResult{Mappings: []KickoffMapping{}, Missing: []string{}}
		if len(regions) == 0 {
			if !extended {
				task.Missing = append(task.Missing, fmt.Sprintf("步骤%d：资料中未发现本步明确依据，请补充建模输入。", step))
			}
		} else {
			tables := []engineering.InputTable{}
			for _, t := range catalog.Tables {
				if t.Step == step {
					tables = append(tables, t)
				}
			}
			err := unavailable
			if err == nil {
				err = ask(fmt.Sprintf("extract_step_%d", step), map[string]any{"regions": regions, "source_regions": cp.Regions, "region_samples": kickoffSamples(safe, regions), "tables": tables, "prior_tasks": kickoffTaskIndex(cp.Tasks), "relationship_plan": cp.Relationships}, func(raw []byte) error {
					var proposed KickoffTaskResult
					if err := kickoffDecode(raw, &proposed); err != nil {
						return err
					}
					if proposed.Failure != "" {
						return errors.New("INVALID_TASK_FAILURE_FIELD")
					}
					if extended {
						for _, m := range proposed.Mappings {
							if !strings.HasPrefix(m.Table, "kickoff23/") {
								return errors.New("KICKOFF23_TARGET_REQUIRED")
							}
						}
					}
					if err := validateKickoffMappings(original, regions, step, proposed); err != nil {
						return err
					}
					if cp.Relationships != nil {
						if err := validateKickoffRelationshipBindings(original, cp.Regions, *cp.Relationships, proposed); err != nil {
							return err
						}
					}
					task = proposed
					return nil
				})
			}
			if err != nil {
				if ctx.Err() != nil {
					return result, ctx.Err()
				}
				var providerErr *ProviderError
				providerFailure := errors.As(err, &providerErr) || strings.HasPrefix(err.Error(), "PROVIDER_")
				if !extended && providerFailure || kickoffAgentFatal(err) && !providerFailure {
					return result, err
				}
				if providerFailure {
					unavailable = err
				}
				task.Failure = err.Error()
				task.Missing = []string{fmt.Sprintf("步骤%d：提取未完成（%s），请补充对应建模输入。", step, err.Error())}
				task.Mappings = nil
			}
		}
		cp.Tasks[step] = task
		if err := artifact(fmt.Sprintf("kickoff_step_%d", step), task); err != nil {
			return result, err
		}
		if err := persist(); err != nil {
			return result, err
		}
		state := "succeeded"
		if len(task.Missing) > 0 {
			state = "partial"
		}
		if err := trace(fmt.Sprintf("kickoff_step_%d", step), state, fmt.Sprintf("%d个提取计划，%d项待补充", len(task.Mappings), len(task.Missing))); err != nil {
			return result, err
		}
	}
	if len(cp.Verified) > 0 {
		data, _ := json.Marshal(original)
		var verified bom.Workbook
		if err := json.Unmarshal(data, &verified); err != nil {
			return result, err
		}
		for si := range verified.Sheets {
			for ci := range verified.Sheets[si].Cells {
				c := &verified.Sheets[si].Cells[ci]
				if v, ok := cp.Verified[verified.Sheets[si].Name+"!"+c.Ref]; ok && c.Formula != "" && !c.Sensitive {
					c.Value = v.Value
					c.Formula = ""
				}
			}
		}
		original = verified
		if err := artifact("kickoff_verified_formula_sources", cp.Verified); err != nil {
			return result, err
		}
	}
	if extended {
		original = kickoffAgentFilteredWorkbook(original)
	}
	if cp.Relationships != nil {
		var err error
		relationshipIndex, _, err = evaluateKickoffRelations(original, *cp.Relationships)
		if err != nil {
			return result, err
		}
	}
	rowsByTable := map[string][][]string{}
	result.Missing = append(result.Missing, cp.Missing...)
	for step := 1; step <= lastStep; step++ {
		_, recorded := cp.Tasks[step]
		if !selected[step] && (!extended || !recorded) {
			continue
		}
		task := cp.Tasks[step]
		result.Missing = append(result.Missing, task.Missing...)
		for _, mapping := range task.Mappings {
			rows, evidence, err := expandKickoffRegionWithRelationships(original, func() KickoffRegion {
				for _, r := range cp.Regions {
					if r.ID == mapping.Region {
						return r
					}
				}
				return KickoffRegion{}
			}(), mapping, relationshipIndex, confirmedLengthUnit)
			if err != nil {
				if !extended {
					return result, err
				}
				result.Missing = append(result.Missing, fmt.Sprintf("%s/%s: expansion failed", KickoffAgentDomain(step), mapping.Region))
				result.Coverage = append(result.Coverage, KickoffCoverage{Region: mapping.Region, Step: step, State: "needs_input", Reason: "EXPANSION_FAILED"})
				continue
			}
			state, reason := "extracted", "候选已展开，来源与未消费字段保留"
			if extended {
				candidate := kickoffAgentCandidate(mapping.Table, rows, evidence)
				if len(candidate.Issues) > 0 {
					state = "needs_input"
					reason = fmt.Sprintf("候选存在%d项字段或关联缺口，尚未完整识别", len(candidate.Issues))
				}
				for i := range candidate.Evidence {
					for field, cell := range candidate.Evidence[i].Fields {
						for _, binding := range mapping.Bindings {
							if binding.Field == field {
								cell.Lookup = binding.Lookup
							}
						}
						if v, ok := cp.Verified[cell.Sheet+"!"+cell.Cell]; ok {
							cell.Formula = true
							cell.CalculationRunID = v.RunID
						}
						candidate.Evidence[i].Fields[field] = cell
					}
				}
				result.Candidates = append(result.Candidates, candidate)
				// Keep exact coordinates, never cached secrets, in public artifacts.
				evidence = candidate.Evidence
			}
			rowsByTable[mapping.Table] = append(rowsByTable[mapping.Table], rows...)
			if err = artifact("kickoff_evidence", evidence); err != nil {
				return result, err
			}
			result.Coverage = append(result.Coverage, KickoffCoverage{Region: mapping.Region, Step: step, State: state, Rows: len(rows), Reason: reason})
		}
	}
	for _, r := range cp.Regions {
		covered := kickoffRegionCoverage(original, r, cp.Tasks)
		result.SourceCoverage = append(result.SourceCoverage, covered)
		if err := artifact("kickoff_region_coverage", covered); err != nil {
			return result, err
		}
		for _, step := range r.Steps {
			_, recorded := cp.Tasks[step]
			if !selected[step] && (!extended || !recorded) {
				continue
			}
			found := false
			for _, c := range result.Coverage {
				if c.Region == r.ID && c.Step == step {
					found = true
				}
			}
			if !found {
				result.Coverage = append(result.Coverage, KickoffCoverage{Region: r.ID, Step: step, State: "needs_input", Reason: "区域未形成可用固定格式映射"})
			}
		}
		if len(r.Steps) == 0 {
			result.Coverage = append(result.Coverage, KickoffCoverage{Region: r.ID, State: "not_applicable", Reason: r.Reason})
		}
	}
	// Reuse the verified endpoint identities instead of asking the model to
	// rediscover them for step 3. This does not supply missing port specs.
	projected := []any{}
	if !extended {
		projected = completeKickoffPortIdentities(rowsByTable)
	}
	if len(projected) > 0 {
		result.Missing = append(result.Missing, fmt.Sprintf("端口规格：从端接候选补齐%d个端口身份，介质、连接器和速率仍须来源证据或人工补充。", len(projected)))
		if err := artifact("kickoff_port_identity_projection", projected); err != nil {
			return result, err
		}
	}
	ids := []string{}
	for id := range rowsByTable {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if extended {
			continue
		}
		t, _ := kickoffTable(id)
		var b bytes.Buffer
		writer := csv.NewWriter(&b)
		headers := []string{}
		for _, f := range t.Fields {
			headers = append(headers, f.Label)
		}
		_ = writer.Write(headers)
		if err := writer.WriteAll(rowsByTable[id]); err != nil {
			return result, err
		}
		writer.Flush()
		preview := engineering.PreviewStepInput(engineering.InputPreviewRequest{TableID: id, CSV: b.String()})
		result.Drafts = append(result.Drafts, KickoffDraft{TableID: id, CSV: b.String(), Issues: preview.Issues})
	}
	if err := artifact("kickoff_coverage", result.Coverage); err != nil {
		return result, err
	}
	result.Regions = cp.Regions
	if unavailable != nil && len(result.Candidates) == 0 {
		return result, unavailable
	}
	return result, nil
}
func kickoffTaskIndex(tasks map[int]KickoffTaskResult) any {
	out := map[int]any{}
	for k, v := range tasks {
		out[k] = map[string]any{"mapping_count": len(v.Mappings), "missing_inputs": v.Missing}
	}
	return out
}

// KickoffAgentOptions selects domains without creating another model client or queue.
type KickoffAgentOptions struct {
	Domains         []string
	CableLengthUnit string
}

type KickoffAgentNormalizationTick struct {
	Sheet string `json:"sheet"`
	Cell  string `json:"cell"`
	Value string `json:"value"`
	U     int    `json:"u"`
}
type KickoffAgentNormalizationHeader struct {
	Sheet string `json:"sheet"`
	Cell  string `json:"cell"`
	Value string `json:"value"`
}
type KickoffAgentNormalizationEvidence struct {
	Rule            string                            `json:"rule"`
	Sheet           string                            `json:"sheet,omitempty"`
	Column          int                               `json:"column,omitempty"`
	MappingBounds   *engineering.Bounds               `json:"mapping_bounds,omitempty"`
	RegionBounds    *engineering.Bounds               `json:"region_bounds,omitempty"`
	Ticks           []KickoffAgentNormalizationTick   `json:"ticks,omitempty"`
	UnitHeaders     []KickoffAgentNormalizationHeader `json:"unit_headers,omitempty"`
	StartU          string                            `json:"start_u,omitempty"`
	HeightU         string                            `json:"height_u,omitempty"`
	NormalizedValue string                            `json:"normalized_value,omitempty"`
}
type KickoffAgentCell struct {
	RelationshipEvidence  []KickoffCellRef                   `json:"relationshipEvidence,omitempty"`
	RelationshipReason    string                             `json:"relationshipReason,omitempty"`
	Sheet                 string                             `json:"sheet"`
	Cell                  string                             `json:"cell"`
	Normalization         string                             `json:"normalization,omitempty"`
	Classification        string                             `json:"classification,omitempty"`
	Formula               bool                               `json:"formula"`
	Lookup                *KickoffLookup                     `json:"lookup,omitempty"`
	CalculationRunID      string                             `json:"calculationRunId,omitempty"`
	NormalizationEvidence *KickoffAgentNormalizationEvidence `json:"normalizationEvidence,omitempty"`
}
type KickoffAgentEvidence struct {
	Row          int                         `json:"row"`
	Region       string                      `json:"region"`
	Sheet        string                      `json:"sheet"`
	SourceRow    int                         `json:"sourceRow"`
	SourceColumn int                         `json:"sourceColumn"`
	Fields       map[string]KickoffAgentCell `json:"fields"`
	Missing      []string                    `json:"missing"`
}
type KickoffAgentCandidate struct {
	TableID     string                     `json:"tableId"`
	Domain      string                     `json:"domain"`
	Rows        []map[string]string        `json:"rows"`
	Issues      []engineering.InputProblem `json:"issues"`
	Evidence    []KickoffAgentEvidence     `json:"evidence"`
	BaseVersion int                        `json:"baseVersion"`
}

func KickoffAgentStep(domain string) int {
	return map[string]int{"equipment": 1, "facility": 2, "cabling": 3, "network": 4, "power": 5, "provisioning": 6}[domain]
}
func KickoffAgentDomain(step int) string {
	return map[int]string{1: "equipment", 2: "facility", 3: "cabling", 4: "network", 5: "power", 6: "provisioning"}[step]
}

// A namespace explicitly distinguishes the new 23-table field contract from
// identically named legacy StepInputCatalog tables. No CSV header guessing.
func KickoffAgentCatalog() engineering.InputCatalog {
	c := engineering.InputCatalog{Version: "kickoff23-agent/termination-v1", Tables: []engineering.InputTable{}}
	add := func(id, domain, fields string) {
		t := engineering.InputTable{ID: "kickoff23/" + id, Step: KickoffAgentStep(domain), Name: id, Fields: []engineering.InputField{}}
		for _, field := range strings.Split(fields, "|") {
			t.Fields = append(t.Fields, engineering.InputField{Code: field, Label: field, Kind: "text"})
		}
		c.Tables = append(c.Tables, t)
	}
	add("devices", "equipment", "device_ref|serial|label|equipment_type|model|role")
	add("ports", "equipment", "device_ref|serial|port|mac|medium|connector|rate|rate_unit|parent_port|breakout_index|role")
	add("connections", "equipment", "a_device_ref|a_serial|a_port|b_device_ref|b_serial|b_port|cable_model|cable_length_m|medium|a_optic_model|b_optic_model|a_model|b_model|a_equipment_type|b_equipment_type|a_role|b_role|a_mac|b_mac|rate|rate_unit")
	add("components", "equipment", "parent_serial|serial|label|equipment_type|model|slot")
	add("rooms", "facility", "room_id|name|site|address|frame|unit")
	add("racks", "facility", "rack_id|room_id|capacity_u|width_mm|depth_mm|x|y|orientation|frame|unit")
	add("equipment_specs", "facility", "model|height_u|width_mm|depth_mm|face|panel_ref")
	add("placements", "facility", "device_id|rack_id|start_u|height_u|face|position")
	add("management", "network", "device_id|management_ip|bmc_ip|management_port|credential_ref|method")
	add("network_planes", "network", "fabric|plane|role|architecture|profile")
	add("network_roles", "network", "device_id|fabric|plane|role")
	add("network_interfaces", "network", "device_id|port|interface|plane|vlan|vrf|parent_port")
	add("network_addresses", "network", "device_id|interface|address|asn|pool_ref|source")
	add("network_bgp", "network", "device_id|interface|peer|peer_address|local_asn|peer_asn|family|bfd_ref")
	add("network_policies", "network", "scope|profile|roce|ar|evpn|source")
	add("boot_interfaces", "provisioning", "device_id|port|mac|boot_mode|network|address_policy")
	add("install_profiles", "provisioning", "scope|architecture|image_ref|disk_policy|bios_ref|firmware_ref|credential_ref")
	add("route_nodes", "cabling", "code|room_id|x|y|z|unit")
	add("route_edges", "cabling", "code|from|to|distance_m|category|source")
	add("rack_access", "cabling", "rack_id|node|lead_m")
	add("cable_specs", "cabling", "code|medium|connector_a|connector_b|rate|length_m|optic_mode")
	add("optic_specs", "cabling", "model|port_type|medium|rate|reach_m|source")
	add("cabling_rules", "cabling", "code|version|strategy|slack_m|slack_percent|cable_spec|source")
	add("power_profiles", "power", "device_id|power_kw|basis|cooling|source")
	add("rack_capacity", "power", "rack_id|capacity_kw|cooling|cooling_kw|source")
	add("power_circuits", "power", "code|rack_id|upstream|circuit|capacity_kw|group")
	add("power_connections", "power", "device_id|psu|circuit|group|allocation")
	return c
}
func kickoffAgentCandidate(tableID string, rows [][]string, evidence any) KickoffAgentCandidate {
	t, _ := kickoffTable(tableID)
	c := KickoffAgentCandidate{TableID: strings.TrimPrefix(tableID, "kickoff23/"), Domain: KickoffAgentDomain(t.Step), Rows: []map[string]string{}, Issues: []engineering.InputProblem{}, Evidence: []KickoffAgentEvidence{}}
	for _, row := range rows {
		values := map[string]string{}
		for i, f := range t.Fields {
			if i < len(row) {
				values[f.Code] = row[i]
			}
		}
		for _, required := range map[string][]string{"devices": {"device_ref|serial"}, "ports": {"device_ref|serial", "port"}, "connections": {"a_device_ref|a_serial", "a_port", "b_device_ref|b_serial", "b_port"}, "placements": {"device_id", "rack_id"}}[c.TableID] {
			present := false
			for _, alternative := range strings.Split(required, "|") {
				present = present || strings.TrimSpace(values[alternative]) != ""
			}
			if !present {
				c.Issues = append(c.Issues, engineering.InputProblem{Row: len(c.Rows) + 1, Field: required, Code: "KICKOFF_RELATION_UNRESOLVED", Message: "关键关联字段未闭合：" + required})
			}
		}
		if c.Domain != "equipment" {
			if err := kickoff.ValidateInput(c.TableID, []map[string]string{values}); err != nil {
				c.Issues = append(c.Issues, engineering.InputProblem{Row: len(c.Rows) + 1, Code: "KICKOFF_INPUT_INVALID", Message: err.Error()})
			}
		}
		c.Rows = append(c.Rows, values)
	}
	raw, _ := json.Marshal(evidence)
	var refs []struct {
		Row          *int   `json:"row"`
		Region       string `json:"source_region"`
		Sheet        string `json:"source_sheet"`
		SourceRow    int    `json:"source_row"`
		SourceColumn int    `json:"source_column"`
		Fields       map[string]struct {
			RelationshipEvidence  []KickoffCellRef                   `json:"relationship_evidence"`
			RelationshipReason    string                             `json:"relationship_reason"`
			Sheet                 string                             `json:"sheet"`
			Cell                  string                             `json:"cell"`
			Formula               string                             `json:"formula"`
			Normalization         string                             `json:"normalization"`
			Classification        string                             `json:"classification"`
			NormalizationEvidence *KickoffAgentNormalizationEvidence `json:"normalization_evidence"`
			NormalizedValue       string                             `json:"normalized_value"`
		} `json:"fields"`
		Missing []string `json:"missing_inputs"`
	}
	_ = json.Unmarshal(raw, &refs)
	for _, ref := range refs {
		row := 0
		if ref.Row != nil {
			row = *ref.Row - 1
		}
		e := KickoffAgentEvidence{Row: row, Region: ref.Region, Sheet: ref.Sheet, SourceRow: ref.SourceRow, SourceColumn: ref.SourceColumn, Fields: map[string]KickoffAgentCell{}, Missing: ref.Missing}
		if e.Missing == nil {
			e.Missing = []string{}
		}
		for field, cell := range ref.Fields {
			if cell.NormalizationEvidence != nil {
				cell.NormalizationEvidence.NormalizedValue = cell.NormalizedValue
				if cell.NormalizationEvidence.Sheet == "" {
					cell.NormalizationEvidence.Sheet = cell.Sheet
				}
			}
			e.Fields[field] = KickoffAgentCell{Sheet: cell.Sheet, Cell: cell.Cell, Formula: cell.Formula != "", Normalization: cell.Normalization, Classification: cell.Classification, NormalizationEvidence: cell.NormalizationEvidence, RelationshipEvidence: cell.RelationshipEvidence, RelationshipReason: cell.RelationshipReason}
		}
		c.Evidence = append(c.Evidence, e)
	}
	return c
}
func kickoffAgentFilteredWorkbook(w bom.Workbook) bom.Workbook {
	out := w
	out.Sheets = append([]bom.Sheet{}, w.Sheets...)
	for i := range out.Sheets {
		out.Sheets[i].Cells = append([]bom.Cell{}, w.Sheets[i].Cells...)
		for j := range out.Sheets[i].Cells {
			c := &out.Sheets[i].Cells[j]
			if c.Sensitive || bom.Sensitive(c.Value) {
				c.Value = ""
				c.Formula = ""
				c.Sensitive = true
			}
		}
	}
	return out
}

func kickoffAgentFatal(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return true
	}
	for _, code := range []string{"AI_PROVIDER_NOT_CONFIGURED", "AI_CREDENTIAL_UNAVAILABLE", "PROVIDER_NOT_AVAILABLE", "PROVIDER_ERROR", "PROVIDER_TIMEOUT_OR_UNAVAILABLE", "PROVIDER_RESPONSE_FAILED", "ANALYSIS_STORAGE_UNAVAILABLE", "ANALYSIS_ARTIFACT_HASH_MISMATCH", "ENGINEERING_JOB_CLAIM_LOST"} {
		if strings.HasPrefix(err.Error(), code) {
			return true
		}
	}
	return false
}
