package bom

import (
	"bytes"
	"github.com/xuri/excelize/v2"
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
