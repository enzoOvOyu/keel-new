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
	"sort"
	"strings"
)

// Plans describe source coordinates, never generated inventory rows.
type KickoffBinding struct {
	Field        string         `json:"field"`
	Normalize    string         `json:"normalize,omitempty"`
	Column       int            `json:"column,omitempty"`
	Row          int            `json:"row,omitempty"`
	OffsetRow    int            `json:"offset_row,omitempty"`
	OffsetColumn int            `json:"offset_column,omitempty"`
	Lookup       *KickoffLookup `json:"lookup,omitempty"`
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
	Verified    map[string]KickoffVerifiedCell `json:"verified_cells,omitempty"`
	Fingerprint string                         `json:"fingerprint"`
	Regions     []KickoffRegion                `json:"regions"`
	Tasks       map[int]KickoffTaskResult      `json:"tasks"`
	Coverage    []KickoffCoverage              `json:"coverage"`
}
type KickoffDraft struct {
	TableID string                     `json:"table_id"`
	CSV     string                     `json:"csv"`
	Issues  []engineering.InputProblem `json:"issues"`
}
type KickoffResult struct {
	Drafts   []KickoffDraft    `json:"drafts"`
	Missing  []string          `json:"missing_inputs"`
	Coverage []KickoffCoverage `json:"coverage"`
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
	for _, t := range engineering.StepInputCatalog().Tables {
		if t.ID == id {
			return t, true
		}
	}
	return engineering.InputTable{}, false
}

func (p *Provider) ExtractKickoff(ctx context.Context, safe, original bom.Workbook, trace func(string, string, string) error, hooks EngineeringToolHooks) (KickoffResult, error) {
	result := KickoffResult{Drafts: []KickoffDraft{}, Missing: []string{}, Coverage: []KickoffCoverage{}}
	promptSource, _ := prompts.ReadFile("prompts/kickoff-v1.txt")
	fingerprint := engineering.Digest([]any{hooks.SourceHash, p.ConfigID, p.ConfigRevision, p.Model, string(promptSource), engineering.StepInputCatalog().Version})
	cp := KickoffCheckpoint{Fingerprint: fingerprint, Regions: []KickoffRegion{}, Tasks: map[int]KickoffTaskResult{}, Coverage: []KickoffCoverage{}}
	if hooks.Resume != nil && hooks.Resume.Version == engineeringCheckpointVersion && hooks.Resume.Kickoff != nil && hooks.Resume.Kickoff.Fingerprint == fingerprint {
		cp = *hooks.Resume.Kickoff
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
	var graph []engineering.FormulaEvidence
	// One semantic correction, with a bounded number of read tools, per task.
	ask := func(task string, input map[string]any, decode func([]byte) error) error {
		history := []any{}
		correction := false
		for turn := 0; turn < 8; turn++ {
			input["history"] = boundedEngineeringHistory(history)
			input["remaining_tool_calls"] = 7 - turn
			input["must_finish"] = turn == 7
			prompt, _ := prompts.ReadFile("prompts/kickoff-v1.txt")
			callPrompt := string(prompt) + "\nTask: " + task
			if turn == 7 {
				callPrompt += "\nNo read/tool calls remain. Return the final route or extraction plan now, with explicit missing_inputs for unresolved facts. Do not request another tool."
			}
			raw, err := p.engineeringCompletion(ctx, callPrompt, input, nil, trace, hooks.Artifact)
			if err != nil {
				if (err.Error() == "INVALID_JSON" || err.Error() == "SCHEMA_VALIDATION_FAILED" || err.Error() == "PROVIDER_OUTPUT_TRUNCATED") && !correction {
					correction = true
					history = append(history, map[string]any{"error": err.Error()})
					continue
				}
				return err
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
				Action  string                        `json:"action"`
				Sheet   string                        `json:"sheet"`
				Bounds  engineering.Bounds            `json:"bounds"`
				Cell    string                        `json:"cell"`
				Rule    *engineering.CalculationRule  `json:"rule,omitempty"`
				RuleRef *engineering.VersionReference `json:"rule_ref,omitempty"`
				Cells   []engineering.FormulaCell     `json:"cells,omitempty"`
				Inputs  []engineering.NamedValue      `json:"inputs,omitempty"`
			}
			_ = json.Unmarshal(raw, &action)
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
			if correction {
				return err
			}
			correction = true
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
		err := ask("route", map[string]any{"sheets": sheets, "steps": engineering.StepInputCatalog()}, func(raw []byte) error {
			var plan struct {
				Regions []KickoffRegion `json:"regions"`
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
					if step < 1 || step > 5 {
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
			return nil
		})
		if err != nil {
			return result, err
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
	for step := 1; step <= 5; step++ {
		if _, ok := cp.Tasks[step]; ok {
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
			task.Missing = append(task.Missing, fmt.Sprintf("步骤%d：资料中未发现本步明确依据，请补充建模输入。", step))
		} else {
			tables := []engineering.InputTable{}
			for _, t := range engineering.StepInputCatalog().Tables {
				if t.Step == step {
					tables = append(tables, t)
				}
			}
			err := ask(fmt.Sprintf("extract_step_%d", step), map[string]any{"regions": regions, "region_samples": kickoffSamples(safe, regions), "tables": tables, "prior_tasks": kickoffTaskIndex(cp.Tasks)}, func(raw []byte) error {
				var proposed KickoffTaskResult
				if err := kickoffDecode(raw, &proposed); err != nil {
					return err
				}
				if err := validateKickoffMappings(original, regions, step, proposed); err != nil {
					return err
				}
				task = proposed
				return nil
			})
			if err != nil {
				if ctx.Err() != nil {
					return result, ctx.Err()
				}
				var providerErr *ProviderError
				if errors.As(err, &providerErr) {
					return result, err
				}
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
	rowsByTable := map[string][][]string{}
	for step := 1; step <= 5; step++ {
		task := cp.Tasks[step]
		result.Missing = append(result.Missing, task.Missing...)
		for _, mapping := range task.Mappings {
			rows, evidence, err := expandKickoffRegion(original, func() KickoffRegion {
				for _, r := range cp.Regions {
					if r.ID == mapping.Region {
						return r
					}
				}
				return KickoffRegion{}
			}(), mapping)
			if err != nil {
				return result, err
			}
			rowsByTable[mapping.Table] = append(rowsByTable[mapping.Table], rows...)
			if err = artifact("kickoff_evidence", evidence); err != nil {
				return result, err
			}
			result.Coverage = append(result.Coverage, KickoffCoverage{Region: mapping.Region, Step: step, State: "extracted", Rows: len(rows), Reason: "固定格式候选已展开；未知字段和未消费单元格保留在coverage详情"})
		}
	}
	for _, r := range cp.Regions {
		if err := artifact("kickoff_region_coverage", kickoffRegionCoverage(original, r, cp.Tasks)); err != nil {
			return result, err
		}
		for _, step := range r.Steps {
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
	ids := []string{}
	for id := range rowsByTable {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
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
	return result, nil
}
func kickoffTaskIndex(tasks map[int]KickoffTaskResult) any {
	out := map[int]any{}
	for k, v := range tasks {
		out[k] = map[string]any{"mapping_count": len(v.Mappings), "missing_inputs": v.Missing}
	}
	return out
}
