package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/xuri/excelize/v2"
	"io"
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

type EngineeringToolHooks struct {
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
	var graph []engineering.FormulaEvidence
	var proposal engineering.EngineeringProposal
	prompt, e := prompts.ReadFile("prompts/engineering-v1.txt")
	if e != nil {
		return proposal, e
	}
	ir := bom.NewWorkbookIR(w)
	history := []any{}
	registry := engineering.DefaultRegistry().Definitions()
	summary := []any{}
	for _, s := range w.Sheets {
		formulas := 0
		for _, c := range s.Cells {
			if c.Formula != "" {
				formulas++
			}
		}
		summary = append(summary, map[string]any{"sheet": s.Name, "cells": len(s.Cells), "merges": s.Merges, "formulas": formulas, "dimension": s.Dimension})
	}
	for turn := 0; turn < 40; turn++ {
		raw, err := p.completeJSON(ctx, string(prompt), map[string]any{"workbook": summary, "modules": registry, "history": history, "turns_remaining": 40 - turn})
		if err != nil {
			return proposal, err
		}
		if schemaErr := validateEngineeringSchema(ctx, raw); schemaErr != nil {
			if schemaErr.Error() == "AI_SCHEMA_VALIDATOR_UNAVAILABLE" {
				return proposal, schemaErr
			}
			if e := trace("schema_validation", "rejected", schemaErr.Error()); e != nil {
				return proposal, e
			}
			history = append(history, map[string]string{"error": schemaErr.Error()})
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
				result = map[string]any{"sheet": s.Name, "merges": s.Merges, "regions": regions}
			case "inspect_region", "read_range":
				if action.Bounds == nil {
					err = fmt.Errorf("BOUNDS_REQUIRED")
				} else {
					result, err = engineering.SourceRange(w, action.Sheet, *action.Bounds)
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
					covered := map[string]bool{}
					for _, r := range action.Proposal.Regions {
						covered[r.Sheet] = true
					}
					missing := []string{}
					for _, s := range w.Sheets {
						if len(s.Cells) > 0 && !covered[s.Name] {
							missing = append(missing, s.Name)
						}
					}
					if len(missing) > 0 {
						err = fmt.Errorf("UNACCOUNTED_SHEETS: %s", strings.Join(missing, ", "))
					}
				}
				if err == nil {
					proposal = *action.Proposal
					result = map[string]any{"schema_valid": true, "regions": len(proposal.Regions), "formal_write": false}
					if action.Action == "validate_model" || action.Action == "analyze_validation_issues" {
						x, extractErr := engineering.ExtractPhysical(w, proposal, "analysis", "source", "revision")
						if extractErr != nil {
							err = extractErr
						} else {
							result = map[string]any{"issues": x.Issues, "equipment": len(x.Model.Equipment), "facilities": len(x.Model.Facilities), "placements": len(x.Model.Placements), "mapped_cells": x.MappedCells, "total_cells": x.TotalCells}
						}
					}
					if action.Action == "propose_engineering_model" {
						if e := trace(action.Action, "succeeded", "结构化区域、字段及模块提案已校验；等待本地提取与人工审核"); e != nil {
							return proposal, e
						}
						return proposal, nil
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
			result = map[string]string{"error": detail}
		}
		if e := trace(action.Action, state, detail); e != nil {
			return proposal, e
		}
		history = append(history, map[string]any{"action": action, "result": result})
		if len(history) > 12 {
			history = history[len(history)-12:]
		}
	}
	return proposal, fmt.Errorf("ENGINEERING_AGENT_TURN_LIMIT")
}
