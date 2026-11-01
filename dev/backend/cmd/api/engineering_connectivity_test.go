package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/cmdb"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/identity"
)

func TestEngineeringConnectivityPublication(t *testing.T) {
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
	svc := &Server{db: db, project: "engineering-connectivity-" + uuid.NewString()}
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
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic connectivity',$1)`, svc.project); e != nil {
		t.Fatal(e)
	}
	sheet := bom.Sheet{Name: "Arbitrary structured region", Cells: []bom.Cell{}}
	fields := []string{"a_device", "a_port", "z_device", "z_port", "patch_device", "patch_port", "patch_exit_port", "medium", "length", "a_optic", "z_optic"}
	values := []string{"Synthetic-A", "port-1", "Synthetic-Z", "port-2", "Synthetic-Patch", "front-1", "rear-1", "fiber", "3", "Synthetic optic", "Synthetic optic"}
	columns := []engineering.ColumnBinding{}
	for i, field := range fields {
		u := ""
		if field == "length" {
			u = "m"
		}
		columns = append(columns, engineering.ColumnBinding{Field: field, Column: i + 1, Unit: u})
		sheet.Cells = append(sheet.Cells, bom.Cell{Sheet: sheet.Name, Ref: fmt.Sprintf("%c1", 'A'+i), Row: 1, Column: i + 1, Value: values[i]})
	}
	wb := bom.Workbook{Sheets: []bom.Sheet{sheet}}
	original := uuid.NewString()
	if _, e = db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,workbook) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic-sha','MAPPING',$5)`, original, svc.project, uuid.NewString(), svc.project+"/synthetic.xlsx", jsonBytes(wb)); e != nil {
		t.Fatal(e)
	}
	source, e := svc.attachEngineering(ctx, original, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	p := engineering.EngineeringProposal{Summary: "Synthetic planned connection with patch", Regions: []engineering.RegionMapping{{Sheet: sheet.Name, Bounds: engineering.Bounds{StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: len(fields)}, Kind: "cable", Columns: columns}}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	x, e := engineering.ExtractPhysical(wb, p, svc.project, source, uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	for _, issue := range x.Issues {
		if issue.Severity == "BLOCK" {
			t.Fatal(issue)
		}
	}
	if _, e = svc.saveEngineeringDraft(ctx, source, 1, "agent", "synthetic", "synthetic mapping", x, p); e != nil {
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
	if len(m.Cables) != 1 || m.Cables[0].Estimated == nil || m.Cables[0].Estimated.Length.Value != "3" || m.Cables[0].AsBuilt != nil {
		t.Fatalf("lost/separated cable state: %+v", m.Cables)
	}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: identity.Subject{Username: "synthetic-human"}}))
	r.Header.Set("Idempotency-Key", "synthetic-connectivity")
	targets := []string{engineering.EquipmentTarget, engineering.ConnectivityTarget, engineering.CablingTarget}
	q := engineering.TargetAction{ExpectedRevision: w.Revision, Targets: targets, Decision: "approve", Reason: "Synthetic review"}
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
			_, err = tx.Exec(ctx, `UPDATE outbox_event SET published_at=now() WHERE id=$1`, event)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
	}
	visual, e := (cmdb.Service{DB: db}).VisualConnections(ctx, cmdb.Scope{Projects: []string{svc.project}}, url.Values{"limit": {"200"}})
	if e != nil {
		t.Fatal(e)
	}
	if len(visual.Cables) != 2 {
		t.Fatalf("existing topology expected two patch segments, got %d", len(visual.Cables))
	}
	for _, c := range visual.Cables {
		if c.A == "" || c.B == "" || c.DeviceA == "" || c.DeviceB == "" {
			t.Fatalf("incomplete topology segment: %+v", c)
		}
	}
	var bad int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_entity e JOIN cmdb_asset_scope s ON s.asset_id=e.item_uid WHERE s.project_id=$1 AND ((e.asset_type='cable' AND e.payload ? 'length_m') OR e.asset_type='optic')`, svc.project).Scan(&bad); e != nil {
		t.Fatal(e)
	}
	if bad != 0 {
		t.Fatal("estimate/procurement leaked into CMDB actual facts")
	}
	before := targetDigest(m, engineering.ConnectivityTarget)
	m.Cables[0].Estimated.Length.Value = "4"
	if before != targetDigest(m, engineering.ConnectivityTarget) {
		t.Fatal("cable estimate invalidated independent connectivity approval")
	}
}
