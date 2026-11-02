package engineering

import (
	"bytes"
	"strings"
	"testing"
)

func TestModelingPackagePreservesPartialIdentityAndBasis(t *testing.T) {
	row := []string{"000123", "02:00:00:00:00:01", "eth1", "000456", "rack-01\nfront", "", "eth2", "fiber"}
	basis := ModelingPackageBasis{SourceID: "source-one", Revision: 7, Inputs: []ModelingInputVersion{{TableID: "terminations", Version: 3}}}
	raw, err := WriteModelingPackage(1, []StandardTableData{{TableID: "terminations", Rows: [][]string{row}}}, basis)
	if err != nil {
		t.Fatal(err)
	}
	inputs, got, err := ReadModelingPackage(bytes.NewReader(raw), 1)
	if err != nil || got == nil || got.SourceID != basis.SourceID || got.Revision != 7 || got.Inputs[0].Version != 3 {
		t.Fatalf("basis lost: %+v %v", got, err)
	}
	p := PreviewStepInput(inputs[0])
	if p.Valid || len(p.Rows) != 1 || p.Rows[0][0] != "000123" || p.Rows[0][4] != "rack-01\nfront" || p.Rows[0][5] != "" {
		t.Fatalf("partial evidence changed: %+v", p)
	}
	if _, _, err = ReadModelingPackage(bytes.NewReader(raw), 2); err == nil {
		t.Fatal("wrong domain accepted")
	}
}

func TestModelingRackCoordinatesUseDeclaredUnits(t *testing.T) {
	m, err := MergeStandardInput(EmptyPhysical("coordinate-project"), standardRequest("rooms", []string{"site", "Site", "room", "Room", "customer-axis", "m"}))
	if err != nil {
		t.Fatal(err)
	}
	q := standardRequest("racks", []string{"room", "rack", "Rack", "48", "800", "1200", "", "", "1.25", "-2", "0", "m", "90"})
	m, err = MergeStandardInput(m, q)
	if err != nil {
		t.Fatal(err)
	}
	r := m.Facilities[len(m.Facilities)-1]
	if r.XMM == nil || *r.XMM != "1250" || r.YMM == nil || *r.YMM != "-2000" || r.ZMM == nil || *r.ZMM != "0" || r.CoordinateFrameID == nil || *r.CoordinateFrameID != *m.Facilities[1].CoordinateFrameID {
		t.Fatalf("geometry mismatch: %+v", r)
	}
	q.CSV = strings.Replace(q.CSV, ",m,90", ",,90", 1)
	if _, err = MergeStandardInput(m, q); err == nil {
		t.Fatal("missing coordinate units guessed")
	}
}
