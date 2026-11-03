package ai

import (
	"fmt"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"regexp"
	"strings"
)

func validateKickoffMappings(w bom.Workbook, regions []KickoffRegion, step int, result KickoffTaskResult) error {
	if len(result.Mappings) == 0 && len(result.Missing) == 0 {
		return fmt.Errorf("MISSING_DISPOSITION_REQUIRED")
	}
	seen := map[string]bool{}
	for _, m := range result.Mappings {
		t, ok := kickoffTable(m.Table)
		if !ok || t.Step != step {
			return fmt.Errorf("INVALID_TARGET_TABLE: %s", m.Table)
		}
		if m.Layout != "rows" && m.Layout != "cells" {
			return fmt.Errorf("INVALID_LAYOUT: %s", m.Layout)
		}
		var region *KickoffRegion
		for i := range regions {
			if regions[i].ID == m.Region {
				region = &regions[i]
				break
			}
		}
		if region == nil || !kickoffBounds(w, region.Sheet, m.Bounds) || m.Bounds.StartRow < region.Bounds.StartRow || m.Bounds.EndRow > region.Bounds.EndRow || m.Bounds.StartColumn < region.Bounds.StartColumn || m.Bounds.EndColumn > region.Bounds.EndColumn {
			return fmt.Errorf("MAPPING_OUTSIDE_REGION: %s", m.Region)
		}
		// Endpoint A and B may share a source range but bind different columns.
		key := engineering.Digest(m)
		if seen[key] {
			return fmt.Errorf("DUPLICATE_MAPPING: %s", key)
		}
		seen[key] = true
		if m.Layout == "rows" && (m.KeyColumn < m.Bounds.StartColumn || m.KeyColumn > m.Bounds.EndColumn) {
			return fmt.Errorf("KEY_COLUMN_REQUIRED: %s", m.Region)
		}
		fields := map[string]bool{}
		for _, b := range m.Bindings {
			valid := false
			for _, f := range t.Fields {
				if f.Code == b.Field {
					valid = true
				}
			}
			if !valid || fields[b.Field] {
				return fmt.Errorf("INVALID_BINDING: %s", b.Field)
			}
			fields[b.Field] = true
			if b.Classification != "" {
				allowed := map[string]bool{"server": true, "switch": true, "gpu": true, "nic": true}
				axis := b.Classification == "rack_u_axis" && (b.Normalize == "u_scale_capacity" || b.Normalize == "u_grid_start" || b.Normalize == "u_grid_height")
				if !axis && (m.Table != "equipment_specs" || b.Field != "equipment_type" || !allowed[b.Classification] || b.Normalize != "") {
					return fmt.Errorf("INVALID_EQUIPMENT_CLASSIFICATION: %s", b.Field)
				}
			}
			if b.Column < 0 || b.Row < 0 || b.Row > 20000 || b.Column > 256 || b.OffsetColumn < -256 || b.OffsetColumn > 256 || b.OffsetRow < -20000 || b.OffsetRow > 20000 {
				return fmt.Errorf("INVALID_COORDINATE: %s", b.Field)
			}
			if b.Normalize != "" && b.Normalize != "trim" && b.Normalize != "unit" && b.Normalize != "mount_type" && b.Normalize != "face" && b.Normalize != "equipment_type" && b.Normalize != "rate_value" && b.Normalize != "rate_unit" && b.Normalize != "u_scale_capacity" && b.Normalize != "u_range_start" && b.Normalize != "u_range_height" && b.Normalize != "u_grid_start" && b.Normalize != "u_grid_height" {
				return fmt.Errorf("UNSUPPORTED_NORMALIZATION: %s", b.Normalize)
			}
			tableID := strings.TrimPrefix(m.Table, "kickoff23/")
			switch b.Normalize {
			case "u_scale_capacity":
				if tableID != "racks" || b.Field != "capacity_u" || b.Lookup != nil || b.Column < m.Bounds.StartColumn || b.Column > m.Bounds.EndColumn || b.Row != 0 || b.OffsetRow != 0 || b.OffsetColumn != 0 {
					return fmt.Errorf("INVALID_U_SCALE_NORMALIZATION: %s", b.Field)
				}
			case "u_grid_start", "u_grid_height":
				if tableID != "placements" || b.Normalize == "u_grid_start" && b.Field != "start_u" || b.Normalize == "u_grid_height" && b.Field != "height_u" || b.Column < 1 || b.Lookup != nil {
					return fmt.Errorf("INVALID_U_GRID_NORMALIZATION: %s", b.Field)
				}
			case "u_range_start", "u_range_height":
				if tableID != "placements" || b.Normalize == "u_range_start" && b.Field != "start_u" || b.Normalize == "u_range_height" && b.Field != "height_u" {
					return fmt.Errorf("INVALID_U_RANGE_NORMALIZATION: %s", b.Field)
				}
			}
			if (b.Normalize == "rate_value" && b.Field != "rate") || (b.Normalize == "rate_unit" && b.Field != "rate_unit") {
				return fmt.Errorf("INVALID_RATE_NORMALIZATION: %s", b.Field)
			}
			if m.Layout == "rows" && b.Column == 0 {
				return fmt.Errorf("SOURCE_COLUMN_REQUIRED: %s", b.Field)
			}
			if b.Lookup != nil {
				l := b.Lookup
				if l.KeyColumn < 1 || l.ValueColumn < 1 || !kickoffBounds(w, l.Sheet, engineering.Bounds{StartRow: l.StartRow, EndRow: l.EndRow, StartColumn: min(l.KeyColumn, l.ValueColumn), EndColumn: max(l.KeyColumn, l.ValueColumn)}) {
					return fmt.Errorf("INVALID_LOOKUP: %s", b.Field)
				}
			}
		}
	}
	return nil
}

