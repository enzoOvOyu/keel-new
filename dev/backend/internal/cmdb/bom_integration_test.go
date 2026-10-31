package cmdb

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/bom"
	"os"
	"testing"
	"time"
)

// Exercises an existing committed local BOM inside a rolled-back transaction.
func TestCommittedBOMProjectionReplay(t *testing.T) {
	if os.Getenv("KEEL_CMDB_INTEGRATION") != "1" {
		t.Skip("explicit local PostgreSQL integration run required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	var project string
	var raw []byte
	if e = tx.QueryRow(ctx, "SELECT project_id,payload FROM outbox_event WHERE event_type='bom.committed' AND published_at IS NOT NULL ORDER BY created_at DESC LIMIT 1").Scan(&project, &raw); e != nil {
		t.Fatal(e)
	}
	var event struct {
		Items    []bom.Item `json:"items"`
		Revision string     `json:"revision_id"`
	}
	if e = json.Unmarshal(raw, &event); e != nil {
		t.Fatal(e)
	}
	if len(event.Items) == 0 {
		t.Fatal("published BOM is empty")
	}
	var before int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM cmdb_entity").Scan(&before); e != nil {
		t.Fatal(e)
	}
	service := Service{DB: db}
	key := uuid.NewString()
	for i := 0; i < 2; i++ {
		if e = service.ProjectBOM(ctx, tx, key, project, event.Revision, event.Items); e != nil {
			t.Fatal(e)
		}
	}
	var after int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM cmdb_entity").Scan(&after); e != nil {
		t.Fatal(e)
	}
	if before != after {
		t.Fatalf("BOM replay duplicated identities: %d -> %d", before, after)
	}
	for _, item := range event.Items {
		var asset string
		if e = tx.QueryRow(ctx, "SELECT item_uid FROM cmdb_entity WHERE bom_item_uid=$1", item.UID).Scan(&asset); e != nil {
			t.Fatalf("missing canonical BOM association: %s", item.Category)
		}
		if item.Category == "cable" {
			var ends int
			if e = tx.QueryRow(ctx, "SELECT count(DISTINCT to_uid) FROM cmdb_relationship WHERE from_uid=$1 AND definition_id IN('cable_endpoint_a','cable_endpoint_b') AND valid_to IS NULL AND fact_layer='planned'", asset).Scan(&ends); e != nil {
				t.Fatal(e)
			}
			if ends != 2 {
				t.Fatal("published cable lacks two real port endpoints")
			}
		}
	}
	var records int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM cmdb_ingest_event WHERE event_id=$1", key).Scan(&records); e != nil {
		t.Fatal(e)
	}
	if records != 1 {
		t.Fatal("event replay is not idempotent")
	}
}

