package ai

import (
	"fmt"
	"sort"
	"strings"

	"keel.local/dev/internal/bom"
)

// Validate semantic extraction separately from raw-cell retention. These checks
// only use explicit source headers and values; they never invent missing assets.
func inspectSourceCoverage(s bom.Sheet, p SheetPlan) error {
	rows := bom.MergedSourceCells(s)
	// A changed header inside a data range is a new table, not a new
	// type value. Report the structural correction before type filtering.
	for _, t := range p.Tables {
		if t.Kind == "reference" {
			continue
		}
		heads := sourceHeaders(s, p, t.Start)
		for row := t.Start; row <= t.End; row++ {
			known, conflicts := 0, 0
			for col, cell := range rows[row] {
				meaning := bom.Field(cell.Value)
				if cell.Sensitive || meaning == "" {
					continue
				}
				known++
				old := bom.Field(heads[col].Value)
				if old != "" && old != meaning {
					conflicts++
				}
			}
			if known >= 3 && conflicts >= 2 {
				return fmt.Errorf("TABLE_CROSSES_HEADER: row=%d column=1; split the table before this changed header, add this header row, and bind the following region independently", row)
			}
		}
	}
	seen := map[string]bool{}
	connections := map[string]bool{}
	for _, t := range p.Tables {
		if t.Kind == "reference" {
			continue
		}
		for row := t.Start; row <= t.End; row++ {
			if !tableRowPresent(t, rows[row]) || !tableTypeMatches(t, rows[row]) {
				continue
			}
			key := fmt.Sprint(row, ":", SourceAnchor(t.Fields))
			if seen[key] {
				return fmt.Errorf("DUPLICATE_SOURCE_OBJECT: row=%d; overlapping extractors produce the same source identity", row)
			}
			seen[key] = true
			if t.Kind == "physical_connection" && t.Fields["endpoint_a_port"] > 0 && t.Fields["endpoint_b_port"] > 0 {
				key := fmt.Sprint(row, ":", t.Fields["endpoint_a_port"], ":", t.Fields["endpoint_b_port"])
				if connections[key] {
					return fmt.Errorf("DUPLICATE_SOURCE_CONNECTION: row=%d column=%d; alternate device labels for the same source ports must not create another cable", row, t.Fields["endpoint_a_port"])
				}
				connections[key] = true
			}
		}
	}
	// Every otherwise extractable typed row must match an extractor. A new
	// distinct category cannot silently disappear because type_values omitted it.
	for _, t := range p.Tables {
		if t.TypeColumn == 0 || t.Kind == "reference" {
			continue
		}
		for row := t.Start; row <= t.End; row++ {
			if !tableRowPresent(t, rows[row]) {
				continue
			}
			covered := false
			for _, u := range p.Tables {
				if u.Kind == t.Kind && row >= u.Start && row <= u.End && tableRowPresent(u, rows[row]) && tableTypeMatches(u, rows[row]) {
					covered = true
					break
				}
			}
			if !covered {
				return fmt.Errorf("SOURCE_TYPE_UNCOVERED: row=%d column=%d; inspect the actual value and include it in type_values", row, t.TypeColumn)
			}
		}
	}
	headers := map[int]map[string][]int{}
	for _, c := range s.Cells {
		if c.Sensitive {
			continue
		}
		f := bom.Field(c.Value)
		if f == "" {
			continue
		}
		if headers[c.Row] == nil {
			headers[c.Row] = map[string][]int{}
		}
		headers[c.Row][f] = append(headers[c.Row][f], c.Column)
	}
	order := []int{}
	for row, h := range headers {
		inventory := len(h["hostname"]) == 1 && len(h["model"]) > 0
		explicitCable := len(h["endpoint_a_device"]) > 0 && len(h["endpoint_b_device"]) > 0 && (len(h["endpoint_a_port"])+len(h["endpoint_b_port"])+len(h["endpoint_a_uid"])+len(h["endpoint_b_uid"]) > 0)
		if inventory || explicitCable || dualEndpointHeaders(h) {
			order = append(order, row)
		}
	}
	sort.Ints(order)
	max := 0
	for row := range rows {
		if row > max {
			max = row
		}
	}
	for j, header := range order {
		end := max
		if j+1 < len(order) {
			end = order[j+1] - 1
		}
		h := headers[header]
		for row := header + 1; row <= end; row++ {
			identity := false
			for _, f := range []string{"hostname", "endpoint_a_device", "endpoint_b_device"} {
				for _, col := range h[f] {
					if bom.ExtractCellValue(rows[row][col], f) != "" {
						identity = true
					}
				}
			}
			if !identity {
				continue
			}
			if dualEndpointHeaders(h) {
				for source, targets := range map[string][]string{"hostname": {"endpoint_a_device", "endpoint_b_device"}, "port_index": {"endpoint_a_port", "endpoint_b_port"}, "model": {"endpoint_a_optic", "endpoint_b_optic", "model"}} {
					for _, col := range h[source] {
						if bom.ExtractCellValue(rows[row][col], source) == "" {
							continue
						}
						covered := false
						for _, field := range targets {
							if planMapsCell(p, rows[row], row, field, col) {
								covered = true
							}
						}
						if !covered {
							return fmt.Errorf("SOURCE_FIELD_UNCOVERED: row=%d column=%d field=%s; preserve each explicit side of the connection", row, col, source)
						}
					}
				}
			}
			// Deterministic diagnostics contain coordinates, never asset values.
			for _, f := range []string{"hostname", "device_alias", "serial", "model", "category", "management_ip", "mac", "rack", "site", "u_start", "u_height", "endpoint_a_device", "endpoint_a_port", "endpoint_b_device", "endpoint_b_port", "endpoint_a_optic", "endpoint_b_optic", "length", "speed", "media_type"} {
				if dualEndpointHeaders(h) && (f == "hostname" || f == "model") {
					continue
				}
				first, covered := 0, false
				for _, col := range h[f] {
					if bom.ExtractCellValue(rows[row][col], f) == "" {
						continue
					}
					if first == 0 {
						first = col
					}
					if planMapsCell(p, rows[row], row, f, col) {
						covered = true
					}
				}
				// Alternate labels may denote the same endpoint. One explicit
				// binding identifies it; other source columns stay retained for review.
				if first > 0 && !covered {
					return fmt.Errorf("SOURCE_FIELD_UNCOVERED: row=%d column=%d field=%s; inspect this region and preserve the explicit source field", row, first, f)
				}
			}
		}
	}

	return nil
}