// Expansion runs locally against original cells. The model never returns raw
// identity values or materialized rows and cannot invent missing source data.
func expandKickoffRegion(w bom.Workbook, region KickoffRegion, m KickoffMapping) ([][]string, any, error) {
	table, _ := kickoffTable(m.Table)
	sheet, _ := kickoffSheet(w, region.Sheet)
	cells := map[[2]int]bom.Cell{}
	for _, c := range sheet.Cells {
		cells[[2]int{c.Row, c.Column}] = c
	}
	read := func(row, col int) bom.Cell {
		if c, ok := cells[[2]int{row, col}]; ok && strings.TrimSpace(c.Value) != "" {
			return c
		}
		for _, merge := range sheet.Merges {
			var c1, r1, c2, r2 int
			parts := strings.Split(merge, ":")
			if len(parts) != 2 {
				continue
			}
			c1, r1 = kickoffAddress(parts[0])
			c2, r2 = kickoffAddress(parts[1])
			if row >= r1 && row <= r2 && col >= c1 && col <= c2 {
				return cells[[2]int{r1, c1}]
			}
		}
		return cells[[2]int{row, col}]
	}
	rows := [][]string{}
	evidence := []any{}
	scales := map[string]kickoffLayoutValue{}
	for _, b := range m.Bindings {
		if b.Normalize == "u_scale_capacity" {
			scales[b.Field] = kickoffUScaleCapacity(sheet, region, m, b)
		}
	}
	lookupIndices := map[string]map[string][]bom.Cell{}
	for _, b := range m.Bindings {
		if b.Lookup == nil {
			continue
		}
		l := b.Lookup
		s, _ := kickoffSheet(w, l.Sheet)
		values := map[[2]int]bom.Cell{}
		for _, c := range s.Cells {
			values[[2]int{c.Row, c.Column}] = c
		}
		index := map[string][]bom.Cell{}
		for r := l.StartRow; r <= l.EndRow; r++ {
			k := values[[2]int{r, l.KeyColumn}]
			v := values[[2]int{r, l.ValueColumn}]
			if !k.Sensitive && strings.TrimSpace(k.Value) != "" {
				index[strings.TrimSpace(k.Value)] = append(index[strings.TrimSpace(k.Value)], v)
			}
		}
		lookupIndices[b.Field] = index
	}
	for r := m.Bounds.StartRow; r <= m.Bounds.EndRow; r++ {
		first, last := m.KeyColumn, m.KeyColumn
		if m.Layout == "cells" {
			first = m.Bounds.StartColumn
			last = m.Bounds.EndColumn
		}
		for col := first; col <= last; col++ {
			key := cells[[2]int{r, col}]
			if key.Sensitive || strings.TrimSpace(key.Value) == "" {
				continue
			}
			row := make([]string, len(table.Fields))
			refs := map[string]any{}
			missing := []string{}
			for _, b := range m.Bindings {
				rr, cc := r+b.OffsetRow, col+b.OffsetColumn
				if b.Row > 0 {
					rr = b.Row
				}
				if b.Column > 0 {
					cc = b.Column
				}
				cell := read(rr, cc)
				value := strings.TrimSpace(cell.Value)
				if b.Lookup != nil {
					matches := lookupIndices[b.Field][value]
					if len(matches) != 1 {
						missing = append(missing, b.Field+":关联键缺失或不唯一")
						value = ""
					} else {
						cell = matches[0]
						value = strings.TrimSpace(cell.Value)
					}
				}
				refs[b.Field] = map[string]any{"sheet": cell.Sheet, "cell": cell.Ref, "formula": cell.Formula, "cached_value": cell.Value, "normalization": b.Normalize, "classification": b.Classification}
				if cell.Sensitive {
					value = ""
					missing = append(missing, b.Field+":敏感字段未导入")
				}
				if cell.Formula != "" {
					value = ""
					missing = append(missing, b.Field+":公式缓存需独立核对")
				}
				if b.Classification != "" && b.Classification != "rack_u_axis" && value != "" {
					value = b.Classification
					refs[b.Field].(map[string]any)["state"] = "ai_classification_needs_review"
				}
				if strings.HasSuffix(b.Field, "serial") || b.Field == "serial" {
					source, sourceColumn, headerStart, dataStart := sheet, cc, region.Bounds.StartRow, m.Bounds.StartRow
					if b.Lookup != nil {
						source, _ = kickoffSheet(w, b.Lookup.Sheet)
						sourceColumn = b.Lookup.ValueColumn
						headerStart = 1
						dataStart = b.Lookup.StartRow
					}
					serialHeader := false
					for _, header := range source.Cells {
						if header.Column != sourceColumn || header.Row < headerStart || header.Row >= dataStart {
							continue
						}
						h := strings.ToLower(strings.TrimSpace(header.Value))
						if bom.Field(header.Value) == "serial" || strings.Contains(h, "序列号") || strings.HasSuffix(h, "sn") {
							serialHeader = true
							break
						}
					}
					if !serialHeader {
						value = ""
						missing = append(missing, b.Field+":缺少明确SN来源，规划标签不能替代物理身份")
					}
				}
				if b.Normalize == "mount_type" {
					v := strings.ToLower(value)
					if v == "u" || v == "u位" || v == "按u安装" {
						value = "u"
					} else if v == "non_u" || v == "非u安装" {
						value = "non_u"
					} else {
						value = ""
					}
				}
				if b.Normalize == "face" {
					value = map[string]string{"前": "front", "前面": "front", "front": "front", "后": "rear", "后面": "rear", "rear": "rear", "双面": "both", "both": "both"}[strings.ToLower(value)]
				}
				if b.Normalize == "equipment_type" {
					value = map[string]string{"服务器": "server", "server": "server", "交换机": "switch", "switch": "switch", "gpu": "gpu", "nic": "nic", "网卡": "nic"}[strings.ToLower(value)]
				}
				if b.Normalize == "unit" {
					value = kickoffUnit(value)
				}
				if b.Normalize == "rate_value" || b.Normalize == "rate_unit" {
					number, unit := kickoffRate(value)
					value = number
					if b.Normalize == "rate_unit" {
						value = unit
					}
				}
				if b.Normalize == "u_scale_capacity" {
					scale := scales[b.Field]
					value = scale.Value
					refs[b.Field].(map[string]any)["normalization_evidence"] = scale.Evidence
					refs[b.Field].(map[string]any)["normalized_value"] = value
					if scale.Missing != "" {
						missing = append(missing, b.Field+":"+scale.Missing)
					}
				}
				if b.Normalize == "u_range_start" || b.Normalize == "u_range_height" {
					start, height := kickoffURange(value)
					value = start
					if b.Normalize == "u_range_height" {
						value = height
					}
					refs[b.Field].(map[string]any)["normalization_evidence"] = map[string]string{"rule": "inclusive_u_range", "start_u": start, "height_u": height}
					refs[b.Field].(map[string]any)["normalized_value"] = value
					if value == "" {
						missing = append(missing, b.Field+":无法确认明确带U标识的连续U区间")
					}
				}
				if b.Normalize == "u_grid_start" || b.Normalize == "u_grid_height" {
					derived := kickoffUGridPlacement(sheet, region, m, b, key)
					value = derived.Value
					refs[b.Field].(map[string]any)["normalization_evidence"] = derived.Evidence
					refs[b.Field].(map[string]any)["normalized_value"] = value
					if derived.Missing != "" {
						missing = append(missing, b.Field+":"+derived.Missing)
					}
				}
				for i, f := range table.Fields {
					if f.Code == b.Field {
						row[i] = value
					}
				}
			}
			hasValue := false
			for _, value := range row {
				hasValue = hasValue || strings.TrimSpace(value) != ""
			}
			if !hasValue {
				evidence = append(evidence, map[string]any{"table_id": m.Table, "row": nil, "state": "source_only_no_target_values", "source_region": region.ID, "source_sheet": region.Sheet, "source_row": r, "source_column": col, "source_key": key.Value, "fields": refs, "missing_inputs": missing})
				continue
			}
			rows = append(rows, row)
			evidence = append(evidence, map[string]any{"table_id": m.Table, "row": len(rows) + 1, "source_region": region.ID, "source_sheet": region.Sheet, "source_row": r, "source_column": col, "source_key": key.Value, "fields": refs, "missing_inputs": missing})
		}
	}
	return rows, evidence, nil
}

var kickoffRatePattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*(bps|[kMGT](?:bps|bit/s)?)$`)

// Only an explicit bandwidth with a recognized unit is split. Product names,
// bare numbers, lane expressions and ranges need separate source evidence.
func kickoffRate(value string) (string, string) {
	parts := kickoffRatePattern.FindStringSubmatch(strings.TrimSpace(value))
	if parts == nil {
		return "", ""
	}
	unit := "bps"
	if strings.ToLower(parts[2]) != "bps" {
		unit = map[byte]string{'k': "kbps", 'm': "Mbps", 'g': "Gbps", 't': "Tbps"}[strings.ToLower(parts[2])[0]]
	}
	return parts[1], unit
}
func kickoffAddress(cell string) (col, row int) {
	for _, c := range strings.ReplaceAll(cell, "$", "") {
		if c >= 'A' && c <= 'Z' {
			col = col*26 + int(c-'A'+1)
		} else if c >= '0' && c <= '9' {
			row = row*10 + int(c-'0')
		}
	}
	return
}

// Endpoint references are source-backed identities, not inferred specifications.
// Leave all unknown specifications blank so the existing review gate applies.
func completeKickoffPortIdentities(tables map[string][][]string) []any {
	terms, _ := kickoffTable("terminations")
	specs, _ := kickoffTable("port_specs")
	index := func(t engineering.InputTable, field string) int {
		for i, f := range t.Fields {
			if f.Code == field {
				return i
			}
		}
		panic("unknown kickoff contract field: " + field)
	}
	si, pi := index(specs, "serial"), index(specs, "port")
	seen := map[[2]string]bool{}
	for _, r := range tables["port_specs"] {
		seen[[2]string{strings.TrimSpace(r[si]), strings.TrimSpace(r[pi])}] = true
	}
	evidence := []any{}
	for i, row := range tables["terminations"] {
		for _, side := range []string{"a", "b"} {
			sn := strings.TrimSpace(row[index(terms, side+"_serial")])
			port := strings.TrimSpace(row[index(terms, side+"_port")])
			key := [2]string{sn, port}
			if sn == "" || port == "" || seen[key] {
				continue
			}
			seen[key] = true
			r := make([]string, len(specs.Fields))
			r[si], r[pi] = sn, port
			tables["port_specs"] = append(tables["port_specs"], r)
			evidence = append(evidence, map[string]any{"target_table": "port_specs", "target_row": len(tables["port_specs"]) + 1, "source_table": "terminations", "source_row": i + 2, "side": side, "state": "identity_only_specs_need_input"})
		}
	}
	return evidence
}
func kickoffUnit(value string) string {
	v := strings.TrimSpace(value)
	for _, u := range []string{"Gbps", "Mbps", "kW", "mm", "cm", "kVA", "W", "m", "U"} {
		if v == u || strings.HasSuffix(v, "("+u+")") || strings.HasSuffix(v, "（"+u+"）") {
			return u
		}
	}
	return ""
}

func kickoffPreview(s bom.Sheet) any {
	preview := engineeringSheetPreview(s, bom.NewWorkbookIR(bom.Workbook{Sheets: []bom.Sheet{s}}).InspectWorkbook()).(map[string]any)
	if samples, ok := preview["sample_cells"].([]map[string]any); ok && len(samples) > 48 {
		preview["sample_cells"] = samples[:48]
	}
	if regions, ok := preview["regions"].([]bom.Region); ok && len(regions) > 24 {
		preview["regions"] = regions[:24]
		preview["region_index_truncated"] = true
	}
	return preview
}
func kickoffRegionCoverage(w bom.Workbook, r KickoffRegion, tasks map[int]KickoffTaskResult) any {
	s, _ := kickoffSheet(w, r.Sheet)
	unmapped := []string{}
	mapped := 0
	for _, cell := range s.Cells {
		if cell.Row < r.Bounds.StartRow || cell.Row > r.Bounds.EndRow || cell.Column < r.Bounds.StartColumn || cell.Column > r.Bounds.EndColumn || cell.Value == "" {
			continue
		}
		consumed := false
		for _, task := range tasks {
			for _, m := range task.Mappings {
				if m.Region != r.ID {
					continue
				}
				for _, b := range m.Bindings {
					rr := cell.Row - b.OffsetRow
					cc := cell.Column - b.OffsetColumn
					if b.Row > 0 {
						rr = m.Bounds.StartRow
					}
					if b.Column > 0 {
						cc = m.Bounds.StartColumn
					}
					if rr >= m.Bounds.StartRow && rr <= m.Bounds.EndRow && cc >= m.Bounds.StartColumn && cc <= m.Bounds.EndColumn && (b.Row == 0 || b.Row == cell.Row) && (b.Column == 0 || b.Column == cell.Column) {
						consumed = true
					}
				}
			}
		}
		if consumed {
			mapped++
		} else {
			unmapped = append(unmapped, cell.Ref)
		}
	}
	return map[string]any{"region": r.ID, "sheet": r.Sheet, "mapped_cells": mapped, "unmapped_cells": unmapped, "unmapped_state": "needs_input", "reason": r.Reason}
}

func kickoffSamples(w bom.Workbook, regions []KickoffRegion) []any {
	out := []any{}
	for _, r := range regions {
		b := r.Bounds
		b.EndRow = min(b.EndRow, b.StartRow+2)
		b.EndColumn = min(b.EndColumn, b.StartColumn+31)
		cells, _ := engineering.SourceRange(w, r.Sheet, b)
		out = append(out, map[string]any{"region": r.ID, "sample": engineeringCellPreview(cells), "full_bounds": r.Bounds, "sample_is_not_full_data": true})
	}
	return out
}
