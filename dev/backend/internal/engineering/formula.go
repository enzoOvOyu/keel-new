package engineering

import (
	"fmt"
	"github.com/xuri/efp"
	"github.com/xuri/excelize/v2"
	"keel.local/dev/internal/bom"
	"math"
	"sort"
	"strconv"
	"strings"
)

// FormulaGraph tokenizes expressions: quoted text is never mistaken for a cell
// reference. Dynamic, external and unresolved references remain explicit.
func FormulaGraph(w bom.Workbook, names []excelize.DefinedName) []FormulaEvidence {
	sheets := map[string]bool{}
	for _, s := range w.Sheets {
		sheets[s.Name] = true
	}
	named := map[string]string{}
	for _, n := range names {
		named[n.Scope+"!"+strings.ToLower(n.Name)] = n.RefersTo
	}
	out := []FormulaEvidence{}
	for _, sheet := range w.Sheets {
		for _, cell := range sheet.Cells {
			if cell.Formula == "" || cell.Sensitive {
				continue
			}
			ev := FormulaEvidence{Cell: FormulaCell{SheetID: sheet.Name, Address: cell.Ref}, Expression: cell.Formula, Dependencies: []FormulaDependency{}, DependencyStatus: "complete", Compatibility: "unknown"}
			if cell.Value != "" {
				ev.CachedValue = SpreadsheetValue(cell.Value)
			}
			seen := map[string]bool{}
			var parse func(string, string, int)
			parse = func(expression, contextSheet string, depth int) {
				if depth > 16 {
					ev.DependencyStatus = "partial"
					return
				}
				parser := efp.ExcelParser()
				tokens := parser.Parse(expression)
				for _, tok := range tokens {
					if tok.TType == efp.TokenTypeFunction && tok.TSubType == efp.TokenSubTypeStart {
						switch strings.ToUpper(strings.TrimPrefix(tok.TValue, "_xlfn.")) {
						case "INDIRECT", "OFFSET", "WEBSERVICE", "RTD":
							ev.DependencyStatus = "partial"
							ev.Dependencies = append(ev.Dependencies, FormulaDependency{From: ev.Cell, ToSheetID: contextSheet, ToRange: tok.TValue, Kind: "dynamic"})
						}
					}
					if tok.TType != efp.TokenTypeOperand || tok.TSubType != efp.TokenSubTypeRange {
						continue
					}
					ref := tok.TValue
					target := contextSheet
					if strings.ContainsAny(ref, "[]") {
						ev.DependencyStatus = "partial"
						ev.Dependencies = append(ev.Dependencies, FormulaDependency{From: ev.Cell, ToSheetID: target, ToRange: ref, Kind: "external_or_structured"})
						continue
					}
					if idx := strings.LastIndex(ref, "!"); idx >= 0 {
						target = strings.ReplaceAll(strings.Trim(ref[:idx], "'"), "''", "'")
						ref = ref[idx+1:]
					}
					key := target + "!" + strings.ToLower(ref)
					definition, ok := named[key]
					if !ok {
						definition, ok = named["Workbook!"+strings.ToLower(ref)]
					}
					if !ok {
						definition, ok = named["!"+strings.ToLower(ref)]
					}
					if ok {
						if seen["name:"+key] {
							ev.DependencyStatus = "partial"
							continue
						}
						seen["name:"+key] = true
						parse(definition, target, depth+1)
						delete(seen, "name:"+key)
						continue
					}
					kind := "range"
					clean := strings.ReplaceAll(ref, "$", "")
					if _, ok := referenceBounds(clean); !ok {
						kind = "unresolved"
						ev.DependencyStatus = "partial"
					} else if !sheets[target] {
						kind = "missing_sheet"
						ev.DependencyStatus = "missing"
					} else if !strings.Contains(clean, ":") {
						kind = "cell"
					}
					if !seen[key] {
						ev.Dependencies = append(ev.Dependencies, FormulaDependency{From: ev.Cell, ToSheetID: target, ToRange: clean, Kind: kind})
						seen[key] = true
					}
				}
			}
			parse(cell.Formula, sheet.Name, 0)
			out = append(out, ev)
		}
	}
	// Index formula cells per sheet by row. A range edge only visits matching
	// rows, avoiding a full workbook scan for each dependency.
	type position struct{ row, col, index int }
	index := map[string][]position{}
	for i, e := range out {
		col, row, err := excelize.CellNameToCoordinates(e.Cell.Address)
		if err == nil {
			index[e.Cell.SheetID] = append(index[e.Cell.SheetID], position{row, col, i})
		}
	}
	for s := range index {
		sort.Slice(index[s], func(i, j int) bool { return index[s][i].row < index[s][j].row })
	}
	edges := make([][]int, len(out))
	for i, e := range out {
		for _, d := range e.Dependencies {
			if d.Kind != "cell" && d.Kind != "range" {
				continue
			}
			b, ok := referenceBounds(d.ToRange)
			if !ok {
				continue
			}
			positions := index[d.ToSheetID]
			start := sort.Search(len(positions), func(j int) bool { return positions[j].row >= b.StartRow })
			for j := start; j < len(positions) && positions[j].row <= b.EndRow; j++ {
				p := positions[j]
				if p.col >= b.StartColumn && p.col <= b.EndColumn {
					edges[i] = append(edges[i], p.index)
				}
			}
		}
	}
	state := make([]uint8, len(out))
	stack := []int{}
	stackIndex := map[int]int{}
	var visit func(int)
	visit = func(i int) {
		if state[i] == 2 {
			return
		}
		if state[i] == 1 {
			for _, j := range stack[stackIndex[i]:] {
				out[j].DependencyStatus = "cycle"
			}
			return
		}
		state[i] = 1
		stackIndex[i] = len(stack)
		stack = append(stack, i)
		for _, j := range edges[i] {
			visit(j)
			if out[j].DependencyStatus != "complete" && out[i].DependencyStatus == "complete" {
				out[i].DependencyStatus = "partial"
			}
		}
		stack = stack[:len(stack)-1]
		delete(stackIndex, i)
		state[i] = 2
	}
	for i := range out {
		visit(i)
	}
	return out
}

