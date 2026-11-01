package engineering

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// StandardTableData belongs to the versioned standard-input contract. Arbitrary
// source workbooks go through Intelligence rather than this strict reader.
type StandardTableData struct {
	TableID string
	Rows    [][]string
}

func WriteStepWorkbook(step int, tables []StandardTableData) ([]byte, error) {
	catalog := StepInputCatalog()
	definitions := map[string]InputTable{}
	for _, t := range catalog.Tables {
		if t.Step == step {
			definitions[t.ID] = t
		}
	}
	if len(definitions) == 0 {
		return nil, fmt.Errorf("STEP_UNKNOWN")
	}
	f := excelize.NewFile()
	defer f.Close()
	style, e := f.NewStyle(&excelize.Style{NumFmt: 49})
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	for _, data := range tables {
		t, ok := definitions[data.TableID]
		if !ok || seen[t.ID] {
			return nil, fmt.Errorf("STEP_TABLE_INVALID: %s", data.TableID)
		}
		seen[t.ID] = true
		if _, e = f.NewSheet(t.ID); e != nil {
			return nil, e
		}
		last, _ := excelize.ColumnNumberToName(len(t.Fields))
		if e = f.SetColStyle(t.ID, "A:"+last, style); e != nil {
			return nil, e
		}
		for col, field := range t.Fields {
			cell, _ := excelize.CoordinatesToCellName(col+1, 1)
			if e = f.SetCellStr(t.ID, cell, field.Label); e != nil {
				return nil, e
			}
		}
		for row, values := range data.Rows {
			if len(values) != len(t.Fields) {
				return nil, fmt.Errorf("STEP_COLUMN_COUNT: %s", t.ID)
			}
			for col, value := range values {
				cell, _ := excelize.CoordinatesToCellName(col+1, row+2)
				if e = f.SetCellStr(t.ID, cell, value); e != nil {
					return nil, e
				}
			}
		}
	}
	if len(tables) == 0 {
		return nil, fmt.Errorf("STEP_EMPTY_WORKBOOK")
	}
	if e = f.DeleteSheet("Sheet1"); e != nil {
		return nil, e
	}
	var b bytes.Buffer
	_, e = f.WriteTo(&b)
	return b.Bytes(), e
}

// ReadStepWorkbook preserves strings and rejects formulas whose cached values
// have not passed the calculation provider. No unknown nonempty sheet is lost.
func ReadStepWorkbook(input io.Reader, step int) ([]InputPreviewRequest, error) {
	return readStepWorkbook(input, step, true)
}

// ReadStepWorkbookCandidates retains missing values so the workbench can show
// field-level problems. Unknown sheets and unverified formulas still fail.
func ReadStepWorkbookCandidates(input io.Reader, step int) ([]InputPreviewRequest, error) {
	return readStepWorkbook(input, step, false)
}
func readStepWorkbook(input io.Reader, step int, strict bool) ([]InputPreviewRequest, error) {
	f, e := excelize.OpenReader(input)
	if e != nil {
		return nil, fmt.Errorf("STEP_XLSX_INVALID")
	}
	defer f.Close()
	definitions := map[string]InputTable{}
	for _, t := range StepInputCatalog().Tables {
		if t.Step == step {
			definitions[t.ID] = t
		}
	}
	if len(definitions) == 0 {
		return nil, fmt.Errorf("STEP_UNKNOWN")
	}
	out := []InputPreviewRequest{}
	for _, sheet := range f.GetSheetList() {
		rows, e := f.GetRows(sheet, excelize.Options{RawCellValue: true})
		if e != nil {
			return nil, e
		}
		if len(rows) == 0 {
			continue
		}
		t, ok := definitions[sheet]
		if !ok {
			return nil, fmt.Errorf("STEP_UNKNOWN_SHEET: %s", sheet)
		}
		var b strings.Builder
		w := csv.NewWriter(&b)
		for row, values := range rows {
			normalized := make([]string, len(t.Fields))
			copy(normalized, values)
			if len(values) > len(normalized) {
				return nil, fmt.Errorf("STEP_EXTRA_COLUMNS: %s", sheet)
			}
			for col := range normalized {
				cell, _ := excelize.CoordinatesToCellName(col+1, row+1)
				formula, e := f.GetCellFormula(sheet, cell)
				if e != nil {
					return nil, e
				}
				if formula != "" {
					return nil, fmt.Errorf("STEP_FORMULA_UNVERIFIED: %s!%s", sheet, cell)
				}
			}
			if e = w.Write(normalized); e != nil {
				return nil, e
			}
		}
		w.Flush()
		if e = w.Error(); e != nil {
			return nil, e
		}
		q := InputPreviewRequest{TableID: t.ID, CSV: b.String()}
		preview := PreviewStepInput(q)
		if strict && !preview.Valid {
			return nil, fmt.Errorf("STEP_INPUT_INVALID: %s (%d issues)", sheet, len(preview.Issues))
		}
		out = append(out, q)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("STEP_EMPTY_WORKBOOK")
	}
	return out, nil
}
