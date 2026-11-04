package cmdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/engineering"
)

// Run through check-kickoff-backend.py --package ./internal/cmdb --run
// '^TestEngineeringBatchProjection$'. Only KEEL_CMDB_BATCH_SCALE=full enables
// 809 devices, 14308 ports, 7154 cables, 88 racks and 805 known locations.
// This is a direct CMDB projection measurement, not AI/adoption or HTTP RBAC
// acceptance. All fixture/projection writes are rolled back; the runner owns
// schema cloning and disposal. No customer data or external object store is used.
func TestEngineeringBatchProjection(t *testing.T) {
	scale := os.Getenv("KEEL_CMDB_BATCH_SCALE")
	if scale != "" && scale != "small" && scale != "full" {
		t.Fatal("KEEL_CMDB_BATCH_SCALE must be small or full")
	}
	full := scale == "full"
	devices, ports, cables, racks, locations := 6, 20, 10, 3, 5
	if full {
		devices, ports, cables, racks, locations = 809, 14308, 7154, 88, 805
	}
	dsn := os.Getenv("KEEL_KICKOFF_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated KEEL_KICKOFF_TEST_DATABASE_URL supplied by kickoff runner")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid isolated database configuration") // Do not expose credentials.
	}
	const prefix = "keel_engineering_kickoff_"
	if !strings.HasPrefix(config.ConnConfig.Database, prefix) || len(config.ConnConfig.Database) == len(prefix) {
		t.Fatal("refusing a database outside the isolated keel_engineering_kickoff_* namespace")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	defer cancel()
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("cannot create isolated database pool")
	}
	defer db.Close()
	var database string
	if err = db.QueryRow(ctx, `SELECT current_database()`).Scan(&database); err != nil {
		t.Fatal("cannot verify isolated database identity")
	}
	if database != config.ConnConfig.Database {
		t.Fatal("connected database does not match the isolated configuration")
	}
	service := Service{DB: db}
	// The runner clones schema, not data. Install the existing built-in registry
	// only in this verified disposable DB, outside the measured projection.
	if err = service.Initialize(ctx); err != nil {
		t.Fatalf("initialize isolated CMDB registry: %v", err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := tx.Rollback(cleanup); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback isolated fixture: %v", err)
		}
	}()
	setupStarted := time.Now()
	project := "synthetic-cmdb-batch-" + uuid.NewString()
	publication, importID, actor := uuid.NewString(), uuid.NewString(), "synthetic-batch-projector"
	if _, err = tx.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic batch projection',$1)`, project); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,import_mode)
 VALUES($1,$2,$3,$4,'synthetic-batch.xlsx',$5,'committed','engineering')`, importID, project, uuid.NewString(), "synthetic/"+importID, Hash(project)); err != nil {
		t.Fatal(err)
	}
	model := engineeringBatchModel(project, publication, devices, ports, cables, racks, locations)
	if _, err = tx.Exec(ctx, `INSERT INTO bom_revision(id,import_id,project_id,digest) VALUES($1,$2,$3,$4)`, publication, importID, project, engineering.Digest(model)); err != nil {
		t.Fatal(err)
	}
	objects := engineering.ModelObjects(&model)
	// Only stable engineering identities are required by the projector; do not
	// construct AI artifacts, workbooks, adoption tasks or persisted model rows.
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"eng_entity"}, []string{"id", "project_id", "kind", "planning_key"}, pgx.CopyFromSlice(len(objects), func(i int) ([]any, error) {
		return []any{objects[i].Version.EntityID, project, objects[i].Kind, objects[i].Version.EntityID}, nil
	}))
	if err != nil {
		t.Fatalf("seed engineering identities: %v", err)
	}
	schema, err := txSchema(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	type preservedAsset struct {
		ID, Snapshot string
	}
	preserved := []preservedAsset{}
	for _, seed := range []struct {
		id, kind, layer, source string
		attrs                   map[string]any
	}{
		{model.Equipment[0].EntityID, "server", "observed", "discovery", map[string]any{"name": "Synthetic observed server", "hostname": "synthetic-observed-host", "lifecycle_state": "discovered"}},
		{model.Ports[0].EntityID, "port", "observed", "discovery", map[string]any{"name": "Synthetic observed port", "physical_name": "observed-port"}},
		{model.Connections[0].EntityID, "cable", "observed", "discovery", map[string]any{"name": "Synthetic observed cable", "length_m": float64(99)}},
		{model.Facilities[0].EntityID, "rack", "observed", "discovery", map[string]any{"name": "Synthetic observed rack", "u_height": float64(48)}},
		{model.Equipment[1].EntityID, "server", "recorded", "manual", map[string]any{"name": "Synthetic manually confirmed server", "hostname": "synthetic-confirmed-host"}},
	} {
		if _, err = service.UpsertAsset(ctx, tx, schema, AssetInput{ID: seed.id, Type: seed.kind, Project: project, Attributes: seed.attrs, Source: Source{Type: seed.source, Ref: "synthetic-protected:" + seed.id, Layer: seed.layer, Operator: actor, Approval: "approved", Freshness: "fresh", Support: "supported"}}); err != nil {
			t.Fatalf("seed protected %s: %v", seed.kind, err)
		}
		preserved = append(preserved, preservedAsset{seed.id, engineeringBatchProtectedSnapshot(t, ctx, tx, seed.id)})
	}
	t.Logf("cmdb_batch setup duration_ms=%d scale=%s devices=%d ports=%d cables=%d core_objects=%d racks=%d known_locations=%d", time.Since(setupStarted).Milliseconds(), map[bool]string{false: "small", true: "full"}[full], devices, ports, cables, devices+ports+cables, racks, locations)

	// Known rack associations deliberately have no U geometry. Mirror the
	// publisher's separation between precise placements and KnownLocations.
	frozen := model
	frozen.Placements = nil
	known := engineering.PhysicalModel{ProjectID: project, Equipment: model.Equipment, Placements: model.Placements}
	targets := []string{engineering.EquipmentTarget, engineering.FacilityTarget, engineering.PlacementTarget, engineering.ConnectivityTarget}
	type projectionStage struct {
		Phase      string `json:"phase"`
		Target     string `json:"target"`
		Count      int    `json:"count"`
		DurationMS int64  `json:"duration_ms"`
	}
	stages := []projectionStage{}
	setupMS := time.Since(setupStarted).Milliseconds()
	projectAll := func(t *testing.T, phase string) {
		t.Helper()
		start := time.Now()
		for _, target := range targets {
			stage := time.Now()
			if err := service.ProjectEngineering(ctx, tx, "synthetic-batch:"+publication+":"+target, project, publication, target, actor, frozen); err != nil {
				t.Fatalf("%s %s: %v", phase, target, err)
			}
			n := map[string]int{engineering.EquipmentTarget: devices, engineering.FacilityTarget: racks, engineering.PlacementTarget: 0, engineering.ConnectivityTarget: ports + cables}[target]
			elapsed := time.Since(stage).Milliseconds()
			stages = append(stages, projectionStage{phase, target, n, elapsed})
			t.Logf("cmdb_batch phase=%s target=%s count=%d duration_ms=%d", phase, target, n, elapsed)
		}
		stage := time.Now()
		if err := service.ProjectKnownLocations(ctx, tx, "synthetic-batch:"+publication+":known-locations", project, publication, actor, known); err != nil {
			t.Fatalf("%s known locations: %v", phase, err)
		}
		stages = append(stages, projectionStage{phase, "known_locations", locations, time.Since(stage).Milliseconds()})
		t.Logf("cmdb_batch phase=%s target=known_locations count=%d duration_ms=%d", phase, locations, stages[len(stages)-1].DurationMS)
		stages = append(stages, projectionStage{phase, "all", devices + ports + cables + racks, time.Since(start).Milliseconds()})
		t.Logf("cmdb_batch phase=%s target=all duration_ms=%d", phase, stages[len(stages)-1].DurationMS)
	}
	projectAll(t, "projection")
	verificationStarted := time.Now()
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := tx.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	check := func(label string, got, want int) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %d, want %d", label, got, want)
		}
	}
	assets := devices + ports + cables + racks
	check("scoped assets including canonical project", count(`SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1`, project), assets+1)
	for _, kind := range []struct {
		name string
		n    int
	}{{"server", devices}, {"port", ports}, {"cable", cables}, {"rack", racks}, {"project", 1}} {
		check(kind.name, count(`SELECT count(*) FROM cmdb_entity e JOIN cmdb_asset_scope s ON s.asset_id=e.item_uid WHERE s.project_id=$1 AND e.asset_type=$2`, project, kind.name), kind.n)
	}
	ref := "engineering:" + publication
	check("canonical BOM associations", count(`SELECT count(*) FROM project_asset WHERE project_id=$1 AND revision_id=$2`, project, publication), assets)
	check("planned provenance covers every projected asset", count(`SELECT count(DISTINCT asset_id) FROM cmdb_field_source WHERE project_id=$1 AND source_ref=$2 AND fact_layer='planned' AND superseded_at IS NULL`, project, ref), assets)
	check("invalid planning provenance", count(`SELECT count(*) FROM cmdb_field_source WHERE project_id=$1 AND source_ref=$2 AND (fact_layer<>'planned' OR source_type<>'bom' OR approval_status<>'approved' OR operator<>$3 OR NOT(evidence_refs @> jsonb_build_array($4::text)))`, project, ref, actor, publication), 0)
	check("duplicate active field provenance", count(`SELECT count(*) FROM (SELECT asset_id,field_name FROM cmdb_field_source WHERE project_id=$1 AND source_ref=$2 AND superseded_at IS NULL GROUP BY asset_id,field_name HAVING count(*)<>1) duplicates`, project, ref), 0)
	for _, relation := range []struct {
		name string
		n    int
	}{{"contains", ports}, {"cable_endpoint_a", cables}, {"cable_endpoint_b", cables}, {"located_in", locations}, {"project_association", assets}} {
		check(relation.name, count(`SELECT count(*) FROM cmdb_relationship WHERE project_id=$1 AND source_ref=$2 AND definition_id=$3 AND valid_to IS NULL`, project, ref, relation.name), relation.n)
	}
	check("active relationship total", count(`SELECT count(*) FROM cmdb_relationship WHERE project_id=$1 AND valid_to IS NULL`, project), assets+ports+2*cables+locations)
	check("unexpected planned relationship source", count(`SELECT count(*) FROM cmdb_relationship WHERE project_id=$1 AND fact_layer='planned' AND (source_ref<>$2 OR source_type<>'bom') AND valid_to IS NULL`, project, ref), 0)
	// Compare every endpoint/owner/location with the synthetic input, not just
	// aggregate counts, so swapped or dropped links cannot satisfy the test.
	expectedLinks := []map[string]string{}
	for _, p := range model.Ports {
		expectedLinks = append(expectedLinks, map[string]string{"kind": "contains", "a": p.EquipmentID, "z": p.EntityID})
	}
	for _, c := range model.Connections {
		expectedLinks = append(expectedLinks, map[string]string{"kind": "cable_endpoint_a", "a": c.EntityID, "z": c.APortID}, map[string]string{"kind": "cable_endpoint_b", "a": c.EntityID, "z": c.ZPortID})
	}
	for _, p := range model.Placements {
		expectedLinks = append(expectedLinks, map[string]string{"kind": "located_in", "a": p.EquipmentID, "z": p.RackID})
	}
	check("missing exact planned links", count(`SELECT count(*) FROM jsonb_to_recordset($3::jsonb) AS x(kind text,a text,z text) WHERE NOT EXISTS(SELECT 1 FROM cmdb_relationship r WHERE r.project_id=$1 AND r.source_ref=$2 AND r.definition_id=x.kind AND r.from_uid=x.a AND r.to_uid=x.z AND r.fact_layer='planned' AND r.valid_to IS NULL)`, project, ref, Encode(expectedLinks)), 0)
	expectedLengths := []map[string]any{}
	for i, connection := range model.Connections {
		expectedLengths = append(expectedLengths, map[string]any{"id": connection.EntityID, "length": float64(2 + i%17)})
	}
	check("missing or changed planned cable length", count(`SELECT count(*) FROM jsonb_to_recordset($3::jsonb) AS x(id text,length numeric) WHERE NOT EXISTS(SELECT 1 FROM cmdb_field_source f WHERE f.project_id=$1 AND f.source_ref=$2 AND f.asset_id=x.id AND f.field_name='length_m' AND f.fact_layer='planned' AND f.value=to_jsonb(x.length) AND f.superseded_at IS NULL)`, project, ref, Encode(expectedLengths)), 0)
	check("known locations still need geometry", count(`SELECT count(*) FROM cmdb_field_source WHERE project_id=$1 AND source_ref=$2 AND field_name='placement_status' AND value='"needs_input"'::jsonb AND superseded_at IS NULL`, project, ref), locations)
	check("no fabricated device U positions or heights", count(`SELECT count(*) FROM cmdb_field_source f JOIN cmdb_entity e ON e.item_uid=f.asset_id WHERE f.project_id=$1 AND f.source_ref=$2 AND e.asset_type='server' AND f.field_name IN('u_start','u_height') AND f.superseded_at IS NULL`, project, ref), 0)
	for _, asset := range preserved {
		if after := engineeringBatchProtectedSnapshot(t, ctx, tx, asset.ID); after != asset.Snapshot {
			t.Error("projection changed an observed/manual payload or its source facts")
		}
	}
	snapshot := func() []int {
		return []int{
			count(`SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1`, project),
			count(`SELECT count(*) FROM cmdb_revision WHERE project_id=$1`, project),
			count(`SELECT count(*) FROM cmdb_field_source WHERE project_id=$1`, project),
			count(`SELECT count(*) FROM cmdb_relationship WHERE project_id=$1`, project),
			count(`SELECT COALESCE(sum(version),0)::int FROM cmdb_entity e JOIN cmdb_asset_scope s ON s.asset_id=e.item_uid WHERE s.project_id=$1`, project),
			count(`SELECT count(*) FROM cmdb_lifecycle_event WHERE project_id=$1`, project),
			count(`SELECT count(*) FROM cmdb_ingest_event WHERE source_ref=$1`, ref),
		}
	}
	t.Logf("cmdb_batch verified assets=%d planned_relations=%d known_locations=%d protected_assets=%d field_sources=%d revisions=%d duration_ms=%d", assets, ports+2*cables+locations, locations, len(preserved), count(`SELECT count(*) FROM cmdb_field_source WHERE project_id=$1`, project), count(`SELECT count(*) FROM cmdb_revision WHERE project_id=$1`, project), time.Since(verificationStarted).Milliseconds())
	t.Logf("cmdb_batch_summary=%s", Encode(map[string]any{"scale": map[bool]string{false: "small", true: "full"}[full], "counts": map[string]int{"devices": devices, "ports": ports, "cables": cables, "core_objects": devices + ports + cables, "racks": racks, "known_locations": locations, "assets": assets, "project_assets": 1}, "setup_ms": setupMS, "verification_ms": time.Since(verificationStarted).Milliseconds(), "stages": stages}))
	if full {
		return // Do not double the performance run with replay/negative cases.
	}
	t.Run("replay", func(t *testing.T) {
		before := snapshot()
		projectAll(t, "replay")
		if after := snapshot(); !reflect.DeepEqual(before, after) {
			t.Errorf("replay changed asset/revision/source/relation/version/event counts: before=%v after=%v", before, after)
		}
	})
	for _, negative := range []struct {
		name, code string
		status     int
	}{
		{"foreign_project", "CROSS_PROJECT_ASSET_ASSOCIATION_DENIED", 403},
		{"identity_type_conflict", "ASSET_TYPE_IDENTITY_CONFLICT", 409},
	} {
		t.Run(negative.name, func(t *testing.T) {
			// Each rejection has its own savepoint, including any SQL error, so
			// negative cases cannot poison the surrounding measured transaction.
			sp, err := tx.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer sp.Rollback(context.Background())
			p := project
			device := model.Equipment[0]
			if negative.status == 403 {
				p = "synthetic-unrelated-" + uuid.NewString()
				if _, err = sp.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic unrelated project',$1)`, p); err != nil {
					t.Fatal(err)
				}
			} else {
				device.EquipmentType = "switch"
			}
			err = service.ProjectEngineering(ctx, sp, uuid.NewString(), p, publication, engineering.EquipmentTarget, actor, engineering.PhysicalModel{ProjectID: p, Equipment: []engineering.EquipmentPlan{device}})
			var rejected *Error
			if !errors.As(err, &rejected) || rejected.Code != negative.code || rejected.Status != negative.status {
				t.Fatalf("expected %s (%d), got %v", negative.code, negative.status, err)
			}
		})
	}
}

