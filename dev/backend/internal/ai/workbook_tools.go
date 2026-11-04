package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xuri/excelize/v2"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"time"
)

// Increment when source interpretation changes invalidate previously accepted mappings.
const engineeringCheckpointVersion = 3

type EngineeringCheckpoint struct {
	Kickoff         *KickoffCheckpoint                         `json:"kickoff,omitempty"`
	Version         int                                        `json:"version"`
	Proposal        engineering.EngineeringProposal            `json:"proposal"`
	InspectedSheets []string                                   `json:"inspected_sheets"`
	History         []any                                      `json:"history"`
	CompletedSheets []string                                   `json:"completed_sheets"`
	PendingDrafts   map[string]engineering.EngineeringProposal `json:"pending_drafts,omitempty"`
}
type EngineeringToolHooks struct {
	VerifyWorkbook func(context.Context) (map[string]KickoffVerifiedCell, error)
	SourceHash     string
	Resume         *EngineeringCheckpoint
	Checkpoint     func(context.Context, EngineeringCheckpoint) error
	Artifact       func(context.Context, string, string, any) error
	Attributes     []engineering.AttributeDefinition
	Names          []excelize.DefinedName
	ProposeRule    func(context.Context, engineering.CalculationRule) (engineering.RuleView, error)
	Calculate      func(context.Context, *engineering.VersionReference, []engineering.FormulaCell, []engineering.NamedValue) (engineering.CalculationPage, error)
}

func (p *Provider) engineeringCompletion(ctx context.Context, prompt string, input any, functions []any, trace func(string, string, string) error, artifacts ...func(context.Context, string, string, any) error) ([]byte, error) {
	corrections := 0
	for attempt := 0; ; attempt++ {
		raw, err := p.complete(ctx, prompt, input, functions)
		if err == nil {
			return raw, nil
		}
		if err.Error() == "INVALID_JSON" || err.Error() == "PROVIDER_OUTPUT_TRUNCATED" || err.Error() == "SCHEMA_VALIDATION_FAILED" {
			if len(artifacts) > 0 && artifacts[0] != nil {
				if e := artifacts[0](ctx, "provider_output", "rejected", map[string]any{"error": err.Error(), "response": string(p.Raw)}); e != nil {
					return nil, e
				}
			}
			if corrections >= 1 {
				_ = trace("provider_request", "failed", err.Error())
				return nil, err
			}
			corrections++
			if e := trace("provider_output", "retrying", err.Error()+"；要求模型重新输出精简、合法的工具参数"); e != nil {
				return nil, e
			}
			instruction := "Return exactly one complete JSON object matching the current task's action or final-result schema. Do not output commentary or markdown."
			if len(functions) > 0 {
				instruction = "Call exactly one supplied function with a complete valid JSON object as arguments. Do not output commentary or markdown."
			}
			prompt += "\nYour previous output was rejected: " + err.Error() + ". " + instruction
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

// Tool examples from a completed/different sheet must not steer a resumed sheet.
// Full history remains in artifacts; only the active evidence enters this turn.

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
