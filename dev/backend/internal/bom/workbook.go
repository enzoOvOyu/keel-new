package bom

import (
	"archive/zip"
	"bytes"
	"fmt"
	"github.com/xuri/excelize/v2"
	"io"
	"path"
	"strings"
)

type Cell struct {
	Ref       string `json:"source_cell"`
	Row       int    `json:"source_row"`
	Column    int    `json:"source_column"`
	Value     string `json:"original_value"`
	Formula   string `json:"formula,omitempty"`
	Hidden    bool   `json:"hidden"`
	Sensitive bool   `json:"sensitive"`
	Sheet     string `json:"sheet_name"`
}
type Sheet struct {
	Name      string   `json:"sheet_name"`
	Order     int      `json:"order"`
	Hidden    bool     `json:"hidden"`
	Purpose   string   `json:"purpose"`
	Dimension string   `json:"dimension"`
	Merges    []string `json:"merged_cells"`
	Cells     []Cell   `json:"source_cells"`
	Parser    string   `json:"parser_version"`
}
type Workbook struct {
	Sheets       []Sheet `json:"sheets"`
	Mode         string  `json:"mode"`
	AIConfigured bool    `json:"ai_configured"`
}

func Sensitive(s string) bool {
	s = strings.ToLower(s)
	for _, x := range []string{"password", "passwd", "密码", "secret", "credential", "token", "private key", "api key"} {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}
func Parse(data []byte) (Workbook, error) {
	result := Workbook{Mode: "rules", Sheets: []Sheet{}}
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		return result, fmt.Errorf("INVALID_WORKBOOK")
	}
	var total uint64
	seen := map[string]bool{}
	for _, f := range z.File {
		total += f.UncompressedSize64
		if total > 256<<20 || f.UncompressedSize64 > 64<<20 || path.Clean(strings.TrimSuffix(f.Name, "/")) != strings.TrimSuffix(f.Name, "/") || strings.HasPrefix(f.Name, "/") || strings.HasPrefix(f.Name, "../") || seen[f.Name] || f.Mode()&0o170000 == 0o120000 {
			return result, fmt.Errorf("UNSAFE_ARCHIVE")
		}
		seen[f.Name] = true
	}
	f, e := excelize.OpenReader(bytes.NewReader(data), excelize.Options{RawCellValue: true, UnzipSizeLimit: 256 << 20, UnzipXMLSizeLimit: 64 << 20})
	if e != nil {
		return result, fmt.Errorf("INVALID_WORKBOOK")
	}
	defer f.Close()
	if len(f.GetSheetList()) > 64 {
		return result, fmt.Errorf("SHEET_LIMIT")
	}
	for idx, name := range f.GetSheetList() {
		visible, _ := f.GetSheetVisible(name)
		dim, _ := f.GetSheetDimension(name)
		s := Sheet{Name: name, Order: idx, Hidden: !visible, Dimension: dim, Purpose: "unclassified", Parser: "excelize-structure/v2", Cells: []Cell{}, Merges: []string{}}
		merges, e := f.GetMergeCells(name)
		if e != nil {
			return result, e
		}
		for _, m := range merges {
			s.Merges = append(s.Merges, m.GetStartAxis()+":"+m.GetEndAxis())
		}
		rows, e := f.Rows(name)
		if e != nil {
			return result, e
		}
		r := 0
		sensitiveCols := map[int]bool{}
		for rows.Next() {
			r++
			if r > 20000 {
				rows.Close()
				return result, fmt.Errorf("ROW_LIMIT")
			}
			cols, e := rows.Columns(excelize.Options{RawCellValue: true})
			if e != nil {
				rows.Close()
				return result, e
			}
			if len(cols) > 512 {
				rows.Close()
				return result, fmt.Errorf("COLUMN_LIMIT")
			}
			for c, v := range cols {
				ref, _ := excelize.CoordinatesToCellName(c+1, r)
				formula, _ := f.GetCellFormula(name, ref)
				if v == "" && formula == "" {
					continue
				}
				if Sensitive(v) {
					sensitiveCols[c+1] = true
				}
				redact := sensitiveCols[c+1] || Sensitive(v)
				rv, _ := f.GetRowVisible(name, r)
				cn, _ := excelize.ColumnNumberToName(c + 1)
				cv, _ := f.GetColVisible(name, cn)
				if redact {
					v = "[REDACTED]"
					formula = ""
				}
				s.Cells = append(s.Cells, Cell{ref, r, c + 1, v, formula, !rv || !cv, redact, name})
			}
		}
		if e := rows.Error(); e != nil {
			rows.Close()
			return result, e
		}
		rows.Close()
		// Apply late-discovered sensitive columns to earlier rows too; cover key/value layouts.
		blocked := map[string]bool{}
		for _, c := range s.Cells {
			if c.Sensitive {
				blocked[fmt.Sprintf("%d:%d", c.Row, c.Column+1)] = true
			}
		}
		for j := range s.Cells {
			c := &s.Cells[j]
			if sensitiveCols[c.Column] || (blocked[fmt.Sprintf("%d:%d", c.Row, c.Column)] && Field(c.Value) == "") {
				c.Value = "[REDACTED]"
				c.Formula = ""
				c.Sensitive = true
			}
		}
		result.Sheets = append(result.Sheets, s)
	}
	return result, nil
}

var _ io.Reader
