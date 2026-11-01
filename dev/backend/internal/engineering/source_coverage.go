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
		return checkRecognizedTableCoverage(w, p, sheet)
	}
	start, _ := excelize.CoordinatesToCellName(minCol, minRow)
	end, _ := excelize.CoordinatesToCellName(maxCol, maxRow)
	return fmt.Errorf("INCOMPLETE_SHEET_COVERAGE: sheet=%s missing_cells=%d uncovered_bounds=%s:%s; extend mappings to all data rows and columns, or retain unknown areas as other regions; preview is not the full table", sheet, missing, start, end)
}

// Strong, explicit header groups must not be silently downgraded to "other".
// This only checks recognized evidence; unfamiliar customer wording is still
// interpreted by the Agent, and no rows or domain objects are generated here.
func checkRecognizedTableCoverage(w bom.Workbook, p EngineeringProposal, sheet string) error {
	for _, s := range w.Sheets {
		if s.Name != sheet {
			continue
		}
		rows := map[int]map[string][]int{}
		for _, c := range s.Cells {
			if c.Sensitive {
				continue
			}
			for _, field := range HeaderFields(c.Value) {
				if rows[c.Row] == nil {
					rows[c.Row] = map[string][]int{}
				}
				rows[c.Row][field] = append(rows[c.Row][field], c.Column)
			}
		}
		for row, fields := range rows {
			required := []string{}
			kinds := map[string]bool{}
			if len(fields["a_device"]) > 0 && len(fields["a_port"]) > 0 && len(fields["z_device"]) > 0 && len(fields["z_port"]) > 0 {
				required = []string{"a_device", "a_port", "z_device", "z_port", "a_optic", "z_optic", "cable_model", "length"}
				kinds["connection"] = true
				kinds["cable"] = true
			} else if len(fields["label"]) > 0 && len(fields["model"]) > 0 && (len(fields["rack"]) > 0 || len(fields["serial"]) > 0) {
				required = []string{"label", "model", "rack", "start_u"}
				kinds["equipment"] = true
				kinds["placement"] = true
			}
			for _, field := range required {
				// An ambiguous duplicate header needs semantic review, not forced selection.
				if len(fields[field]) != 1 {
					continue
				}
				column := fields[field][0]
				mapped := false
				for _, r := range p.Regions {
					if r.Sheet != sheet || !kinds[r.Kind] || row > r.Bounds.EndRow || column < r.Bounds.StartColumn || column > r.Bounds.EndColumn {
						continue
					}
					for _, c := range r.Columns {
						if c.Field == field && c.Column == column {
							mapped = true
						}
					}
				}
				if !mapped {
					return fmt.Errorf("SUPPORTED_HEADER_UNMAPPED: sheet=%s header_row=%d column=%d field=%s; explicit supported table evidence cannot be discarded as other; map the evidenced column across all data rows", sheet, row, column, field)
				}
			}
		}
	}
	return nil
}
