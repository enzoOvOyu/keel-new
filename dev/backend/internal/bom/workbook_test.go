package bom

import (
	"archive/zip"
	"bytes"
	"github.com/xuri/excelize/v2"
	"io"
	"testing"
)

func TestWorkbookSafetyAndProvenance(t *testing.T) {
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "SN")
	f.SetCellValue("Sheet1", "A2", "synthetic-1")
	f.SetCellValue("Sheet1", "B1", "Password")
	f.SetCellValue("Sheet1", "B2", "DO_NOT_EXPOSE")
	f.MergeCell("Sheet1", "C1", "D1")
	var b bytes.Buffer
	if e := f.Write(&b); e != nil {
		t.Fatal(e)
	}
	w, e := Parse(b.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	if len(w.Sheets) != 1 || len(w.Sheets[0].Merges) != 1 {
		t.Fatal("structure lost")
	}
	for _, c := range w.Sheets[0].Cells {
		if c.Value == "DO_NOT_EXPOSE" {
			t.Fatal("secret leaked")
		}
		if c.Ref == "A2" && c.Row != 2 {
			t.Fatal("source lost")
		}
	}
}
func TestValidationRejectsUnsafeRelations(t *testing.T) {
	items := []Item{{UID: "c", Category: "cable", A: "missing", B: "missing"}}
	if len(Validate(items)) == 0 {
		t.Fatal("missing endpoints passed")
	}
	if StableID("switch:SN") != StableID("switch:SN") {
		t.Fatal("unstable identity")
	}
}
func TestNormalizePreservesEndpointGraph(t *testing.T) {
	items := []Item{{UID: "d1", Category: "server", Serial: "S1"}, {UID: "d2", Category: "switch", Serial: "S2"}, {UID: "p1", Category: "port", Parent: "d1", Port: "1"}, {UID: "p2", Category: "port", Parent: "d2", Port: "1"}, {UID: "c", Category: "cable", A: "p1", B: "p2"}}
	out, e := Normalize(items)
	if e != nil {
		t.Fatal(e)
	}
	if out[2].Parent != out[0].UID || out[4].A != out[2].UID || out[4].B != out[3].UID {
		t.Fatal("reference graph broken")
	}
}
func TestUnknownSheetBlocksAndReconcileUsesSerial(t *testing.T) {
	w := Workbook{Sheets: []Sheet{{Name: "unknown"}}}
	_, issues := Build(w, Profiles(w))
	if len(issues) == 0 {
		t.Fatal("unknown Sheet silently dropped")
	}
	items := []Item{{UID: "a", Category: "server", Serial: "S", Fields: map[string]string{"serial": "S"}, Sources: map[string]Cell{}}, {UID: "b", Category: "server", Serial: "S", Fields: map[string]string{"serial": "S"}, Sources: map[string]Cell{}}}
	merged, issues := Reconcile(items)
	if len(merged) != 1 || len(issues) > 0 {
		t.Fatal("stable identity not merged")
	}
}
func TestUncachedFormulaIsPreservedNotExecuted(t *testing.T) {
	f := excelize.NewFile()
	f.SetCellFormula("Sheet1", "A1", "1+2")
	var b bytes.Buffer
	f.Write(&b)
	w, e := Parse(b.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	if len(w.Sheets[0].Cells) != 1 || w.Sheets[0].Cells[0].Formula != "1+2" || w.Sheets[0].Cells[0].Value != "" {
		t.Fatal("formula evaluated or missing-cache source lost")
	}
}
func TestSpeedsUseExplicitUnits(t *testing.T) {
	a, oa := Speed("100G")
	b, ob := Speed("100000 Mbps")
	if !oa || !ob || a != b {
		t.Fatal("unit conversion failed")
	}
	if _, ok := Speed("100"); ok {
		t.Fatal("unknown unit guessed")
	}
}
func TestUnknownIdentityIsNotDuplicateEvidence(t *testing.T) {
	issues := Validate([]Item{{UID: "a", Category: "other"}, {UID: "b", Category: "other"}})
	for _, i := range issues {
		if i.Rule == "BOM-IDENTITY" {
			t.Fatal("missing identity masqueraded as duplicate evidence")
		}
	}
}
func TestStandardPackageIncludesRequiredBOMFields(t *testing.T) {
	data, e := Package([]Item{{UID: "synthetic", Category: "server", Serial: "S", Status: "imported", Fields: map[string]string{}, Sources: map[string]Cell{}}}, "batch", "project")
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		t.Fatal(e)
	}
	if len(z.File) != 8 {
		t.Fatal("seven CSV files and manifest required")
	}
	for _, f := range z.File {
		if f.Name == "server/servers.csv" {
			r, _ := f.Open()
			b, _ := io.ReadAll(r)
			r.Close()
			if !bytes.Contains(b, []byte("source_of_record,status")) {
				t.Fatal("required BOM status missing")
			}
		}
	}
}
func TestApprovedTemplateTracksReorderedHeaders(t *testing.T) {
	w := Workbook{Sheets: []Sheet{{Name: "assets", Cells: []Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Hostname", Sheet: "assets"}, {Ref: "B1", Row: 1, Column: 2, Value: "SN", Sheet: "assets"}, {Ref: "A2", Row: 2, Column: 1, Value: "synthetic-host", Sheet: "assets"}, {Ref: "B2", Row: 2, Column: 2, Value: "synthetic-serial", Sheet: "assets"}}}}}
	saved := Profile{Sheet: "assets", Purpose: "assets", Entity: "server", Columns: []Column{{Index: 1, Header: "SN", Field: "serial"}, {Index: 2, Header: "Hostname", Field: "hostname"}}, Confirmed: true}
	out := ApplyTemplate(w, Profiles(w), []Profile{saved}, "synthetic/v1")
	if !out[0].Confirmed || out[0].Columns[0].Index != 2 || out[0].Columns[1].Index != 1 {
		t.Fatal("template was tied to original column position")
	}
}

func TestColumnLimitIgnoresWhitespaceTailButRejectsRealEvidence(t *testing.T) {
	for _, kind := range []string{"whitespace", "value", "formula"} {
		t.Run(kind, func(t *testing.T) {
			f := excelize.NewFile()
			defer f.Close()
			f.SetCellValue("Sheet1", "A1", "Hostname")
			f.SetCellValue("Sheet1", "A2", "synthetic-device")
			f.SetCellValue("Sheet1", "XFC2", " \t ")
			if kind == "value" {
				f.SetCellValue("Sheet1", "XFC2", "synthetic-important-value")
			}
			if kind == "formula" {
				f.SetCellFormula("Sheet1", "XFC2", "SUM(1,2)")
			}
			var data bytes.Buffer
			if err := f.Write(&data); err != nil {
				t.Fatal(err)
			}
			w, err := Parse(data.Bytes())
			if kind != "whitespace" {
				if err == nil || err.Error() != "COLUMN_LIMIT" {
					t.Fatal("meaningful over-limit evidence silently discarded", err)
				}
				return
			}
			if err != nil || len(w.Sheets) != 1 || len(w.Sheets[0].Cells) != 2 {
				t.Fatal("whitespace tail rejected or actual cells lost", err)
			}
		})
	}
}

func TestTokenDeviceNameDoesNotRedactInventoryColumn(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	for ref, value := range map[string]string{"A1": "设备名称", "A2": "synthetic-token-leaf-1", "A3": "synthetic-leaf-2", "B1": "设备型号", "B2": "synthetic-model", "C1": "API Token", "C2": "DO_NOT_EXPOSE_TOKEN"} {
		if err := f.SetCellValue("Sheet1", ref, value); err != nil {
			t.Fatal(err)
		}
	}
	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		t.Fatal(err)
	}
	w, err := Parse(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	kept := 0
	for _, c := range w.Sheets[0].Cells {
		if c.Column == 1 && !c.Sensitive && c.Value != "[REDACTED]" {
			kept++
		}
		if c.Column == 3 && (!c.Sensitive || c.Value != "[REDACTED]") {
			t.Fatal("credential column exposed")
		}
	}
	if kept != 3 {
		t.Fatal("device label erased inventory identity", kept)
	}
	for _, label := range []string{"token", "Access Token", "token: synthetic", "API_TOKEN", "token（凭据）", "令牌"} {
		if !Sensitive(label) {
			t.Fatalf("credential label not detected: %s", label)
		}
	}
}