func TestCableEndpointRemovalCannotBypassIntegrityFromPort(t *testing.T) {
	if os.Getenv("KEEL_CMDB_INTEGRATION") != "1" {
		t.Skip("explicit local PostgreSQL integration run required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	var relation, port, project string
	var version int64
	if e = tx.QueryRow(ctx, "SELECT id::text,to_uid,project_id,version FROM cmdb_relationship WHERE from_uid='cable-srv-0001-01' AND definition_id='cable_endpoint_a' AND valid_to IS NULL LIMIT 1").Scan(&relation, &port, &project, &version); e != nil {
		t.Fatal(e)
	}
	schema, e := txSchema(ctx, tx)
	if e != nil {
		t.Fatal(e)
	}
	service := Service{DB: db}
	e = service.applyChanges(ctx, tx, schema, Scope{Projects: []string{project}, Actor: "synthetic-integrity-check"}, uuid.NewString(), project, []Change{{Op: "relationship.end", AssetID: port, RelationshipID: relation, ExpectedRevision: version}})
	if domain, ok := e.(*Error); !ok || domain.Code != "CABLE_BOTH_ENDPOINTS_REQUIRED" {
		t.Fatalf("single cable-end removal must be blocked, got %v", e)
	}
}

// Synthetic, isolated project; every write is rolled back. Exercise the actual
// topology SQL used by physical graphs, connection lists, matrices and traces.
func TestPhysicalComponentOwnerQuery(t *testing.T) {
	if os.Getenv("KEEL_CMDB_INTEGRATION") != "1" {
		t.Skip("explicit local PostgreSQL integration run required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	project := "test-bom-root-" + uuid.NewString()
	if _, e = tx.Exec(ctx, "INSERT INTO project(id,name,code) VALUES($1,'独立测试-事务回滚-BOM拓扑',$1)", project); e != nil {
		t.Fatal(e)
	}
	for id, kind := range map[string]string{"port": "port", "gpu": "gpu", "server-a": "server", "server-b": "server"} {
		if _, e = tx.Exec(ctx, "INSERT INTO cmdb_entity(item_uid,asset_type,name,payload,source_ref) VALUES($1,$2,$2,'{}','synthetic-root-regression')", project+id, kind); e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(ctx, "INSERT INTO cmdb_asset_scope(project_id,asset_id,source_ref) VALUES($1,$2,'synthetic-root-regression')", project, project+id); e != nil {
			t.Fatal(e)
		}
	}
	link := func(a, b string) {
		t.Helper()
		if _, e := tx.Exec(ctx, "INSERT INTO cmdb_relationship(from_uid,to_uid,kind,definition_id,project_id,source_ref) VALUES($1,$2,'contains','contains',$3,'synthetic-root-regression')", project+a, project+b, project); e != nil {
			t.Fatal(e)
		}
	}
	owner := func(scope Scope) string {
		t.Helper()
		var value string
		args := append(scope.Args(), "", project+"port")
		if e := tx.QueryRow(ctx, visualCTE()+"SELECT COALESCE((SELECT id FROM owner WHERE port_id=$4),'')", args...).Scan(&value); e != nil {
			t.Fatal(e)
		}
		return value
	}
	scope := Scope{Projects: []string{project}}
	link("gpu", "port")
	if owner(scope) != project+"gpu" {
		t.Fatal("component-owned port missing from topology query")
	}
	if owner(Scope{Projects: []string{"unrelated-project"}}) != "" {
		t.Fatal("component owner crossed project scope")
	}
	link("server-a", "gpu")
	if owner(scope) != project+"server-a" {
		t.Fatal("enclosing device precedence changed")
	}
	link("server-b", "gpu")
	if owner(scope) != "" {
		t.Fatal("ambiguous ownership was hidden by component fallback")
	}
}

// An explicitly selected local event supplies immutable source evidence; no
// real workbook values or identities are committed as a fixture.
func TestBOMProjectionDimensions(t *testing.T) {
	eventID := os.Getenv("KEEL_CMDB_TEST_OUTBOX_ID")
	if os.Getenv("KEEL_CMDB_INTEGRATION") != "1" || eventID == "" {
		t.Skip("explicit local dimensional BOM event required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var project string
	var raw []byte
	if err = tx.QueryRow(ctx, "SELECT project_id,payload FROM outbox_event WHERE id=$1", eventID).Scan(&project, &raw); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Items    []bom.Item `json:"items"`
		Revision string     `json:"revision_id"`
	}
	if err = json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	heightID := ""
	unknownRack := false
	for _, item := range event.Items {
		if item.Fields["u_height"] == "4U" {
			heightID = item.UID
		}
		if bom.AssetType(item) == "rack" && item.Fields["rack_height"] == "" {
			unknownRack = true
		}
	}
	if heightID == "" || !unknownRack {
		t.Fatal("selected event lacks explicit U height and unknown rack capacity")
	}
	if err = (Service{DB: db}).ProjectBOM(ctx, tx, uuid.NewString(), project, event.Revision, event.Items); err != nil {
		t.Fatal(err)
	}
	var value, typ string
	if err = tx.QueryRow(ctx, "SELECT payload->>'u_height',jsonb_typeof(payload->'u_height') FROM cmdb_entity WHERE bom_item_uid=$1", heightID).Scan(&value, &typ); err != nil {
		t.Fatal(err)
	}
	if value != "4" || typ != "number" {
		t.Fatal("explicit U height not converted to numeric CMDB attribute")
	}
}
