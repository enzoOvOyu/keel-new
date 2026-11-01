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

func TestEngineeringEstimatePublicationAndWorkload(t *testing.T) {
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
	svc := &Server{db: db, project: "engineering-estimate-" + uuid.NewString()}
	if e = (cmdb.Service{DB: db}).Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	prepareEngineeringDB(t, db)
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic estimate',$1)`, svc.project); e != nil {
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

	device, material := uuid.NewString(), uuid.NewString()
	model := engineering.EmptyPhysical(svc.project)
	model.Equipment = []engineering.EquipmentPlan{{EntityVersion: engineering.EntityVersion{EntityID: device, Maturity: engineering.Estimated}, EquipmentType: "server", Label: "Synthetic group", SourceModelLabel: "Synthetic model", Quantity: 3, IdentityKind: "group"}}
	model.Materials = []engineering.MaterialEstimate{{EntityVersion: engineering.EntityVersion{EntityID: material, Maturity: engineering.Estimated}, SpecRef: uuid.NewString(), SourceModelLabel: "Synthetic procurement", Required: engineering.Quantity{Value: "10", Unit: "count"}, Purchased: &engineering.Quantity{Value: "7", Unit: "count"}}}
	p := engineering.EngineeringProposal{Summary: "Synthetic estimates", Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	if _, e = svc.saveEngineeringDraft(ctx, source, 1, "human", "synthetic-human", "Explicit planning quantities", engineering.Extraction{Model: model}, p); e != nil {
		t.Fatal(e)
	}
	runID, e := svc.queueEngineeringCalculation(ctx, source, engineering.CalculationRequest{ExpectedRevision: 2, Operation: "workload_summary", RequestedCells: []engineering.FormulaCell{}, Inputs: []engineering.NamedValue{}}, "synthetic-human")
	if e != nil {
		t.Fatal(e)
	}
	svc.runEngineeringCalculation(ctx)
	page, e := svc.engineeringCalculation(ctx, runID, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	if page.Run.Status != "succeeded" || len(page.Run.Outputs) != 1 {
		t.Fatalf("workload run %+v", page.Run)
	}
	output := page.Run.Outputs[0]
	if e = svc.applyEngineeringCalculation(ctx, source, engineering.CalculationApplication{ExpectedRevision: 2, RunID: runID, Reason: "Apply model quantities", Bindings: []engineering.ResultBinding{{EntityID: output.Name, OutputName: output.Name, Field: "workload_quantity"}}}, "synthetic-human"); e != nil {
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
	if len(m.Workloads) != 1 || m.Workloads[0].Quantity.Value != "3" || m.Workloads[0].LaborHours != nil || m.Materials[0].PurchaseGap != nil {
		t.Fatal("wrong quantity or invented gap/labor")
	}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: identity.Subject{Username: "synthetic-human"}}))
	r.Header.Set("Idempotency-Key", "synthetic-estimates")
	targets := []string{engineering.MaterialTarget, engineering.WorkloadTarget}
	q := engineering.TargetAction{ExpectedRevision: 3, Targets: targets, Decision: "approve", Reason: "Independent planning estimates"}
	if e = svc.engineeringAction(r, source, q, "review"); e != nil {
		t.Fatal(e)
	}
	q.Decision = "publish"
	if e = svc.engineeringAction(r, source, q, "publish"); e != nil {
		t.Fatal(e)
	}
	for _, target := range targets {
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event string
		var raw []byte
		err = tx.QueryRow(ctx, `SELECT o.id::text,o.payload FROM outbox_event o JOIN eng_publication p ON p.outbox_id=o.id WHERE p.source_id=$1 AND p.target=$2`, source, target).Scan(&event, &raw)
		if err == nil {
			err = svc.publishEngineering(ctx, tx, event, svc.project, raw)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	var assets int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1`, svc.project).Scan(&assets); e != nil {
		t.Fatal(e)
	}
	if assets != 0 {
		t.Fatal("estimate publication created real assets")
	}
	_, x, p, e := svc.engineeringEditContext(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	x.Model.Equipment[0].Quantity = 4
	if _, e = svc.saveEngineeringDraft(ctx, source, 3, "human", "synthetic-human", "Change planned quantity", x, p); e != nil {
		t.Fatal(e)
	}
	w, e = svc.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	blocked := false
	for _, i := range w.Issues {
		blocked = blocked || i.Code == "WORKLOAD_CALCULATION_STALE"
	}
	if !blocked {
		t.Fatal("old workload remained publishable after source count changed")
	}
	runID, e = svc.queueEngineeringCalculation(ctx, source, engineering.CalculationRequest{ExpectedRevision: 4, Operation: "workload_summary", RequestedCells: []engineering.FormulaCell{}, Inputs: []engineering.NamedValue{}}, "synthetic-human")
	if e != nil {
		t.Fatal(e)
	}
	svc.runEngineeringCalculation(ctx)
	page, e = svc.engineeringCalculation(ctx, runID, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	output = page.Run.Outputs[0]
	if e = svc.applyEngineeringCalculation(ctx, source, engineering.CalculationApplication{ExpectedRevision: 4, RunID: runID, Reason: "Apply updated quantities", Bindings: []engineering.ResultBinding{{EntityID: output.Name, OutputName: output.Name, Field: "workload_quantity"}}}, "synthetic-human"); e != nil {
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
	if len(m.Workloads) != 1 || m.Workloads[0].Quantity.Value != "4" {
		t.Fatal("recalculation duplicated generated workload")
	}
	for _, i := range w.Issues {
		if i.Code == "WORKLOAD_CALCULATION_STALE" {
			t.Fatal("fresh workload still stale")
		}
	}
}
