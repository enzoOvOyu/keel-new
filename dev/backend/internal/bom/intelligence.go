package bom

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"regexp"
	"sort"
	"strings"
	"time"
)

// WorkbookIR is the only extraction source. Regions are structural, never assets.
type Region struct {
	ID          string `json:"id"`
	Sheet       string `json:"sheet_name"`
	Range       string `json:"range"`
	Start       int    `json:"start_row"`
	End         int    `json:"end_row"`
	FirstColumn int    `json:"first_column"`
	LastColumn  int    `json:"last_column"`
	Cells       int    `json:"cells"`
}
type AgentStep struct {
	Tool   string `json:"tool"`
	Sheet  string `json:"sheet_name,omitempty"`
	Region string `json:"region,omitempty"`
	State  string `json:"state"`
	Detail string `json:"detail"`
	At     string `json:"at"`
}
type RegionCoverage struct {
	Region
	State          string `json:"state"`
	ExtractedCells int    `json:"extracted_cells"`
	UnmappedCells  int    `json:"unmapped_cells"`
	RetainedCells  int    `json:"retained_cells"`
	Candidates     int    `json:"candidates"`
	Confirmed      int    `json:"confirmed"`
	Unrecognized   int    `json:"unrecognized"`
	Ignored        int    `json:"ignored"`
}
type CoverageAudit struct {
	Complete       bool             `json:"complete"`
	TotalCells     int              `json:"total_cells"`
	RetainedCells  int              `json:"retained_cells"`
	ExtractedCells int              `json:"extracted_cells"`
	Regions        []RegionCoverage `json:"regions"`
}
type WorkbookIR struct {
	Workbook Workbook
	Regions  []Region
	bySheet  map[string]Sheet
}

func NewWorkbookIR(w Workbook) *WorkbookIR {
	ir := &WorkbookIR{Workbook: w, bySheet: map[string]Sheet{}, Regions: []Region{}}
	for _, s := range w.Sheets {
		ir.bySheet[s.Name] = s
		byRow := map[int][]Cell{}
		order := []int{}
		for _, c := range s.Cells {
			if c.Sensitive || (c.Value == "" && c.Formula == "" && !c.FormulaBacked) {
				continue
			}
			if _, ok := byRow[c.Row]; !ok {
				order = append(order, c.Row)
			}
			byRow[c.Row] = append(byRow[c.Row], c)
		}
		sort.Ints(order)
		var region *Region
		for _, row := range order {
			// Blank separators and bounded chunks expose later tables, not only the first rows.
			if region == nil || row > region.End+1 || row-region.Start >= 256 {
				if region != nil {
					ir.Regions = append(ir.Regions, *region)
				}
				region = &Region{ID: fmt.Sprintf("%s:%d", s.Name, row), Sheet: s.Name, Start: row, End: row, FirstColumn: 513}
			}
			region.End = row
			for _, c := range byRow[row] {
				region.Cells++
				if c.Column < region.FirstColumn {
					region.FirstColumn = c.Column
				}
				if c.Column > region.LastColumn {
					region.LastColumn = c.Column
				}
			}
			a, _ := excelize.CoordinatesToCellName(region.FirstColumn, region.Start)
			z, _ := excelize.CoordinatesToCellName(region.LastColumn, region.End)
			region.Range = a + ":" + z
		}
		if region != nil {
			ir.Regions = append(ir.Regions, *region)
		}
	}
	return ir
}
func (ir *WorkbookIR) InspectWorkbook() []Region { return ir.Regions }
func (ir *WorkbookIR) AnalyzeSheet(name string) (Sheet, bool) {
	s, ok := ir.bySheet[name]
	return s, ok
}
func (ir *WorkbookIR) InspectRegion(r Region) []Cell {
	s := ir.bySheet[r.Sheet]
	out := []Cell{}
	for _, c := range s.Cells {
		if !c.Sensitive && c.Row >= r.Start && c.Row <= r.End && c.Column >= r.FirstColumn && c.Column <= r.LastColumn {
			out = append(out, c)
		}
	}
	return out
}

var sheetReference = regexp.MustCompile(`(?:'([^']+)'|([\p{L}\p{N}_ -]+))!\$?([A-Z]+)\$?([0-9]+)`)

// Follow real formula references locally. They are evidence, not a source of invented IDs.
func (ir *WorkbookIR) CrossSheetResolve(name string) []Cell {
	out := []Cell{}
	seen := map[string]bool{}
	wanted := map[string]bool{}
	for _, c := range ir.bySheet[name].Cells {
		for _, m := range sheetReference.FindAllStringSubmatch(c.Formula, -1) {
			sheet := m[1]
			if sheet == "" {
				sheet = m[2]
			}
			wanted[sheet+":"+m[3]+m[4]] = true
		}
	}
	index := map[string]Cell{}
	for _, s := range ir.Workbook.Sheets {
		for _, c := range s.Cells {
			if wanted[s.Name+":"+c.Ref] {
				index[s.Name+":"+c.Ref] = c
			}
		}
	}
	for _, c := range ir.bySheet[name].Cells {
		for _, m := range sheetReference.FindAllStringSubmatch(c.Formula, -1) {
			sheet := m[1]
			if sheet == "" {
				sheet = strings.TrimSpace(m[2])
			}
			key := sheet + ":" + m[3] + m[4]
			if cell, ok := index[key]; ok && !cell.Sensitive && !seen[key] {
				seen[key] = true
				out = append(out, cell)
			}
		}
	}
	return out
}
func (b *Workbench) Trace(tool, sheet, region, state, detail string) {
	b.Steps = append(b.Steps, AgentStep{tool, sheet, region, state, detail, time.Now().UTC().Format(time.RFC3339)})
}

