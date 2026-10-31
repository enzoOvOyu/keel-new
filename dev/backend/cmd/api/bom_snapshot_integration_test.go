package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/bom"
	"os"
	"testing"
)

func TestRecognitionSnapshotAtomicity(t *testing.T) {
	if os.Getenv("KEEL_BOM_INTEGRATION") != "1" {
		t.Skip("requires explicit PostgreSQL integration environment")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// A connection-local shadow table exercises transactions without touching
	// application batches, projects or source files.
	_, err = db.Exec(ctx, `CREATE TEMP TABLE bom_import_batch(id text PRIMARY KEY,workbench jsonb,draft jsonb,revision int,reviewed_revision int,updated_at timestamptz); INSERT INTO bom_import_batch VALUES('synthetic','{}','[]',1,1,now())`)
	if err != nil {
		t.Fatal(err)
	}
	s := Server{db: db, project: "synthetic-project"}
	b := bom.Workbench{State: "pending_review", Records: []bom.ImportRecord{{ID: "synthetic-device", Item: bom.Item{UID: "synthetic-device", Category: "switch", Model: "synthetic-model"}}}}
	if !s.saveRecognitionSnapshot(ctx, "synthetic", 1, &b, true) {
		t.Fatal("final save failed")
	}
	var rev int
	var wb, draft string
	if err = db.QueryRow(ctx, "SELECT revision,workbench::text,draft::text FROM bom_import_batch WHERE id='synthetic'").Scan(&rev, &wb, &draft); err != nil {
		t.Fatal(err)
	}
	if rev != 2 || draft == "[]" {
		t.Fatal("snapshot and draft not committed")
	}
	if s.saveRecognitionSnapshot(ctx, "synthetic", 1, &b, true) {
		t.Fatal("stale revision accepted")
	}
	_, err = db.Exec(ctx, `CREATE FUNCTION pg_temp.reject_changed_draft() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.draft IS DISTINCT FROM OLD.draft THEN RAISE EXCEPTION 'synthetic draft rejection'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_draft BEFORE UPDATE ON bom_import_batch FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_changed_draft()`)
	if err != nil {
		t.Fatal(err)
	}
	b.State = "synthetic-failed-save"
	b.Records[0].Item.Model = "synthetic-changed-model"
	if s.saveRecognitionSnapshot(ctx, "synthetic", 2, &b, true) {
		t.Fatal("rejected second write succeeded")
	}
	var afterWB, afterDraft string
	if err = db.QueryRow(ctx, "SELECT revision,workbench::text,draft::text FROM bom_import_batch WHERE id='synthetic'").Scan(&rev, &afterWB, &afterDraft); err != nil {
		t.Fatal(err)
	}
	if rev != 2 || afterWB != wb || afterDraft != draft {
		t.Fatal("partial final snapshot committed")
	}
}
