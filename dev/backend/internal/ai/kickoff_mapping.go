package ai

import (
	"fmt"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
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
		key := fmt.Sprintf("%s/%s/%v", m.Region, m.Table, m.Bounds)
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
			if b.Column < 0 || b.Row < 0 || b.Row > 20000 || b.Column > 256 || b.OffsetColumn < -256 || b.OffsetColumn > 256 || b.OffsetRow < -20000 || b.OffsetRow > 20000 {
				return fmt.Errorf("INVALID_COORDINATE: %s", b.Field)
			}
			if b.Normalize != "" && b.Normalize != "trim" && b.Normalize != "unit" && b.Normalize != "mount_type" && b.Normalize != "face" && b.Normalize != "equipment_type" {
				return fmt.Errorf("UNSUPPORTED_NORMALIZATION: %s", b.Normalize)
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
				refs[b.Field] = map[string]any{"sheet": cell.Sheet, "cell": cell.Ref, "formula": cell.Formula, "cached_value": cell.Value, "normalization": b.Normalize}
				if cell.Sensitive {
					value = ""
					missing = append(missing, b.Field+":敏感字段未导入")
				}
				if cell.Formula != "" {
					value = ""
					missing = append(missing, b.Field+":公式缓存需独立核对")
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
				for i, f := range table.Fields {
					if f.Code == b.Field {
						row[i] = value
					}
				}
			}
			rows = append(rows, row)
			evidence = append(evidence, map[string]any{"table_id": m.Table, "row": len(rows) + 1, "source_region": region.ID, "source_sheet": region.Sheet, "source_row": r, "source_column": col, "source_key": key.Value, "fields": refs, "missing_inputs": missing})
		}
	}
	return rows, evidence, nil
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