// Independent accounting of every non-sensitive nonempty cell, including values
// that the model did not classify. Retention is NOT extraction accuracy.
func (ir *WorkbookIR) AuditCoverage(b *Workbench) CoverageAudit {
	extracted := map[string]bool{}
	retained := map[string]bool{}
	byRow := map[string]map[int]bool{}
	states := map[string]string{}
	for _, s := range b.Sheets {
		states[s.Name] = s.State
	}
	for i, r := range b.Records {
		for _, c := range r.Raw {
			key := c.Sheet + ":" + c.Ref
			retained[key] = true
			rowKey := fmt.Sprintf("%s:%d", c.Sheet, c.Row)
			if byRow[rowKey] == nil {
				byRow[rowKey] = map[int]bool{}
			}
			byRow[rowKey][i] = true
		}
		for _, c := range r.Item.Sources {
			if c.Ref != "" {
				extracted[c.Sheet+":"+c.Ref] = true
			}
		}
	}
	audit := CoverageAudit{Complete: true, Regions: []RegionCoverage{}}
	for _, r := range ir.Regions {
		c := RegionCoverage{Region: r, State: "pending"}
		seen := map[int]bool{}
		for _, cell := range ir.InspectRegion(r) {
			if cell.Value == "" && cell.Formula == "" && !cell.FormulaBacked {
				continue
			}
			key := cell.Sheet + ":" + cell.Ref
			if extracted[key] {
				c.ExtractedCells++
			}
			if retained[key] {
				c.RetainedCells++
			}
			for i := range byRow[fmt.Sprintf("%s:%d", cell.Sheet, cell.Row)] {
				seen[i] = true
			}
		}
		c.UnmappedCells = c.Cells - c.ExtractedCells
		for i := range seen {
			rec := b.Records[i]
			c.Candidates++
			if rec.Confirmed {
				c.Confirmed++
			}
			if rec.Ignored {
				c.Ignored++
			}
			if rec.Recognition == "unrecognized" {
				c.Unrecognized++
			}
		}
		if states[r.Sheet] == "recognized" {
			c.State = "review_required"
			if c.UnmappedCells == 0 {
				c.State = "extracted"
			}
		} else {
			audit.Complete = false
		}
		if c.RetainedCells != c.Cells {
			c.State = "retention_gap"
			audit.Complete = false
		}
		audit.TotalCells += c.Cells
		audit.RetainedCells += c.RetainedCells
		audit.ExtractedCells += c.ExtractedCells
		audit.Regions = append(audit.Regions, c)
	}
	return audit
}

// Merged ownership is only propagated inside an explicit merge, with the anchor
// kept as provenance. Never fill down over ordinary blank cells.
func MergedSourceCells(s Sheet) map[int]map[int]Cell {
	out := map[int]map[int]Cell{}
	for _, c := range s.Cells {
		if !c.Sensitive {
			if out[c.Row] == nil {
				out[c.Row] = map[int]Cell{}
			}
			out[c.Row][c.Column] = c
		}
	}
	for _, m := range s.Merges {
		p := strings.Split(m, ":")
		if len(p) != 2 {
			continue
		}
		a, r, e := excelize.CellNameToCoordinates(p[0])
		z, t, e2 := excelize.CellNameToCoordinates(p[1])
		if e != nil || e2 != nil {
			continue
		}
		anchor := out[r][a]
		if anchor.Value == "" {
			continue
		}
		for row := r; row <= t; row++ {
			if out[row] == nil {
				out[row] = map[int]Cell{}
			}
			for col := a; col <= z; col++ {
				if out[row][col].Value == "" {
					out[row][col] = anchor
				}
			}
		}
	}
	return out
}

// Human disposition changes update review counters without rerunning AI or parsing.
func RefreshCoverageReview(b *Workbench) {
	if b.Coverage == nil {
		return
	}
	for j := range b.Coverage.Regions {
		region := &b.Coverage.Regions[j]
		region.Confirmed = 0
		region.Ignored = 0
		region.Unrecognized = 0
		for _, r := range b.Records {
			inside := false
			for _, c := range r.Raw {
				if c.Sheet == region.Sheet && c.Row >= region.Start && c.Row <= region.End {
					inside = true
					break
				}
			}
			if !inside {
				continue
			}
			if r.Confirmed {
				region.Confirmed++
			}
			if r.Ignored {
				region.Ignored++
			}
			if r.Recognition == "unrecognized" {
				region.Unrecognized++
			}
		}
	}
}