func referenceBounds(ref string) (Bounds, bool) {
	parts := strings.Split(ref, ":")
	if len(parts) > 2 {
		return Bounds{}, false
	}
	if len(parts) == 1 {
		col, row, err := excelize.CellNameToCoordinates(parts[0])
		return Bounds{StartRow: row, EndRow: row, StartColumn: col, EndColumn: col}, err == nil
	}
	point := func(s string, end bool) (int, int, error) {
		if col, row, e := excelize.CellNameToCoordinates(s); e == nil {
			return col, row, nil
		}
		if row, e := strconv.Atoi(s); e == nil && row > 0 && row <= 1048576 {
			col := 1
			if end {
				col = 16384
			}
			return col, row, nil
		}
		if col, e := excelize.ColumnNameToNumber(s); e == nil && col <= 16384 {
			row := 1
			if end {
				row = 1048576
			}
			return col, row, nil
		}
		return 0, 0, fmt.Errorf("UNSUPPORTED_RANGE")
	}
	c1, r1, e1 := point(parts[0], false)
	c2, r2, e2 := point(parts[1], true)
	return Bounds{StartRow: r1, EndRow: r2, StartColumn: c1, EndColumn: c2}, e1 == nil && e2 == nil && c2 >= c1 && r2 >= r1
}

func SpreadsheetValue(s string) *TypedValue {
	if strings.ContainsAny(s, "eE") {
		if n, err := strconv.ParseFloat(s, 64); err == nil && !math.IsInf(n, 0) && !math.IsNaN(n) {
			s = strconv.FormatFloat(n, 'f', -1, 64)
		}
	}
	if decimal(s) {
		unit := "source"
		return &TypedValue{Kind: "decimal", Decimal: &s, Unit: &unit}
	}
	if strings.EqualFold(s, "true") || strings.EqualFold(s, "false") {
		v := strings.EqualFold(s, "true")
		return &TypedValue{Kind: "boolean", Boolean: &v}
	}
	return &TypedValue{Kind: "text", Text: &s}
}