func engineeringBatchModel(project, revision string, devices, ports, cables, racks, locations int) engineering.PhysicalModel {
	m := engineering.PhysicalModel{ProjectID: project}
	version := func(kind string, i int) engineering.EntityVersion {
		return engineering.EntityVersion{EntityID: engineering.StableID(project, kind, fmt.Sprint(i)), RevisionID: revision, ProvenanceID: engineering.StableID(project, "synthetic-provenance", "batch"), EvidenceIDs: []string{engineering.StableID(project, "synthetic-evidence", fmt.Sprintf("%s/%d", kind, i))}, Maturity: engineering.Confirmed}
	}
	for i := 0; i < devices; i++ {
		m.Equipment = append(m.Equipment, engineering.EquipmentPlan{EntityVersion: version("equipment", i), EquipmentType: "server", SourceModelLabel: "Synthetic compute model", Quantity: 1, IdentityKind: "planned_instance", Label: fmt.Sprintf("Synthetic device %04d", i), Role: "compute"})
	}
	for i := 0; i < racks; i++ {
		capacity := 42
		m.Facilities = append(m.Facilities, engineering.FacilityModel{EntityVersion: version("facility", i), Kind: "rack", Code: fmt.Sprintf("SYN-RACK-%03d", i), Label: fmt.Sprintf("Synthetic rack %03d", i), CapacityU: &capacity})
	}
	for i := 0; i < ports; i++ {
		rate := int64(400000000000)
		m.Ports = append(m.Ports, engineering.PlannedPort{EntityVersion: version("port", i), EquipmentID: m.Equipment[i%devices].EntityID, Name: fmt.Sprintf("synthetic-port-%05d", i), RateBPS: &rate, Medium: "DAC", Role: "fabric", Connector: "QSFP-DD"})
	}
	for i := 0; i < cables; i++ {
		connection := engineering.PlannedConnection{EntityVersion: version("connection", i), APortID: m.Ports[2*i].EntityID, ZPortID: m.Ports[2*i+1].EntityID, TerminationType: "direct"}
		m.Connections = append(m.Connections, connection)
		m.Cables = append(m.Cables, engineering.CablePlan{EntityVersion: version("cable_plan", i), ConnectionID: connection.EntityID, Spec: engineering.CableSpec{SourceModelLabel: "Synthetic direct cable", Medium: "DAC", ConnectorA: "QSFP-DD", ConnectorZ: "QSFP-DD"}, Estimated: &engineering.LengthEvidence{Length: engineering.Quantity{Value: fmt.Sprint(2 + i%17), Unit: "m"}}})
	}
	for i := 0; i < locations; i++ {
		m.Placements = append(m.Placements, engineering.RackPlacement{EntityVersion: version("placement", i), EquipmentID: m.Equipment[i].EntityID, RackID: m.Facilities[i%racks].EntityID, MountFace: "front"})
	}
	return m
}

// Compare authoritative payload/columns and all non-planned facts. Versions
// may advance for new planning evidence; observed/manual content must not.
func engineeringBatchProtectedSnapshot(t *testing.T, ctx context.Context, tx pgx.Tx, id string) string {
	t.Helper()
	var snapshot string
	err := tx.QueryRow(ctx, `SELECT jsonb_build_object('payload',e.payload,'type',e.asset_type,'name',e.name,'serial',e.serial,'source_type',e.source_type,'source_ref',e.source_ref,'lifecycle',e.lifecycle_state,'freshness',e.freshness,'health',e.health,'maintenance',e.maintenance_state,
 'facts',COALESCE((SELECT jsonb_agg(to_jsonb(f) ORDER BY f.field_name,f.source_ref) FROM cmdb_field_source f WHERE f.asset_id=e.item_uid AND f.fact_layer<>'planned'),'[]'::jsonb))::text FROM cmdb_entity e WHERE e.item_uid=$1`, id).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
