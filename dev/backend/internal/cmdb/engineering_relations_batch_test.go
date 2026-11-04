package cmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func batchRelationSchema(t *testing.T) Schema {
	t.Helper()
	var s Schema
	if err := json.Unmarshal(builtinSchema, &s); err != nil {
		t.Fatal(err)
	}
	return s
}
func batchRelation(def, from, to string) RelationInput {
	return RelationInput{Definition: def, SourceID: from, TargetID: to, Project: "test-project", Source: Source{Type: "bom", Ref: "engineering:test-publication", Layer: "planned", Operator: "synthetic-test", Approval: "approved", Evidence: []string{"synthetic-evidence"}}}
}
func TestEngineeringRelationBatchValidation(t *testing.T) {
	schema := batchRelationSchema(t)
	endpoints := map[string]engineeringRelationEndpoint{"d1": {"device", true}, "d2": {"device", true}, "p1": {"port", true}, "p2": {"port", true}, "c1": {"cable", true}, "c2": {"cable", true}, "r1": {"rack", true}, "r2": {"rack", true}, "test-project": {"project", true}, "outside": {"port", false}}
	old := func(def, from, to, project, layer, ref string) engineeringRelationCurrent {
		return engineeringRelationCurrent{id: uuid.NewString(), definition: def, from: from, to: to, project: project, layer: layer, ref: ref, version: 2}
	}
	cases := []struct {
		name          string
		inputs        []RelationInput
		existing      []engineeringRelationCurrent
		code          string
		inserts, ends int
	}{
		{name: "batch deduplication", inputs: []RelationInput{batchRelation("contains", "d1", "p1"), batchRelation("contains", "d1", "p1")}, inserts: 1},
		{name: "batch cardinality", inputs: []RelationInput{batchRelation("contains", "d1", "p1"), batchRelation("contains", "d2", "p1")}, code: "RELATIONSHIP_CARDINALITY"},
		{name: "cross cable ends", inputs: []RelationInput{batchRelation("cable_endpoint_a", "c1", "p1"), batchRelation("cable_endpoint_b", "c2", "p1")}, code: "PORT_ALREADY_CABLED"},
		{name: "same cable same port", inputs: []RelationInput{batchRelation("cable_endpoint_a", "c1", "p1"), batchRelation("cable_endpoint_b", "c1", "p1")}, code: "PORT_ALREADY_CABLED"},
		{name: "other project cable", inputs: []RelationInput{batchRelation("cable_endpoint_b", "c2", "p1")}, existing: []engineeringRelationCurrent{old("cable_endpoint_a", "c1", "p1", "other", "planned", "engineering:old")}, code: "PORT_ALREADY_CABLED"},
		{name: "other project ownership", inputs: []RelationInput{batchRelation("contains", "d2", "p1")}, existing: []engineeringRelationCurrent{old("contains", "d1", "p1", "other", "planned", "engineering:old")}, code: "RELATIONSHIP_CARDINALITY"},
		{name: "manual planning protected", inputs: []RelationInput{batchRelation("located_in", "d1", "r2")}, existing: []engineeringRelationCurrent{old("located_in", "d1", "r1", "test-project", "planned", "manual:confirmed")}, code: "RELATIONSHIP_CARDINALITY"},
		{name: "replace planning location", inputs: []RelationInput{batchRelation("located_in", "d1", "r2")}, existing: []engineeringRelationCurrent{old("located_in", "d1", "r1", "test-project", "planned", "engineering:old")}, inserts: 1, ends: 1},
		{name: "replace planning owner", inputs: []RelationInput{batchRelation("contains", "d2", "p1")}, existing: []engineeringRelationCurrent{old("contains", "d1", "p1", "test-project", "planned", "engineering:old")}, inserts: 1, ends: 1},
		{name: "observed retained separately", inputs: []RelationInput{batchRelation("contains", "d2", "p1")}, existing: []engineeringRelationCurrent{old("contains", "d1", "p1", "test-project", "observed", "engineering:old")}, inserts: 1},
		{name: "replay no writes", inputs: []RelationInput{batchRelation("contains", "d1", "p1")}, existing: []engineeringRelationCurrent{old("contains", "d1", "p1", "test-project", "planned", "engineering:old")}},
		{name: "missing endpoint", inputs: []RelationInput{batchRelation("contains", "d1", "missing")}, code: "RELATIONSHIP_ENDPOINT_SCOPE"},
		{name: "unscoped endpoint", inputs: []RelationInput{batchRelation("contains", "d1", "outside")}, code: "RELATIONSHIP_ENDPOINT_SCOPE"},
		{name: "wrong type", inputs: []RelationInput{batchRelation("cable_endpoint_a", "d1", "p1")}, code: "RELATIONSHIP_ENDPOINT_TYPE"},
		{name: "self cycle", inputs: []RelationInput{batchRelation("contains", "d1", "d1")}, code: "RELATIONSHIP_SELF_CYCLE"},
		{name: "unknown definition", inputs: []RelationInput{batchRelation("unknown", "d1", "p1")}, code: "UNKNOWN_RELATIONSHIP_TYPE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ins, ends, err := planEngineeringRelations(schema, "test-project", c.inputs, endpoints, c.existing)
			if c.code != "" {
				if err == nil || err.Error() != c.code {
					t.Fatalf("want %s, got %v", c.code, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(ins) != c.inserts || len(ends) != c.ends {
				t.Fatalf("inserts=%d ends=%d", len(ins), len(ends))
			}
		})
	}
	for _, field := range []string{"revision", "source", "project"} {
		t.Run(field, func(t *testing.T) {
			in := batchRelation("contains", "d1", "p1")
			want := ""
			switch field {
			case "revision":
				v := int64(1)
				in.ExpectedVersion = &v
				want = "RELATIONSHIP_REVISION_CONFLICT"
			case "source":
				in.Source.Type = "untrusted"
				want = "RELATIONSHIP_SOURCE_DENIED"
			case "project":
				in.Project = "other"
				want = "RELATIONSHIP_ENDPOINT_SCOPE"
			}
			_, _, err := planEngineeringRelations(schema, "test-project", []RelationInput{in}, endpoints, []engineeringRelationCurrent{old("contains", "d1", "p1", "test-project", "planned", "engineering:old")})
			if err == nil || err.Error() != want {
				t.Fatalf("want %s got %v", want, err)
			}
		})
	}
	t.Run("recorded association", func(t *testing.T) {
		in := batchRelation("project_association", "d1", "test-project")
		in.Source.Type = "derived"
		in.Source.Layer = "recorded"
		rows, _, err := planEngineeringRelations(schema, "test-project", []RelationInput{in}, endpoints, nil)
		if err != nil || len(rows) != 1 || rows[0].Layer != "recorded" || rows[0].Payload.Source.Operator != "synthetic-test" {
			t.Fatalf("%+v %v", rows, err)
		}
	})
}

// Explicit synthetic URL only: never use DATABASE_URL or customer rows.
// Temporary tables are private to this test connection and rolled back.
func batchRelationTestTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	url := os.Getenv("KEEL_CMDB_RELATION_BATCH_TEST_URL")
	if url == "" {
		t.Skip("isolated synthetic PostgreSQL URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback(context.Background()) })
	_, err = tx.Exec(ctx, `CREATE TEMP TABLE cmdb_entity(item_uid text PRIMARY KEY,asset_type text NOT NULL);
 CREATE TEMP TABLE cmdb_asset_scope(asset_id text REFERENCES cmdb_entity,project_id text,PRIMARY KEY(asset_id,project_id));
 CREATE TEMP TABLE cmdb_relationship(id uuid PRIMARY KEY,from_uid text REFERENCES cmdb_entity,to_uid text REFERENCES cmdb_entity,kind text,definition_id text,project_id text,fact_layer text,source_type text,source_ref text,direction text,cardinality text,definition_version text,version bigint DEFAULT 1,valid_to timestamptz,relationship_status text DEFAULT 'active');
 CREATE UNIQUE INDEX ON cmdb_relationship(from_uid,to_uid,kind,fact_layer) WHERE valid_to IS NULL;
 CREATE INDEX ON cmdb_relationship(from_uid); CREATE INDEX ON cmdb_relationship(to_uid);
 CREATE TEMP TABLE cmdb_relationship_revision(id uuid PRIMARY KEY,relationship_id uuid NOT NULL,version bigint NOT NULL,payload jsonb NOT NULL,source_ref text NOT NULL,created_at timestamptz DEFAULT now(),UNIQUE(relationship_id,version));`)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, tx
}
func batchRelationAssets(t *testing.T, ctx context.Context, tx pgx.Tx, assets map[string]string) {
	t.Helper()
	rows := []map[string]string{}
	for id, kind := range assets {
		rows = append(rows, map[string]string{"id": id, "kind": kind})
	}
	_, err := tx.Exec(ctx, `WITH inserted AS (INSERT INTO cmdb_entity SELECT id,kind FROM jsonb_to_recordset($1::jsonb) x(id text,kind text) RETURNING item_uid) INSERT INTO cmdb_asset_scope SELECT item_uid,'test-project' FROM inserted`, Encode(rows))
	if err != nil {
		t.Fatal(err)
	}
}

type relationCountingTx struct {
	pgx.Tx
	calls int
}

func (tx *relationCountingTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.calls++
	return tx.Tx.Exec(ctx, sql, args...)
}
func (tx *relationCountingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.calls++
	return tx.Tx.Query(ctx, sql, args...)
}
func (tx *relationCountingTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.calls++
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func TestEngineeringRelationBatchSQL(t *testing.T) {
	schema := batchRelationSchema(t)
	svc := Service{}
	t.Run("14308 contains constant queries replay provenance", func(t *testing.T) {
		ctx, tx := batchRelationTestTx(t)
		assets := map[string]string{"d1": "device", "test-project": "project"}
		inputs := []RelationInput{}
		for i := 0; i < 14308; i++ {
			id := fmt.Sprintf("port-%05d", i)
			assets[id] = "port"
			inputs = append(inputs, batchRelation("contains", "d1", id))
		}
		in := batchRelation("project_association", "d1", "test-project")
		in.Source.Type = "derived"
		in.Source.Layer = "recorded"
		inputs = append(inputs, in)
		batchRelationAssets(t, ctx, tx, assets)
		counted := &relationCountingTx{Tx: tx}
		started := time.Now()
		if err := svc.writeEngineeringRelations(ctx, counted, schema, "test-project", inputs); err != nil {
			t.Fatal(err)
		}
		if counted.calls != 4 {
			t.Fatalf("SQL calls %d", counted.calls)
		}
		t.Logf("synthetic 14308 contains + association: %d calls, %s", counted.calls, time.Since(started))
		var n int
		var operator string
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM cmdb_relationship_revision`).Scan(&n); err != nil || n != 14309 {
			t.Fatalf("revisions=%d err=%v", n, err)
		}
		if err := tx.QueryRow(ctx, `SELECT payload->'source'->>'operator' FROM cmdb_relationship_revision LIMIT 1`).Scan(&operator); err != nil || operator != "synthetic-test" {
			t.Fatalf("provenance %s %v", operator, err)
		}
		counted.calls = 0
		if err := svc.writeEngineeringRelations(ctx, counted, schema, "test-project", inputs); err != nil {
			t.Fatal(err)
		}
		if counted.calls != 3 {
			t.Fatalf("replay SQL calls %d", counted.calls)
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM cmdb_relationship_revision`).Scan(&n); err != nil || n != 14309 {
			t.Fatalf("replay revisions %d %v", n, err)
		}
	})
	t.Run("replacement versions preserve observations", func(t *testing.T) {
		ctx, tx := batchRelationTestTx(t)
		batchRelationAssets(t, ctx, tx, map[string]string{"d1": "device", "r1": "rack", "r2": "rack"})
		old := batchRelation("located_in", "d1", "r1")
		observed := old
		observed.Source.Layer = "observed"
		observed.Source.Ref = "observation:test"
		if err := svc.writeEngineeringRelations(ctx, tx, schema, "test-project", []RelationInput{old, observed}); err != nil {
			t.Fatal(err)
		}
		next := batchRelation("located_in", "d1", "r2")
		next.Source.Ref = "engineering:next"
		if err := svc.writeEngineeringRelations(ctx, tx, schema, "test-project", []RelationInput{next}); err != nil {
			t.Fatal(err)
		}
		var active, historical, versions int
		if err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE valid_to IS NULL),count(*) FILTER(WHERE valid_to IS NOT NULL AND version=2 AND relationship_status='historical') FROM cmdb_relationship`).Scan(&active, &historical); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM cmdb_relationship_revision`).Scan(&versions); err != nil {
			t.Fatal(err)
		}
		if active != 2 || historical != 1 || versions != 4 {
			t.Fatalf("active %d historical %d revisions %d", active, historical, versions)
		}
		var before string
		if err := tx.QueryRow(ctx, `SELECT payload->'before'->>'to_uid' FROM cmdb_relationship_revision WHERE version=2`).Scan(&before); err != nil || before != "r1" {
			t.Fatalf("before snapshot %s %v", before, err)
		}
	})
	t.Run("batch and observed other project cycle", func(t *testing.T) {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprint(existing), func(t *testing.T) {
				ctx, tx := batchRelationTestTx(t)
				batchRelationAssets(t, ctx, tx, map[string]string{"d1": "device", "d2": "device", "d3": "device"})
				inputs := []RelationInput{batchRelation("contains", "d1", "d2"), batchRelation("contains", "d2", "d3"), batchRelation("contains", "d3", "d1")}
				if existing {
					first := inputs[0]
					first.Source.Layer = "observed"
					if _, err := svc.PutRelation(ctx, tx, schema, first); err != nil {
						t.Fatal(err)
					}
					if _, err := tx.Exec(ctx, `UPDATE cmdb_relationship SET project_id='other-project'`); err != nil {
						t.Fatal(err)
					}
					inputs = inputs[1:]
				}
				err := svc.writeEngineeringRelations(ctx, tx, schema, "test-project", inputs)
				if err == nil || err.Error() != "RELATIONSHIP_CYCLE" {
					t.Fatalf("cycle: %v", err)
				}
				var n int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM cmdb_relationship WHERE fact_layer='planned'`).Scan(&n); err != nil || n != 0 {
					t.Fatalf("write before validation: %d %v", n, err)
				}
			})
		}
	})
	t.Run("depth limit", func(t *testing.T) {
		ctx, tx := batchRelationTestTx(t)
		assets := map[string]string{}
		inputs := []RelationInput{}
		for i := 0; i < 34; i++ {
			assets[fmt.Sprint(i)] = "device"
			if i > 0 {
				inputs = append(inputs, batchRelation("contains", fmt.Sprint(i-1), fmt.Sprint(i)))
			}
		}
		batchRelationAssets(t, ctx, tx, assets)
		err := svc.writeEngineeringRelations(ctx, tx, schema, "test-project", inputs)
		if err == nil || err.Error() != "RELATIONSHIP_DEPTH_UNVERIFIED" {
			t.Fatal(err)
		}
	})
	t.Run("cable retarget and cross-end conflict", func(t *testing.T) {
		ctx, tx := batchRelationTestTx(t)
		batchRelationAssets(t, ctx, tx, map[string]string{"c1": "cable", "c2": "cable", "p1": "port", "p2": "port", "p3": "port"})
		if err := svc.writeEngineeringRelations(ctx, tx, schema, "test-project", []RelationInput{batchRelation("cable_endpoint_a", "c1", "p1"), batchRelation("cable_endpoint_b", "c1", "p2")}); err != nil {
			t.Fatal(err)
		}
		if err := svc.writeEngineeringRelations(ctx, tx, schema, "test-project", []RelationInput{batchRelation("cable_endpoint_a", "c1", "p3"), batchRelation("cable_endpoint_a", "c2", "p1")}); err != nil {
			t.Fatal(err)
		}
		err := svc.writeEngineeringRelations(ctx, tx, schema, "test-project", []RelationInput{batchRelation("cable_endpoint_b", "c2", "p3")})
		if err == nil || err.Error() != "PORT_ALREADY_CABLED" {
			t.Fatal(err)
		}
	})
}
