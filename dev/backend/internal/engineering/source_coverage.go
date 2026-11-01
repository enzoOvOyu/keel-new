package engineering

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"keel.local/dev/internal/bom"
)

// Finishing a sheet accounts for all meaningful source areas. Unknown areas can
// be explicit "other" regions; sampled rows must not masquerade as a whole table.
func CheckSheetCoverage(w bom.Workbook, p EngineeringProposal, sheet string) error {
	missing, minRow, minCol, maxRow, maxCol := 0, 0, 0, 0, 0
	for _, s := range w.Sheets {
		if s.Name != sheet {
			continue
		}
		for _, c := range s.Cells {
			if c.Sensitive || (c.Value == "" && c.Formula == "" && !c.FormulaBacked) {
				continue
			}
			covered := false
			for _, r := range p.Regions {
				b := r.Bounds
				if r.Sheet == sheet && c.Row >= b.StartRow && c.Row <= b.EndRow && c.Column >= b.StartColumn && c.Column <= b.EndColumn {
					covered = true
					break
				}
			}
			if covered {
				continue
			}
			missing++
			if minRow == 0 || c.Row < minRow {
				minRow = c.Row
			}
			if minCol == 0 || c.Column < minCol {
				minCol = c.Column
			}
			maxRow = max(maxRow, c.Row)
			maxCol = max(maxCol, c.Column)
		}
	}
	if missing == 0 {
		return nil
	}
	start, _ := excelize.CoordinatesToCellName(minCol, minRow)
	end, _ := excelize.CoordinatesToCellName(maxCol, maxRow)
	return fmt.Errorf("INCOMPLETE_SHEET_COVERAGE: sheet=%s missing_cells=%d uncovered_bounds=%s:%s; extend mappings to all data rows and columns, or retain unknown areas as other regions; preview is not the full table", sheet, missing, start, end)
}
