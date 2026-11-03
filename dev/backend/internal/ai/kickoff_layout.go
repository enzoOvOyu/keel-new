package ai

import (
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var kickoffUMark = regexp.MustCompile(`(?i)^(?:U\s*([0-9]+)|([0-9]+)\s*U)$`)
var kickoffUNumber = regexp.MustCompile(`^[0-9]+$`)
var kickoffURangeSeparator = regexp.MustCompile(`[-–—~～至]`)

type kickoffLayoutValue struct {
	Value    string
	Evidence map[string]any
	Missing  string
}

func kickoffUPosition(value string) (number int, marked, valid bool) {
	value = strings.TrimSpace(value)
	if parts := kickoffUMark.FindStringSubmatch(value); parts != nil {
		value, marked = parts[1]+parts[2], true
	} else if !kickoffUNumber.MatchString(value) {
		return 0, false, false
	}
	number, err := strconv.Atoi(value)
	return number, marked, err == nil && number > 0
}

// A literal inclusive U range is the only supported operation. No expression,
// unlabelled numeric range, merged-cell height or row geometry is evaluated.
func kickoffURange(value string) (start, height string) {
	parts := kickoffURangeSeparator.Split(strings.TrimSpace(value), -1)
	if len(parts) != 2 {
		return "", ""
	}
	a, aMarked, aValid := kickoffUPosition(parts[0])
	z, zMarked, zValid := kickoffUPosition(parts[1])
	if !aValid || !zValid || !aMarked && !zMarked {
		return "", ""
	}
	return strconv.Itoa(min(a, z)), strconv.Itoa(max(a, z) - min(a, z) + 1)
}

func kickoffUHeader(value string) bool {
	value = strings.ToLower(strings.Join(strings.Fields(value), ""))
	value = strings.NewReplacer("(", "", ")", "", "（", "", "）", "").Replace(value)
	switch value {
	case "u", "ru", "u位", "u刻度", "u位置", "机柜u位", "uposition", "rackunit", "rackunits":
		return true
	}
	return false
}

func kickoffUScaleCapacity(sheet bom.Sheet, region KickoffRegion, mapping KickoffMapping, binding KickoffBinding) kickoffLayoutValue {
	out := kickoffLayoutValue{Evidence: map[string]any{"rule": "explicit_contiguous_u_scale", "sheet": sheet.Name, "column": binding.Column, "mapping_bounds": mapping.Bounds, "region_bounds": region.Bounds}, Missing: "无法确认从1连续到N的完整U刻度"}
	if binding.Column < mapping.Bounds.StartColumn || binding.Column > mapping.Bounds.EndColumn || mapping.Bounds.StartRow < region.Bounds.StartRow || mapping.Bounds.EndRow > region.Bounds.EndRow {
		return out
	}
	column := map[int]bom.Cell{}
	ticks := []bom.Cell{}
	for _, cell := range sheet.Cells {
		if cell.Column != binding.Column || cell.Row < region.Bounds.StartRow || cell.Row > region.Bounds.EndRow {
			continue
		}
		column[cell.Row] = cell
		if cell.Row < mapping.Bounds.StartRow || cell.Row > mapping.Bounds.EndRow {
			continue
		}
		if cell.Sensitive || cell.Formula != "" {
			return out
		}
		if strings.TrimSpace(cell.Value) == "" {
			continue
		}
		if kickoffUHeader(cell.Value) {
			continue
		}
		if _, _, valid := kickoffUPosition(cell.Value); !valid {
			return out
		}
		ticks = append(ticks, cell)
	}
	if len(ticks) == 0 {
		return out
	}
	sort.Slice(ticks, func(i, j int) bool { return ticks[i].Row < ticks[j].Row })
	// A nearby header must be immediately above the scale, on this column
	// or an adjacent otherwise-unlabelled column, and inside the declared region.
	headers := []any{}
	conflictingHeader := false
	for _, cell := range sheet.Cells {
		if cell.Row < max(region.Bounds.StartRow, ticks[0].Row-2) || cell.Row >= ticks[0].Row || cell.Column != binding.Column {
			continue
		}
		if strings.TrimSpace(cell.Value) != "" && !kickoffUHeader(cell.Value) {
			conflictingHeader = true
		}
	}
	for _, cell := range sheet.Cells {
		if cell.Row < max(region.Bounds.StartRow, ticks[0].Row-2) || cell.Row >= ticks[0].Row || cell.Column < max(region.Bounds.StartColumn, binding.Column-1) || cell.Column > min(region.Bounds.EndColumn, binding.Column+1) || cell.Sensitive || cell.Formula != "" {
			continue
		}
		if kickoffUHeader(cell.Value) && (cell.Column == binding.Column || !conflictingHeader) {
			headers = append(headers, map[string]string{"sheet": sheet.Name, "cell": cell.Ref, "value": cell.Value})
		}
	}
	proof := []any{}
	first, previous, direction := 0, 0, 0
	for i, cell := range ticks {
		number, marked, _ := kickoffUPosition(cell.Value)
		if !marked && len(headers) == 0 && !(binding.Classification == "rack_u_axis" && kickoffPairedRackAxis(sheet, region, binding.Column, ticks)) {
			out.Missing = "数字刻度缺少明确U标识或邻近U表头"
			return out
		}
		if i == 0 {
			first = number
		} else {
			delta := number - previous
			if delta != 1 && delta != -1 || direction != 0 && delta != direction {
				return out
			}
			direction = delta
		}
		previous = number
		proof = append(proof, map[string]any{"sheet": sheet.Name, "cell": cell.Ref, "value": cell.Value, "u": number})
	}
	if min(first, previous) != 1 || max(first, previous) != len(ticks) {
		return out
	}
	// Any further scale mark beyond a boundary makes the mapping incomplete,
	// even if its number jumps. Blank rows never end the scale or add capacity.
	for _, side := range []int{-1, 1} {
		row, end := mapping.Bounds.StartRow-1, region.Bounds.StartRow
		if side == 1 {
			row, end = mapping.Bounds.EndRow+1, region.Bounds.EndRow
		}
		for ; side == -1 && row >= end || side == 1 && row <= end; row += side {
			cell := column[row]
			if cell.Sensitive || cell.Formula != "" {
				return out
			}
			if strings.TrimSpace(cell.Value) == "" {
				continue
			}
			if _, _, valid := kickoffUPosition(cell.Value); valid {
				return out
			}
			break
		}
	}
	out.Value, out.Missing = strconv.Itoa(max(first, previous)), ""
	out.Evidence["ticks"], out.Evidence["unit_headers"] = proof, headers
	out.Evidence["normalized_value"] = out.Value
	return out
}

// A merged occupied block is measured against explicit U ticks, never against
// spreadsheet row counts, row heights, borders, or blank cells.
func kickoffUGridPlacement(sheet bom.Sheet, region KickoffRegion, mapping KickoffMapping, binding KickoffBinding, key bom.Cell) kickoffLayoutValue {
	out := kickoffLayoutValue{Evidence: map[string]any{"rule": "occupied_block_u_axis", "sheet": sheet.Name, "occupied_cell": key.Ref}, Missing: "设备占位缺少完整、明确的U刻度依据"}
	axisMapping := mapping
	axisMapping.Bounds = region.Bounds
	scale := kickoffUScaleCapacity(sheet, region, axisMapping, KickoffBinding{Column: binding.Column, Classification: binding.Classification})
	if scale.Missing != "" {
		return out
	}
	first, last := key.Row, key.Row
	merged := false
	for _, area := range sheet.Merges {
		parts := strings.Split(area, ":")
		if len(parts) != 2 {
			continue
		}
		c1, r1 := kickoffAddress(parts[0])
		c2, r2 := kickoffAddress(parts[1])
		if key.Row == r1 && key.Column == c1 && r2 > r1 {
			if binding.Column >= c1 && binding.Column <= c2 || r1 < region.Bounds.StartRow || r2 > region.Bounds.EndRow {
				return out
			}
			first, last, merged = r1, r2, true
			out.Evidence["mapping_bounds"] = engineering.Bounds{StartRow: r1, EndRow: r2, StartColumn: c1, EndColumn: c2}
			break
		}
	}
	if binding.Normalize == "u_grid_height" && !merged {
		out.Missing = "单个设备标记仅能定位起始U，未明确设备占用高度"
		return out
	}
	ticks := map[int]bom.Cell{}
	for _, c := range sheet.Cells {
		if c.Column == binding.Column {
			ticks[c.Row] = c
		}
	}
	low, high, previous, direction := 0, 0, 0, 0
	proof := []any{}
	for row := first; row <= last; row++ {
		c, ok := ticks[row]
		n, _, valid := kickoffUPosition(c.Value)
		if !ok || !valid || c.Sensitive || c.Formula != "" {
			return out
		}
		if row == first {
			low, high = n, n
		} else {
			delta := n - previous
			if delta != 1 && delta != -1 || direction != 0 && delta != direction {
				return out
			}
			direction = delta
			low = min(low, n)
			high = max(high, n)
		}
		previous = n
		proof = append(proof, map[string]any{"sheet": sheet.Name, "cell": c.Ref, "value": c.Value, "u": n})
	}
	out.Value = strconv.Itoa(low)
	if binding.Normalize == "u_grid_height" {
		out.Value = strconv.Itoa(high - low + 1)
	}
	out.Missing = ""
	out.Evidence["ticks"] = proof
	out.Evidence["normalized_value"] = out.Value
	return out
}

// Bare numbers require an explicit AI classification AND a paired drawing axis
// around occupied blocks; numbering a normal table cannot establish rack units.
func kickoffPairedRackAxis(sheet bom.Sheet, region KickoffRegion, axis int, ticks []bom.Cell) bool {
	if len(ticks) < 2 {
		return false
	}
	index := map[[2]int]bom.Cell{}
	for _, c := range sheet.Cells {
		index[[2]int{c.Row, c.Column}] = c
	}
	for col := region.Bounds.StartColumn; col <= region.Bounds.EndColumn; col++ {
		if col == axis {
			continue
		}
		same := true
		for _, tick := range ticks {
			c := index[[2]int{tick.Row, col}]
			a, _, av := kickoffUPosition(tick.Value)
			b, _, bv := kickoffUPosition(c.Value)
			if !av || !bv || a != b || c.Sensitive || c.Formula != "" {
				same = false
				break
			}
		}
		if !same {
			continue
		}
		for _, area := range sheet.Merges {
			parts := strings.Split(area, ":")
			if len(parts) != 2 {
				continue
			}
			c1, r1 := kickoffAddress(parts[0])
			c2, r2 := kickoffAddress(parts[1])
			anchor := index[[2]int{r1, c1}]
			if c1 > min(axis, col) && c2 < max(axis, col) && r2 > r1 && r1 >= ticks[0].Row && r2 <= ticks[len(ticks)-1].Row && !anchor.Sensitive && (strings.TrimSpace(anchor.Value) != "" || anchor.Formula != "") {
				return true
			}
		}
	}
	return false
}
