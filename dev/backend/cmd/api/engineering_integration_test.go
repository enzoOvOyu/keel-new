package main

import (
	"context"
	"encoding/json"
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

func TestEngineeringPhysicalPublication(t *testing.T) {
	dsn := os.Getenv("KEEL_ENGINEERING_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "/keel_engineering_") {
		t.Skip("requires an explicitly isolated engineering test database")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	svc := &Server{db: db, project: "engineering-synthetic-" + uuid.NewString()}
	if e = (cmdb.Service{DB: db}).Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	migration, e := os.ReadFile("../../migrations/027_engineering_physical.sql")
	if e != nil {
		t.Fatal(e)
	}
	var migrated bool
	if e = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migration WHERE version=27)`).Scan(&migrated); e != nil {
		t.Fatal(e)
	}
	if !migrated {
		if _, e = db.Exec(ctx, string(migration)); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Engineering synthetic vertical slice',$1)`, svc.project); e != nil {
		t.Fatal(e)
	}
	original, source := uuid.NewString(), uuid.NewString()
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "arbitrary customer sheet", Cells: []bom.Cell{}}}}
	values := [][]string{{"Type", "Label", "Model", "Rack", "Start U", "Height U", "Room", "Site"}, {"server", "Synthetic-A", "Synthetic appliance", "Rack-X", "3", "2", "Room-X", "Site-X"}, {}, {}, {"Rack", "Capacity U", "Room", "Site"}, {"Rack-X", "32", "Room-X", "Site-X"}}
	for row, cells := range values {
		for column, value := range cells {
			ref := string(rune('A'+column)) + string(rune('1'+row))
			wb.Sheets[0].Cells = append(wb.Sheets[0].Cells, bom.Cell{Sheet: wb.Sheets[0].Name, Ref: ref, Row: row + 1, Column: column + 1, Value: value})
		}
	}
	if _, e = db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,workbook) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic-sha','MAPPING',$5)`, original, svc.project, uuid.NewString(), svc.project+"/synthetic.xlsx", jsonBytes(wb)); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, `INSERT INTO eng_source(id,project_id,import_id) VALUES($1,$2,$3)`, source, svc.project, original); e != nil {
		t.Fatal(e)
	}
	columns := []engineering.ColumnBinding{}
	for i, field := range []string{"equipment_type", "label", "model", "rack", "start_u", "height_u", "room", "site"} {
		columns = append(columns, engineering.ColumnBinding{Field: field, Column: i + 1})
	}
	p := engineering.EngineeringProposal{Summary: "Synthetic equipment and placement", Regions: []engineering.RegionMapping{{Sheet: wb.Sheets[0].Name, Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 8}, Kind: "equipment", HeaderRows: []int{1}, Columns: columns, Constants: []engineering.ConstantBinding{}}, {Sheet: wb.Sheets[0].Name, Bounds: engineering.Bounds{StartRow: 5, EndRow: 6, StartColumn: 1, EndColumn: 4}, Kind: "rack", HeaderRows: []int{5}, Columns: []engineering.ColumnBinding{{Field: "label", Column: 1}, {Field: "capacity_u", Column: 2}, {Field: "room", Column: 3}, {Field: "site", Column: 4}}, Constants: []engineering.ConstantBinding{}}}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	extracted, e := engineering.ExtractPhysical(wb, p, svc.project, source, uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	for _, i := range extracted.Issues {
		if i.Severity == "BLOCK" {
			t.Fatalf("unexpected source issue: %+v", i)
		}
	}
	if _, e = svc.saveEngineeringDraft(ctx, source, 1, "agent", "synthetic-agent", "synthetic mapping", extracted, p); e != nil {
		t.Fatal(e)
	}
	workspace, e := svc.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	if workspace.Revision != 2 || len(workspace.Targets) != 3 {
		t.Fatalf("bad workspace: %+v", workspace)
	}
	model, _, e := engineeringModel(ctx, db, *workspace.HeadRevisionID, svc.project)
	if e != nil {
		t.Fatal(e)
	}
	if len(model.Equipment) != 1 || len(model.Facilities) != 3 || len(model.Placements) != 1 {
		t.Fatalf("wrong typed model counts: %+v", model)
	}
	request := httptest.NewRequest("POST", "/", nil)
	request = request.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: identity.Subject{Username: "synthetic-human"}}))
	request.Header.Set("Idempotency-Key", "synthetic-publish")
	q := engineering.TargetAction{ExpectedRevision: 2, Targets: engineering.PhysicalTargets, Decision: "publish", Reason: "synthetic test"}
	if e = svc.engineeringAction(request, source, q, "publish"); e == nil {
		t.Fatal("published before human review")
	}
	q.Decision = "approve"
	if e = svc.engineeringAction(request, source, q, "review"); e != nil {
		t.Fatal(e)
	}
	q.Decision = "publish"
	if e = svc.engineeringAction(request, source, q, "publish"); e != nil {
		t.Fatal(e)
	}
	request.Header.Set("Idempotency-Key", "duplicate-in-flight")
	if e = svc.engineeringAction(request, source, q, "publish"); e == nil || e.Error() != "ENGINEERING_PUBLICATION_PENDING" {
		t.Fatalf("duplicate pending publication accepted: %v", e)
	}
	request.Header.Set("Idempotency-Key", "synthetic-publish")
	// Consume each exact queued event once in dependency order; no background poll,
	// real AI, production import or network device is invoked by this test.
	for _, target := range engineering.PhysicalTargets {
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
			_, err = tx.Exec(ctx, `UPDATE outbox_event SET published_at=now() WHERE id=$1`, event)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
		if err != nil {
			t.Fatalf("%s: %#v", target, err)
		}
	}
	workspace, e = svc.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range workspace.Targets {
		if v.State != engineering.Published {
			t.Fatalf("missing receipt: %+v", v)
		}
	}
	if e = svc.engineeringAction(request, source, q, "publish"); e != nil {
		t.Fatalf("idempotent replay: %v", e)
	}
	var assetCount int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1`, svc.project).Scan(&assetCount); e != nil {
		t.Fatal(e)
	}
	if assetCount != 5 {
		t.Fatalf("expected project + equipment + 3 facilities, got %d", assetCount)
	}
	visual, e := (cmdb.Service{DB: db}).Visualization(ctx, cmdb.Scope{Projects: []string{svc.project}}, "", "")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(visual)
	if !strings.Contains(string(b), "Synthetic-A") || !strings.Contains(string(b), "planned") {
		t.Fatal("existing rack visualization cannot consume planning model")
	}
	// A new placement must preserve an existing observed payload.
	device := model.Equipment[0].EntityID
	observed := []byte(`{"name":"Observed-name","hostname":"Observed-name","u_start":19,"u_height":2,"lifecycle_state":"discovered"}`)
	if _, e = db.Exec(ctx, `UPDATE cmdb_entity SET payload=$2,source_type='agent',source_ref='synthetic-observation',lifecycle_state='discovered' WHERE item_uid=$1`, device, observed); e != nil {
		t.Fatal(e)
	}
	four := 4
	model.Placements[0].StartU = &four
	_, ev, e := engineeringModel(ctx, db, *workspace.HeadRevisionID, svc.project)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.saveEngineeringDraft(ctx, source, 2, "human", "synthetic-human", "adjust planning U", engineering.Extraction{Model: model, Evidence: ev, TotalCells: extracted.TotalCells, MappedCells: extracted.MappedCells}, p); e != nil {
		t.Fatal(e)
	}
	workspace, e = svc.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	for _, target := range workspace.Targets {
		if target.Target != engineering.PlacementTarget && target.State != engineering.Published {
			t.Fatal("unrelated target invalidated", target)
		}
	}
	q = engineering.TargetAction{ExpectedRevision: 3, Targets: []string{engineering.PlacementTarget}, Decision: "approve", Reason: "review exact revised targets"}
	if e = svc.engineeringAction(request, source, q, "review"); e != nil {
		t.Fatal(e)
	}
	q.Decision = "publish"
	request.Header.Set("Idempotency-Key", "synthetic-publish-2")
	if e = svc.engineeringAction(request, source, q, "publish"); e != nil {
		t.Fatal(e)
	}
	for _, target := range []string{engineering.PlacementTarget} {
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event string
		var raw []byte
		err = tx.QueryRow(ctx, `SELECT o.id::text,o.payload FROM outbox_event o JOIN eng_publication p ON p.outbox_id=o.id WHERE p.source_id=$1 AND p.target=$2 AND p.idempotency_key='synthetic-publish-2'`, source, target).Scan(&event, &raw)
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
	var actual []byte
	if e = db.QueryRow(ctx, `SELECT payload FROM cmdb_entity WHERE item_uid=$1`, device).Scan(&actual); e != nil {
		t.Fatal(e)
	}
	var a, o any
	json.Unmarshal(actual, &a)
	json.Unmarshal(observed, &o)
	if engineering.Digest(a) != engineering.Digest(o) {
		t.Fatalf("observed payload overwritten: %s", actual)
	}
	visual, e = (cmdb.Service{DB: db}).Visualization(ctx, cmdb.Scope{Projects: []string{svc.project}}, "", "")
	if e != nil {
		t.Fatal(e)
	}
	b, _ = json.Marshal(visual)
	if !strings.Contains(string(b), `"u_start":4`) {
		t.Fatal("new planned U missing from existing visualization")
	}
	// A later AI suggestion cannot replace the human-confirmed placement.
	if _, e = svc.saveEngineeringDraft(ctx, source, 3, "agent", "synthetic-agent", "reanalysis", extracted, p); e != nil {
		t.Fatal(e)
	}
	current, e := svc.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	protected, _, e := engineeringModel(ctx, db, *current.HeadRevisionID, svc.project)
	if e != nil {
		t.Fatal(e)
	}
	if *protected.Placements[0].StartU != 4 {
		t.Fatal("agent replaced confirmed placement")
	}
	conflict := false
	for _, issue := range current.Issues {
		if issue.Code == "REANALYSIS_REVIEWED_CONFLICT" && issue.PublishTarget == engineering.PlacementTarget {
			conflict = true
		}
	}
	if !conflict {
		t.Fatal("reanalysis conflict disappeared")
	}
}