func tableRowPresent(t TablePlan, cells map[int]bom.Cell) bool {
	// A missing A end does not erase a row with an explicit B end. Such a
	// cable remains visible and blocked until its missing endpoint is supplied.
	if t.Kind == "physical_connection" {
		for _, f := range []string{"endpoint_a_device", "endpoint_b_device", "endpoint_a_uid", "endpoint_b_uid"} {
			if bom.ExtractCellValue(cells[t.Fields[f]], f) != "" {
				return true
			}
		}
		return false
	}
	for _, f := range t.Required {
		if bom.ExtractCellValue(cells[t.Fields[f]], f) == "" {
			return false
		}
	}
	for f, col := range t.Fields {
		if bom.ExtractCellValue(cells[col], f) != "" {
			return true
		}
	}
	return false
}

// The local extractor and independent checker must use the same row predicate.
func TableRowPresent(t TablePlan, cells map[int]bom.Cell) bool { return tableRowPresent(t, cells) }

func tableTypeMatches(t TablePlan, cells map[int]bom.Cell) bool {
	if t.TypeColumn == 0 {
		return true
	}
	_, ok := t.Types[strings.TrimSpace(cells[t.TypeColumn].Value)]
	return ok
}

func planMapsCell(p SheetPlan, cells map[int]bom.Cell, row int, field string, col int) bool {
	for _, t := range p.Tables {
		if t.Kind != "reference" && row >= t.Start && row <= t.End && (t.Fields[field] == col || field == "category" && t.TypeColumn == col) && tableRowPresent(t, cells) && tableTypeMatches(t, cells) {
			return true
		}
	}
	return false
}

// Previously extracted, source-backed fields may not disappear on retry. A
// correction that removes an established binding needs explicit human review.
func CheckPlanRevision(s bom.Sheet, next SheetPlan, previous *SheetPlan) error {
	if e := inspectExtraction(s, next); e != nil {
		return e
	}
	if previous == nil {
		return nil
	}
	rows := bom.MergedSourceCells(s)
	for _, t := range previous.Tables {
		if t.Kind == "reference" {
			continue
		}
		fields := []string{}
		for f := range t.Fields {
			fields = append(fields, f)
		}
		sort.Strings(fields)
		for row := t.Start; row <= t.End; row++ {
			if !tableRowPresent(t, rows[row]) || !tableTypeMatches(t, rows[row]) {
				continue
			}
			for _, f := range fields {
				col := t.Fields[f]
				if bom.ExtractCellValue(rows[row][col], f) != "" && !planMapsCell(next, rows[row], row, f, col) {
					return fmt.Errorf("EXTRACTION_REGRESSION: row=%d column=%d field=%s; preserve the previous source binding or leave this attempt failed for manual review", row, col, f)
				}
			}
		}
	}
	return nil
}

// Source coordinates, not AI-generated table names or categories, identify a candidate.
func SourceAnchor(fields map[string]int) string {
	for _, group := range [][]string{{"endpoint_a_device", "endpoint_a_port", "endpoint_b_device", "endpoint_b_port"}, {"parent_uid", "port_index"}, {"hostname"}, {"serial"}, {"model"}, {"name"}} {
		parts := []string{}
		for _, field := range group {
			if fields[field] <= 0 {
				parts = nil
				break
			}
			parts = append(parts, fmt.Sprint(field, "=", fields[field]))
		}
		if len(parts) > 0 {
			return strings.Join(parts, ",")
		}
	}
	parts := []string{}
	for field, col := range fields {
		parts = append(parts, fmt.Sprint(field, "=", col))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func dualEndpointHeaders(h map[string][]int) bool {
	return len(h["hostname"]) == 2 && len(h["port_index"]) == 2
}
