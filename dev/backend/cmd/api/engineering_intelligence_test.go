package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"os"
	"strings"
	"testing"
)

func TestEngineeringIntelligencePersistence(t *testing.T) {
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
	prepareEngineeringDB(t, db)
	s := &Server{db: db, project: "engineering-plan-" + uuid.NewString()}
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic intelligence',$1)`, s.project); e != nil {
		t.Fatal(e)
	}
	book := bom.Workbook{Sheets: []bom.Sheet{{Name: "synthetic parameters", Cells: []bom.Cell{{Sheet: "synthetic parameters", Ref: "C4", Row: 4, Column: 3, Value: "Synthetic device"}, {Sheet: "synthetic parameters", Ref: "C5", Row: 5, Column: 3, Value: "switch"}, {Sheet: "synthetic parameters", Ref: "D4", Row: 4, Column: 4, Value: "1"}}}}}
	original := uuid.NewString()
	if _, e = db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,workbook) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic-sha','MAPPING',$5)`, original, s.project, uuid.NewString(), s.project+"/synthetic.xlsx", jsonBytes(book)); e != nil {
		t.Fatal(e)
	}
	source, e := s.attachEngineering(ctx, original, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	p := engineering.EngineeringProposal{Summary: "Independent modules", Regions: []engineering.RegionMapping{{Sheet: "synthetic parameters", Kind: "equipment", Bounds: engineering.Bounds{StartRow: 4, EndRow: 5, StartColumn: 3, EndColumn: 4}, Records: []engineering.RecordBinding{{Cells: []engineering.CellBinding{{Field: "label", Sheet: "synthetic parameters", Cell: "C4"}, {Field: "equipment_type", Sheet: "synthetic parameters", Cell: "C5"}, {Field: "quantity", Sheet: "synthetic parameters", Cell: "D4", Unit: "count"}}}}}}, Modules: []engineering.ModuleDetection{{ModuleID: "power_cooling", Applicable: true, Reason: "等待电力资料"}}}
	x, e := engineering.ExtractPhysical(book, p, s.project, source, uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.saveEngineeringDraft(ctx, source, 1, "agent", "synthetic-agent", "Proposed source facts", x, p); e != nil {
		t.Fatal(e)
	}
	w, e := s.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	plan, e := s.engineeringPlan(ctx, w)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan.Plan.Modules) != 8 || plan.Plan.PreviousPlanID == nil {
		t.Fatal("plan/version lineage not stored", plan)
	}
	for _, m := range plan.Plan.Modules {
		if m.DefinitionID == "equipment" && m.State != engineering.InReview {
			t.Fatal("equipment unexpectedly blocked", m)
		}
		if m.DefinitionID == "power_cooling" && m.State != engineering.NeedsInput {
			t.Fatal("missing input not represented", m)
		}
	}
	facts, e := s.engineeringIntelligence(ctx, w, 0)
	if e != nil {
		t.Fatal(e)
	}
	if facts.TotalFacts != 3 || len(facts.Regions) != 1 {
		t.Fatal("facts or regions lost", facts)
	}
	model, evidence, e := engineeringModel(ctx, db, *w.HeadRevisionID, s.project)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range evidence {
		if v.SourceID != source {
			t.Fatal("evidence source lost")
		}
	}
	model.Equipment[0].SourceModelLabel = "Human confirmed model"
	if _, e = s.saveEngineeringDraft(ctx, source, 2, "human", "synthetic-human", "Confirm model", engineering.Extraction{Model: model, Evidence: evidence}, p); e != nil {
		t.Fatal(e)
	}
	w, e = s.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	facts, e = s.engineeringIntelligence(ctx, w, 0)
	if e != nil || facts.TotalFacts != 3 {
		t.Fatal("human revision lost original proposed facts", e, facts)
	}
	var count int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM eng_provenance WHERE id=$1`, w.HeadRevisionID).Scan(&count); e != nil || count != 1 {
		t.Fatal("missing producer provenance", e)
	}
	model.Materials = []engineering.MaterialEstimate{{EntityVersion: engineering.EntityVersion{EntityID: uuid.NewString(), Maturity: engineering.Estimated}, SpecRef: uuid.NewString(), SourceModelLabel: "Synthetic part", Required: engineering.Quantity{Value: "1", Unit: "count"}}}
	if _, e = s.saveEngineeringDraft(ctx, source, 3, "human", "synthetic-human", "Add explicit requirement", engineering.Extraction{Model: model, Evidence: evidence}, p); e != nil {
		t.Fatal(e)
	}
	if e = s.queuePlanCalculations(ctx, source, 4); e != nil {
		t.Fatal(e)
	}
	var provider, module string
	if e = db.QueryRow(ctx, `SELECT provider_id,module_instance_id FROM eng_calculation_run WHERE source_id=$1`, source).Scan(&provider, &module); e != nil {
		t.Fatal(e)
	}
	if provider != "engineering-workload-summary" || module != engineering.StableID(source, "module", "material_workload") {
		t.Fatal("wrong calculation module selected", provider, module)
	}
	w, e = s.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	plan, e = s.engineeringPlan(ctx, w)
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range plan.Plan.Modules {
		if m.DefinitionID == "material_workload" && m.State != engineering.Processing {
			t.Fatal("queued module not processing")
		}
	}
	// Complete the synthetic queued run so it cannot affect another isolated check.
	s.runEngineeringCalculation(ctx)

}
