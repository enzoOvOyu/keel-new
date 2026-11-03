package ai

import (
	"keel.local/dev/internal/bom"
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
		if !marked && len(headers) == 0 {
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
