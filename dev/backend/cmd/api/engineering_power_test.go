package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/cmdb"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/identity"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEngineeringPowerCalculationPublication(t *testing.T) {
	dsn := os.Getenv("KEEL_ENGINEERING_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "/keel_engineering_") {
		t.Skip("requires isolated engineering database")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	svc := &Server{db: db, project: "engineering-power-" + uuid.NewString()}
	if e = (cmdb.Service{DB: db}).Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	for _, step := range []struct {
		version int
		file    string
	}{{27, "027_engineering_physical.sql"}, {28, "028_engineering_calculation.sql"}, {29, "029_engineering_connectivity.sql"}, {30, "030_engineering_patch_through.sql"}, {31, "031_engineering_power.sql"}} {
		var exists bool
		if e = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migration WHERE version=$1)`, step.version).Scan(&exists); e != nil {
			t.Fatal(e)
		}
		if !exists {
			raw, err := os.ReadFile("../../migrations/" + step.file)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(ctx, string(raw)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic power',$1)`, svc.project); e != nil {
		t.Fatal(e)
	}

	original := uuid.NewString()
	if _, e = db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,workbook) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic-sha','MAPPING',$5)`, original, svc.project, uuid.NewString(), svc.project+"/synthetic.xlsx", jsonBytes(bom.Workbook{Sheets: []bom.Sheet{}})); e != nil {
		t.Fatal(e)
	}
	source, e := svc.attachEngineering(ctx, original, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	version := func() engineering.EntityVersion {
		return engineering.EntityVersion{EntityID: uuid.NewString(), Maturity: engineering.Estimated, EvidenceIDs: []string{}, Attributes: []engineering.AttributeValue{}}
	}
	device, rack, plan, placement := version(), version(), version(), version()
	capacityU, start, height := 32, 1, 1
	model := engineering.EmptyPhysical(svc.project)
	model.Equipment = []engineering.EquipmentPlan{{EntityVersion: device, EquipmentType: "server", Label: "Synthetic power device", Quantity: 1, IdentityKind: "planned_instance"}}
	model.Facilities = []engineering.FacilityModel{{EntityVersion: rack, Kind: "rack", Label: "Synthetic power rack", CapacityU: &capacityU}}
	model.Placements = []engineering.RackPlacement{{EntityVersion: placement, EquipmentID: device.EntityID, RackID: rack.EntityID, StartU: &start, HeightU: &height, MountFace: "front"}}
	model.PowerPlans = []engineering.PowerPlan{{EntityVersion: plan, EquipmentProfiles: []engineering.EquipmentPowerProfile{{EquipmentID: device.EntityID, Basis: "planned", Power: engineering.Quantity{Value: "2", Unit: "kW"}, CoolingType: "liquid"}, {EquipmentID: device.EntityID, Basis: "measured", Power: engineering.Quantity{Value: "9999", Unit: "W"}, CoolingType: "liquid"}}, Racks: []engineering.RackPowerPlan{{RackID: rack.EntityID, Capacity: &engineering.Quantity{Value: "3", Unit: "kW"}, CoolingCapacity: &engineering.Quantity{Value: "3", Unit: "kW"}, CoolingType: "liquid"}}}}
	p := engineering.EngineeringProposal{Summary: "Synthetic input", Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	if _, e = svc.saveEngineeringDraft(ctx, source, 1, "human", "synthetic-human", "Explicit synthetic power parameters", engineering.Extraction{Model: model}, p); e != nil {
		t.Fatal(e)
	}
	runID, e := svc.queueEngineeringCalculation(ctx, source, engineering.CalculationRequest{ExpectedRevision: 2, Operation: "power_balance", RequestedCells: []engineering.FormulaCell{}, Inputs: []engineering.NamedValue{}}, "synthetic-human")
	if e != nil {
		t.Fatal(e)
	}
	svc.runEngineeringCalculation(ctx)
	page, e := svc.engineeringCalculation(ctx, runID, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	if page.Run.Status != "succeeded" || len(page.Run.Inputs) != 2 || len(page.Run.Outputs) != 2 {
		t.Fatalf("bad persistent calculation %+v", page.Run)
	}
	application := engineering.CalculationApplication{ExpectedRevision: 2, RunID: runID, Reason: "Apply explicit result", Bindings: []engineering.ResultBinding{{EntityID: rack.EntityID, Field: "power_load", OutputName: rack.EntityID + "/load"}, {EntityID: rack.EntityID, Field: "power_remaining", OutputName: rack.EntityID + "/remaining"}}}
	if e = svc.applyEngineeringCalculation(ctx, source, application, "synthetic-human"); e != nil {
		t.Fatal(e)
	}
	w, e := svc.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	m, _, e := engineeringModel(ctx, db, *w.HeadRevisionID, svc.project)
	if e != nil {
		t.Fatal(e)
	}
	power := m.PowerPlans[0].Racks[0]
	if power.PlannedLoad == nil || power.PlannedLoad.Value != "2000" || power.Remaining == nil || power.Remaining.Value != "1000" || power.CalculationRunID == nil {
		t.Fatalf("wrong typed result %+v", power)
	}
	if e = svc.applyEngineeringCalculation(ctx, source, application, "synthetic-human"); e == nil {
		t.Fatal("stale calculation reapplied")
	}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: identity.Subject{Username: "synthetic-human"}}))
	r.Header.Set("Idempotency-Key", "synthetic-power")
	q := engineering.TargetAction{ExpectedRevision: 3, Targets: []string{engineering.PowerTarget}, Decision: "approve", Reason: "Review power domain"}
	if e = svc.engineeringAction(r, source, q, "review"); e != nil {
		t.Fatal(e)
	}
	q.Decision = "publish"
	if e = svc.engineeringAction(r, source, q, "publish"); e != nil {
		t.Fatal(e)
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var event string
	var raw []byte
	e = tx.QueryRow(ctx, `SELECT o.id::text,o.payload FROM outbox_event o JOIN eng_publication p ON p.outbox_id=o.id WHERE p.source_id=$1 AND p.target=$2`, source, engineering.PowerTarget).Scan(&event, &raw)
	if e == nil {
		e = svc.publishEngineering(ctx, tx, event, svc.project, raw)
	}
	if e == nil {
		e = tx.Commit(ctx)
	} else {
		tx.Rollback(ctx)
	}
	if e != nil {
		t.Fatal(e)
	}
	var assets int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1`, svc.project).Scan(&assets); e != nil {
		t.Fatal(e)
	}
	if assets != 0 {
		t.Fatal("power publication created CMDB assets")
	}
	// A caller cannot tamper with the result while retaining a legitimate calculation ID.
	tampered := m.PowerPlans[0]
	tampered.Racks = append([]engineering.RackPowerPlan{}, tampered.Racks...)
	tampered.Racks[0].PlannedLoad = &engineering.Quantity{Value: "1", Unit: "W"}
	if e = svc.editEngineeringPower(ctx, source, engineering.PowerEdit{ExpectedRevision: 3, Plan: tampered, Reason: "invalid result"}, "synthetic-human"); e == nil || e.Error() != "CALC_VALUE_DOES_NOT_MATCH_RESULT" {
		t.Fatalf("tampered result accepted %v", e)
	}
	changed := m.PowerPlans[0]
	changed.EquipmentProfiles[0].Power = engineering.Quantity{Value: "2.5", Unit: "kW"}
	if e = svc.editEngineeringPower(ctx, source, engineering.PowerEdit{ExpectedRevision: 3, Plan: changed, Reason: "Change planning input"}, "synthetic-human"); e != nil {
		t.Fatal(e)
	}
	w, e = svc.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	m, _, e = engineeringModel(ctx, db, *w.HeadRevisionID, svc.project)
	if e != nil {
		t.Fatal(e)
	}
	if m.PowerPlans[0].Racks[0].PlannedLoad != nil || m.PowerPlans[0].Racks[0].Remaining != nil {
		t.Fatal("old balance retained after input change")
	}
}
