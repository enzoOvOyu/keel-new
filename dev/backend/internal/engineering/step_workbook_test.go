package engineering

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestFiveStepXLSXFixtures(t *testing.T) {
	names := []string{"01-equipment.xlsx", "02-placement.xlsx", "03-cabling.xlsx", "04-network.xlsx", "05-power.xlsx"}
	model := EmptyPhysical("fixture-project")
	for i, name := range names {
		b, e := os.ReadFile(filepath.Join("testdata", "five-step", name))
		if e != nil {
			t.Fatal(e)
		}
		inputs, e := ReadStepWorkbook(bytes.NewReader(b), i+1)
		if e != nil {
			t.Fatal(name, e)
		}
		if len(inputs) == 0 {
			t.Fatal("empty fixture", name)
		}
		// Steps 3-5 are format acceptance only until their domain mergers exist.
		if i < 2 {
			for _, q := range inputs {
				model, e = MergeStandardInput(model, q)
				if e != nil {
					t.Fatal(name, q.TableID, e)
				}
			}
		}
	}
	var expected struct {
		EquipmentCount  int `json:"equipment_count"`
		PortCount       int `json:"port_count"`
		ConnectionCount int `json:"connection_count"`
		RackCount       int `json:"rack_count"`
		Placements      map[string]struct {
			Rack    string `json:"rack"`
			StartU  int    `json:"start_u"`
			HeightU int    `json:"height_u"`
		} `json:"placements"`
	}
	raw, e := os.ReadFile(filepath.Join("testdata", "five-step", "expected.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &expected); e != nil {
		t.Fatal(e)
	}
	if len(model.Equipment) != expected.EquipmentCount || len(model.Ports) != expected.PortCount || len(model.Connections) != expected.ConnectionCount {
		t.Fatal("identity or connection counts differ from independent expectations")
	}
	racks := map[string]string{}
	for _, f := range model.Facilities {
		if f.Kind == "rack" {
			racks[f.EntityID] = f.Code
		}
	}
	if len(racks) != expected.RackCount {
		t.Fatal("rack count differs")
	}
	placed := map[string]bool{}
	for _, p := range model.Placements {
		serial := ""
		for _, eq := range model.Equipment {
			if eq.EntityID == p.EquipmentID && eq.Serial != nil {
				serial = *eq.Serial
			}
		}
		want, ok := expected.Placements[serial]
		if !ok || p.StartU == nil || p.HeightU == nil || racks[p.RackID] != want.Rack || *p.StartU != want.StartU || *p.HeightU != want.HeightU {
			t.Fatal("placement differs from independent expectations", serial)
		}
		placed[serial] = true
	}
	if len(placed) != len(expected.Placements) {
		t.Fatal("missing placement")
	}
}

func TestStandardXLSXPreservesIdentityAndRejectsUnverifiedFormula(t *testing.T) {
	b, e := WriteStepWorkbook(1, []StandardTableData{{TableID: "terminations", Rows: [][]string{{"0000001234567890123456", "02:00:00:00:00:01", "eth1", "000002", "R1/U1", "02:00:00:00:00:02", "eth2", "direct"}}}})
	if e != nil {
		t.Fatal(e)
	}
	q, e := ReadStepWorkbook(bytes.NewReader(b), 1)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(q[0].CSV, "0000001234567890123456") {
		t.Fatal("identity changed")
	}
	f, e := excelize.OpenReader(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = f.SetCellFormula("terminations", "A2", "1+1"); e != nil {
		t.Fatal(e)
	}
	body, e := f.WriteToBuffer()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadStepWorkbook(body, 1); e == nil || !strings.HasPrefix(e.Error(), "STEP_FORMULA_UNVERIFIED") {
		t.Fatal("unverified formula accepted", e)
	}
}
