package cmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"keel.local/dev/internal/engineering"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func engineeringBatchTestSchema() Schema {
	return Schema{Version: "batch-test", Types: []AssetType{{Name: "device", Attributes: []Attribute{
		{Name: "name", Type: "Text"}, {Name: "serial", Type: "Text", Unique: true}, {Name: "model", Type: "Text"},
		{Name: "height_u", Type: "Number"}, {Name: "lifecycle_state", Type: "Text"}, {Name: "fact_layer", Type: "Text"},
	}}}}
}
func engineeringBatchTestInput() engineeringAssetWrite {
	v := "3.5"
	unit := "mm"
	return engineeringAssetWrite{ID: uuid.NewString(), Kind: "device", Label: "synthetic", Attrs: map[string]any{
		"name": "synthetic", "model": "new-model", "height_u": float64(2), "lifecycle_state": "planned", "fact_layer": "planned",
		"extra_geometry": float64(123), "engineering.attribute.depth@1": engineering.TypedValue{Kind: "decimal", Decimal: &v, Unit: &unit},
	}}
}
func TestEngineeringAssetBatchPreparation(t *testing.T) {
	schema := engineeringBatchTestSchema()
	in := engineeringBatchTestInput()
	row, fields, err := prepareEngineeringAsset(schema, "p", "pub", in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if row.Version != 1 || !row.Changed || len(fields) != len(in.Attrs) {
		t.Fatalf("new asset: %+v %v", row, fields)
	}
	if _, ok := row.Payload["extra_geometry"]; ok {
		t.Fatal("undeclared geometry entered payload")
	}
	if _, ok := row.Payload["engineering.attribute.depth@1"]; ok {
		t.Fatal("extension entered generic payload")
	}
	for _, field := range fields {
		if field.Field == "engineering.attribute.depth@1" {
			if _, ok := field.Value.(engineering.TypedValue); !ok {
				t.Fatal("typed planned value lost")
			}
		}
	}
	old := &engineeringAssetState{Kind: in.Kind, Ref: "engineering:old", Payload: Encode(row.Payload), Version: 1, InProject: true, Fields: map[string]bool{}}
	for _, field := range fields {
		old.Fields[field.Field] = true
	}
	replay, source, err := prepareEngineeringAsset(schema, "p", "pub", in, old)
	if err != nil || replay.Changed || replay.Version != 1 || len(source) != 0 {
		t.Fatalf("replay: %+v %v %v", replay, source, err)
	}
	in.Attrs["height_u"] = "not-a-number"
	if _, _, err = prepareEngineeringAsset(schema, "p", "pub", in, nil); err == nil || err.Error() != "ASSET_SCHEMA_VIOLATION" {
		t.Fatal("schema validation bypassed", err)
	}
}
func TestEngineeringAssetBatchProtectionAndIdentity(t *testing.T) {
	for _, ref := range []string{"engineering:old", "manual:confirmed"} {
		t.Run(ref, func(t *testing.T) {
			in := engineeringBatchTestInput()
			old := &engineeringAssetState{Kind: in.Kind, Ref: ref, Payload: json.RawMessage(`{"name":"confirmed","model":"observed-model","height_u":4,"lifecycle_state":"maintained"}`), Version: 7, InProject: true, Nonplanned: ref == "engineering:old", Fields: map[string]bool{}}
			row, fields, err := prepareEngineeringAsset(engineeringBatchTestSchema(), "p", "pub", in, old)
			if err != nil {
				t.Fatal(err)
			}
			var previous map[string]any
			if err = json.Unmarshal(old.Payload, &previous); err != nil {
				t.Fatal(err)
			}
			if !row.Protected || row.Changed || !row.Advance || row.Version != 8 || Hash(row.Payload) != Hash(previous) {
				t.Fatalf("observed state replaced: %+v", row)
			}
			for _, field := range fields {
				old.Fields[field.Field] = true
			}
			old.Version = 8
			row, fields, err = prepareEngineeringAsset(engineeringBatchTestSchema(), "p", "pub", in, old)
			if err != nil || row.Advance || row.Version != 8 || len(fields) != 0 {
				t.Fatal("protected replay creates revision", err)
			}
			old.InProject = false
			if _, _, err = prepareEngineeringAsset(engineeringBatchTestSchema(), "p", "pub", in, old); err == nil || err.Error() != "CROSS_PROJECT_ASSET_ASSOCIATION_DENIED" {
				t.Fatal(err)
			}
			old.InProject = true
			old.Kind = "rack"
			if _, _, err = prepareEngineeringAsset(engineeringBatchTestSchema(), "p", "pub", in, old); err == nil || err.Error() != "ASSET_TYPE_IDENTITY_CONFLICT" {
				t.Fatal(err)
			}
			old.Kind = in.Kind
			other := uuid.NewString()
			old.BOM = &other
			if _, _, err = prepareEngineeringAsset(engineeringBatchTestSchema(), "p", "pub", in, old); err == nil || err.Error() != "BOM_ASSET_IDENTITY_CONFLICT" {
				t.Fatal(err)
			}
			old.BOM = nil
			old.Payload = json.RawMessage(`{"serial":"immutable"}`)
			in.Attrs["serial"] = "replacement"
			if _, _, err = prepareEngineeringAsset(engineeringBatchTestSchema(), "p", "pub", in, old); err == nil || err.Error() != "ASSET_IDENTITY_IMMUTABLE" {
				t.Fatal(err)
			}
		})
	}
}
func TestEngineeringAssetBatchLimits(t *testing.T) {
	for _, tc := range []struct{ n, size, limit, want int }{{1001, 1, 500, 3}, {4001, 1, 2000, 3}, {3, engineeringBatchBytes / 2, 500, 3}} {
		rows := make([]string, tc.n)
		for i := range rows {
			rows[i] = strings.Repeat("x", tc.size)
		}
		count, total := 0, 0
		err := engineeringJSONBatches(rows, tc.limit, func(raw []byte) error {
			count++
			if len(raw) > engineeringBatchBytes {
				t.Fatal("byte bound exceeded")
			}
			var values []string
			if err := json.Unmarshal(raw, &values); err != nil {
				return err
			}
			if len(values) > tc.limit {
				t.Fatal("row bound exceeded")
			}
			total += len(values)
			return nil
		})
		if err != nil || count != tc.want || total != tc.n {
			t.Fatalf("batches=%d total=%d err=%v", count, total, err)
		}
	}
	sentinel := errors.New("write failed")
	if err := engineeringJSONBatches([]string{"a", "b"}, 1, func([]byte) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatal("write error discarded", err)
	}
	if err := engineeringJSONBatches([]string{strings.Repeat("x", engineeringBatchBytes)}, 500, func([]byte) error { t.Fatal("oversized SQL sent"); return nil }); err == nil {
		t.Fatal("oversized row accepted")
	}
}

// Opt-in, disposable PostgreSQL with no host ports/network. Never uses DATABASE_URL.
func TestEngineeringAssetBatchPostgres(t *testing.T) {
	if os.Getenv("KEEL_ENGINEERING_BATCH_DOCKER_TEST") != "1" {
		t.Skip("requires disposable Docker PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	socket, err := os.MkdirTemp("/tmp", "keel-asset-batch-test-")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(socket, 0777); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(socket) })
	name := "keel-asset-batch-test-" + uuid.NewString()
	out, err := exec.CommandContext(ctx, "docker", "run", "-d", "--rm", "--network", "none", "--name", name, "--tmpfs", "/var/lib/postgresql/data", "-v", socket+":/var/run/postgresql", "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "postgres:15").CombinedOutput()
	if err != nil {
		t.Fatalf("isolated PostgreSQL: %v %s", err, out)
	}
	t.Cleanup(func() { exec.Command("docker", "rm", "-f", name).Run() })
	var conn *pgx.Conn
	for i := 0; i < 100; i++ {
		logs, logErr := exec.CommandContext(ctx, "docker", "logs", name).CombinedOutput()
		if logErr != nil || !strings.Contains(string(logs), "init process complete; ready for start up") {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		conn, err = pgx.Connect(ctx, "host="+socket+" user=postgres dbname=postgres sslmode=disable")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || conn == nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	migrations, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range migrations {
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = conn.Exec(ctx, string(raw)); e != nil {
			t.Fatalf("migration %s: %v", filepath.Base(path), e)
		}
	}
	schema := engineeringBatchTestSchema()
	if _, err = conn.Exec(ctx, `INSERT INTO cmdb_schema_version(id,status,document,created_by,content_hash) VALUES($1,'active',$2,'test','batch-test')`, schema.Version, Encode(schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO cmdb_schema_asset_type(schema_version,name,definition) VALUES($1,'device',$2)`, schema.Version, Encode(schema.Types[0])); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO project(id,name,code) VALUES('batch-test','test','test'),('other-batch-test','other','other')`); err != nil {
		t.Fatal(err)
	}
	publication := uuid.NewString()
	importID := uuid.NewString()
	if _, err = conn.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status) VALUES($1,'batch-test',$2,'test','synthetic.xlsx','test','committed')`, importID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO bom_revision(id,import_id,project_id,digest) VALUES($1,$2,'batch-test','synthetic')`, publication, importID); err != nil {
		t.Fatal(err)
	}
	inputs := make([]engineeringAssetWrite, 501)
	for i := range inputs {
		inputs[i] = engineeringBatchTestInput()
		inputs[i].Attrs["serial"] = fmt.Sprintf("synthetic-%d", i)
	}
	ids := make([]string, len(inputs))
	for i, in := range inputs {
		ids[i] = in.ID
	}
	if _, err = conn.Exec(ctx, `INSERT INTO eng_entity(id,project_id,kind,planning_key) SELECT id::uuid,'batch-test','equipment',id FROM unnest($1::text[]) id`, ids); err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	counted := &engineeringCountingTx{Tx: tx}
	if err = (Service{}).writeEngineeringAssets(ctx, counted, schema, "batch-test", publication, "test", inputs); err != nil {
		t.Fatal(err)
	}
	if counted.reads != 4 || counted.writes > 35 {
		t.Fatalf("per-asset SQL returned: reads=%d writes=%d", counted.reads, counted.writes)
	}
	t.Logf("501 assets: %d reads, %d set writes", counted.reads, counted.writes)
	for _, table := range []string{"bom_item", "bom_item_revision", "project_asset", "cmdb_entity", "cmdb_revision", "cmdb_asset_scope", "cmdb_unique_attribute"} {
		var n int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil || n != 501 {
			t.Fatalf("%s count=%d err=%v", table, n, err)
		}
	}
	var n int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM eng_entity WHERE bom_item_uid=id`).Scan(&n); err != nil || n != 501 {
		t.Fatalf("backlinks=%d %v", n, err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM cmdb_field_source WHERE fact_layer='planned'`).Scan(&n); err != nil || n != 501*len(inputs[0].Attrs) {
		t.Fatalf("sources=%d %v", n, err)
	}
	if err = (Service{}).writeEngineeringAssets(ctx, tx, schema, "batch-test", publication, "test", inputs); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM cmdb_revision`).Scan(&n); err != nil || n != 501 {
		t.Fatalf("replay revisions=%d %v", n, err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM cmdb_field_source WHERE field_name='engineering.attribute.depth@1' AND value->>'decimal'='3.5' AND fact_layer='planned'`).Scan(&n); err != nil || n != 501 {
		t.Fatalf("typed planning sources=%d %v", n, err)
	}
	// A new publication updates writable planning payloads and keeps history.
	nextPublication, nextImport := uuid.NewString(), uuid.NewString()
	if _, err = tx.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status) VALUES($1,'batch-test',$2,'test-next','synthetic.xlsx','next','committed')`, nextImport, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO bom_revision(id,import_id,project_id,digest) VALUES($1,$2,'batch-test','next')`, nextPublication, nextImport); err != nil {
		t.Fatal(err)
	}
	inputs[1].Attrs["model"] = "updated-model"
	if err = (Service{}).writeEngineeringAssets(ctx, tx, schema, "batch-test", nextPublication, "test", inputs[1:2]); err != nil {
		t.Fatal(err)
	}
	var updated bool
	if err = tx.QueryRow(ctx, `SELECT version=2 AND payload->>'model'='updated-model' AND revision_id=$2::uuid FROM cmdb_entity WHERE item_uid=$1`, inputs[1].ID, nextPublication).Scan(&updated); err != nil || !updated {
		t.Fatalf("planning update failed: %v", err)
	}
	// Nonplanned field evidence also protects an engineering-origin asset.
	if _, err = tx.Exec(ctx, `INSERT INTO cmdb_field_source(id,asset_id,field_name,value,source_type,source_ref,fact_layer,revision,operator,approval_status,project_id) VALUES(gen_random_uuid(),$1,'model','"observed"','manual','confirmed:test','observed','2','test','confirmed','batch-test')`, inputs[1].ID); err != nil {
		t.Fatal(err)
	}
	inputs[1].Attrs["model"] = "must-not-overwrite"
	inputs[1].Attrs["new_geometry"] = float64(42)
	if err = (Service{}).writeEngineeringAssets(ctx, tx, schema, "batch-test", nextPublication, "test", inputs[1:2]); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, `SELECT version=3 AND payload->>'model'='updated-model' FROM cmdb_entity WHERE item_uid=$1`, inputs[1].ID).Scan(&updated); err != nil || !updated {
		t.Fatalf("nonplanned source protection failed: %v", err)
	}
	// SQL unique constraints reject conflicting assets; the caller can roll back
	// the entire write set, including BOM rows inserted before that constraint.
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conflict := engineeringBatchTestInput()
	conflict.Attrs["serial"] = inputs[2].Attrs["serial"]
	if err = (Service{}).writeEngineeringAssets(ctx, savepoint, schema, "batch-test", publication, "test", []engineeringAssetWrite{conflict}); err == nil || err.Error() != "UNIQUE_ATTRIBUTE_CONFLICT" {
		t.Fatal("unique attribute conflict bypassed", err)
	}
	if err = savepoint.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM bom_item WHERE item_uid=$1`, conflict.ID).Scan(&n); err != nil || n != 0 {
		t.Fatal("partial BOM write survived rollback", err)
	}
	id := inputs[0].ID
	if _, err = tx.Exec(ctx, `UPDATE cmdb_entity SET payload=payload||'{"model":"observed","lifecycle_state":"maintained"}'::jsonb,lifecycle_state='maintained',freshness='fresh',source_ref='manual:confirmed' WHERE item_uid=$1`, id); err != nil {
		t.Fatal(err)
	}
	var before []byte
	if err = tx.QueryRow(ctx, `SELECT to_jsonb(e)-'version'-'updated_at' FROM cmdb_entity e WHERE item_uid=$1`, id).Scan(&before); err != nil {
		t.Fatal(err)
	}
	inputs[0].Attrs["additional_geometry"] = float64(500)
	if err = (Service{}).writeEngineeringAssets(ctx, tx, schema, "batch-test", publication, "test", inputs[:1]); err != nil {
		t.Fatal(err)
	}
	var after []byte
	if err = tx.QueryRow(ctx, `SELECT to_jsonb(e)-'version'-'updated_at' FROM cmdb_entity e WHERE item_uid=$1`, id).Scan(&after); err != nil || string(before) != string(after) {
		t.Fatalf("confirmed state changed: %v", err)
	}
	if err = (Service{}).writeEngineeringAssets(ctx, tx, schema, "batch-test", publication, "test", inputs[:1]); err != nil {
		t.Fatal(err)
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM cmdb_entity WHERE item_uid=$1`, id).Scan(&version); err != nil || version != 2 {
		t.Fatalf("protected replay version=%d %v", version, err)
	}
	if err = (Service{}).writeEngineeringAssets(ctx, tx, schema, "other-batch-test", publication, "test", inputs[:1]); err == nil || err.Error() != "CROSS_PROJECT_ASSET_ASSOCIATION_DENIED" {
		t.Fatal("scope bypass", err)
	}
}

type engineeringCountingTx struct {
	pgx.Tx
	reads, writes int
}

func (tx *engineeringCountingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.reads++
	return tx.Tx.Query(ctx, sql, args...)
}
func (tx *engineeringCountingTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.writes++
	return tx.Tx.Exec(ctx, sql, args...)
}
