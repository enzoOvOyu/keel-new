package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/xuri/excelize/v2"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/cmdb"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/identity"
	"keel.local/dev/internal/kickoff"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestKickoffNoSNAdoptionPublication(t *testing.T) {
	dsn := os.Getenv("KEEL_KICKOFF_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "/keel_engineering_kickoff_") {
		t.Skip("requires isolated kickoff database")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, name := range []string{"048_kickoff_workspace.sql", "049_kickoff_execution.sql", "050_kickoff_history.sql", "051_kickoff_location_closure.sql"} {
		version, _ := strconv.Atoi(name[:3])
		var applied bool
		if e = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migration WHERE version=$1)`, version).Scan(&applied); e != nil {
			t.Fatal(e)
		}
		if applied {
			continue
		}
		raw, e := os.ReadFile("../../migrations/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(ctx, string(raw)); e != nil {
			t.Fatal(e)
		}
	}
	if e = (cmdb.Service{DB: db}).Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	objects, e := minio.New(os.Getenv("KEEL_KICKOFF_TEST_S3_ENDPOINT"), &minio.Options{Creds: credentials.NewStaticV4(os.Getenv("KEEL_KICKOFF_TEST_S3_USER"), os.Getenv("KEEL_KICKOFF_TEST_S3_PASSWORD"), ""), Secure: false})
	if e != nil {
		t.Fatal(e)
	}
	bucket := "kickoff-check-" + uuid.NewString()
	if e = objects.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); e != nil {
		t.Fatal(e)
	}
	defer func() {
		for item := range objects.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if item.Err == nil {
				objects.RemoveObject(ctx, bucket, item.Key, minio.RemoveObjectOptions{})
			}
		}
		objects.RemoveBucket(ctx, bucket)
	}()
	project := "kickoff-synthetic-" + uuid.NewString()
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Kickoff acceptance',$1)`, project); e != nil {
		t.Fatal(e)
	}
	svc := &Server{db: db, objects: objects, bucket: bucket, project: project}
	actor := uuid.NewString()
	subject := identity.Subject{ID: actor, Username: "kickoff-test", GlobalPermissions: []string{"bom:read", "bom:edit", "bom:import", "bom:review", "bom:execute"}}
	ctx = context.WithValue(ctx, principalKey{}, principal{Subject: subject})
	call := func(method, path string, body any, status int) json.RawMessage {
		t.Helper()
		var payload []byte
		if body != nil {
			payload = jsonBytes(body)
		}
		r := httptest.NewRequest(method, "/api/v1/projects/"+project+"/kickoff/"+path, bytes.NewReader(payload))
		r = r.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: subject}))
		w := httptest.NewRecorder()
		if !svc.kickoffRoutes(w, r) {
			t.Fatal("route not handled")
		}
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		var out struct {
			Data json.RawMessage `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &out)
		return out.Data
	}
	snapshot := func() map[string]json.RawMessage {
		t.Helper()
		var state map[string]json.RawMessage
		if e := json.Unmarshal(call("GET", "workspace", nil, 200), &state); e != nil {
			t.Fatal(e)
		}
		return state
	}
	version := func() int { var n int; json.Unmarshal(snapshot()["revision"], &n); return n }
	call("GET", "workspace", nil, 200)
	// Real synthetic XLSX upload; deterministic candidate injection replaces AI only.
	book := excelize.NewFile()
	book.SetSheetRow("Sheet1", "A1", &[]string{"a_device_ref", "a_port", "b_device_ref", "b_port", "cable_length_m"})
	book.SetSheetRow("Sheet1", "A2", &[]string{"Node A", "eth1", "Switch B", "eth2", "3.5"})
	xlsx, _ := book.WriteToBuffer()
	book.Close()
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, _ := writer.CreateFormFile("file", "prepared-fixture.xlsx")
	part.Write(xlsx.Bytes())
	writer.Close()
	r := httptest.NewRequest("POST", "/api/v1/projects/"+project+"/kickoff/sources", &form)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r = r.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: subject}))
	w := httptest.NewRecorder()
	svc.kickoffRoutes(w, r)
	if w.Code != 201 {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	var uploaded struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &uploaded)
	source := uploaded.Data.ID

	connections := identityCandidate("connections", map[string]string{"a_device_ref": "Node A", "a_port": "eth1", "a_equipment_type": "server", "b_device_ref": "Switch B", "b_port": "eth2", "b_equipment_type": "switch", "cable_model": "Synthetic DAC", "cable_length_m": "3.5", "medium": "DAC"})
	// All evidence refers to the synthetic file only. Projection and adoption are real.
	for field, cell := range connections.Evidence[0].Fields {
		cell.Sheet = "Sheet1"
		cell.Cell = "A2"
		connections.Evidence[0].Fields[field] = cell
	}
	candidates := kickoffAgentProjectEndpoints([]ai.KickoffAgentCandidate{connections})
	candidates = append(candidates,
		identityCandidate("racks", map[string]string{"rack_id": "Rack-A", "capacity_u": "42"}),
		identityCandidate("placements", map[string]string{"device_id": " node   a ", "rack_id": "Rack-A", "start_u": "1", "height_u": "1"}, map[string]string{"device_id": "SWITCH B", "rack_id": "Rack-A", "start_u": "3", "height_u": "1"}),
	)
	job, claim := uuid.NewString(), uuid.NewString()
	var engVersion int
	if e = db.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1`, source).Scan(&engVersion); e != nil {
		t.Fatal(e)
	}
	request := kickoffAgentRequest{FlowVersion: "termination-v1", Kind: kickoffAgentKind, SourceID: source, Actor: actor, WorkspaceRevision: version(), BaseVersions: map[string]int{}}
	if _, e = db.Exec(ctx, `INSERT INTO eng_job(id,source_id,project_id,expected_revision,state,claim_id,proposal) VALUES($1,$2,$3,$4,'running',$5,$6)`, job, source, project, engVersion, claim, jsonBytes(request)); e != nil {
		t.Fatal(e)
	}
	if e = svc.saveAnalysisArtifact(ctx, job, claim, "kickoff_domain_result", "succeeded", ai.KickoffResult{FlowVersion: "termination-v1", Candidates: candidates}); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, `UPDATE eng_job SET state='succeeded' WHERE id=$1`, job); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.applyKickoffAdoption(ctx, source, kickoffAdoptBody{JobID: job, ExpectedRevision: version()}, actor, "", ""); e != nil {
		t.Fatal(e)
	}
	load := func() engineering.PhysicalModel {
		t.Helper()
		m, _, _, err := svc.kickoffModel(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	m := load()
	if len(m.Equipment) != 2 || len(m.Ports) != 2 || len(m.Connections) != 1 || len(m.Cables) != 1 || len(m.Placements) != 2 {
		t.Fatalf("persisted closure: devices=%d ports=%d connections=%d cables=%d placements=%d", len(m.Equipment), len(m.Ports), len(m.Connections), len(m.Cables), len(m.Placements))
	}
	ids := []string{}
	for _, device := range m.Equipment {
		if device.Serial != nil || device.IdentityKind != "planned_instance" || device.EntityID != engineering.StableID(project, "equipment_ref", kickoff.NormalizeDeviceRef(device.Label)) {
			t.Fatalf("persisted identity: %+v", device)
		}
		ids = append(ids, device.EntityID)
	}
	if m.Cables[0].Estimated == nil || m.Cables[0].Estimated.Length.Value != "3.5" || m.Cables[0].Estimated.Length.Unit != "m" || m.Cables[0].Spec.SourceModelLabel != "Synthetic DAC" {
		t.Fatalf("persisted cable: %+v", m.Cables[0])
	}
	var beforeEntities, afterEntities int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM eng_entity WHERE project_id=$1`, project).Scan(&beforeEntities); e != nil {
		t.Fatal(e)
	}
	before, rev := engineering.Digest(m), version()
	if _, e = svc.applyKickoffAdoption(ctx, source, kickoffAdoptBody{JobID: job, ExpectedRevision: rev}, actor, "", ""); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(ctx, `SELECT count(*) FROM eng_entity WHERE project_id=$1`, project).Scan(&afterEntities); e != nil {
		t.Fatal(e)
	}
	if beforeEntities != afterEntities || engineering.Digest(load()) != before || version() != rev {
		t.Fatal("repeat adoption changed entity IDs/model/revision")
	}
	var deviceReview struct {
		Items []kickoffReviewRow `json:"rows"`
	}
	if e = json.Unmarshal(call("GET", "review?category=devices", nil, 200), &deviceReview); e != nil {
		t.Fatal(e)
	}
	if len(deviceReview.Items) != 2 {
		t.Fatalf("review devices: %+v", deviceReview)
	}
	for _, row := range deviceReview.Items {
		if row.Facts.Serial != "" || row.Facts.RackID == "" || row.Facts.StartU == nil || row.Facts.HeightU == nil {
			t.Fatalf("review location: %+v", row.Facts)
		}
	}
	var connectionReview struct {
		Items []struct {
			Facts map[string]any `json:"facts"`
		} `json:"rows"`
	}
	if e = json.Unmarshal(call("GET", "review?category=connections", nil, 200), &connectionReview); e != nil {
		t.Fatal(e)
	}
	if len(connectionReview.Items) != 1 {
		t.Fatal("review connection missing")
	}
	facts := connectionReview.Items[0].Facts
	// Keep these assertions at the end too, so publication evidence is collected even if review is incomplete.
	var publication kickoff.Publication
	if e = json.Unmarshal(call("POST", "publications", map[string]any{"deviceIds": ids, "expectedRevision": version(), "reason": "synthetic no-SN acceptance"}, 202), &publication); e != nil {
		t.Fatal(e)
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	var event string
	var payload []byte
	if e = tx.QueryRow(ctx, `SELECT id::text,payload FROM outbox_event WHERE project_id=$1 AND event_type='kickoff.published'`, project).Scan(&event, &payload); e != nil {
		t.Fatal(e)
	}
	if e = svc.projectKickoffPublication(ctx, tx, event, project, payload); e != nil {
		t.Fatal(e)
	}
	// The same outbox delivery must not create a second asset or relationship.
	if e = svc.projectKickoffPublication(ctx, tx, event, project, payload); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `UPDATE outbox_event SET published_at=now() WHERE id=$1`, event); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(call("GET", "publications/"+publication.ID, nil, 200), &publication); e != nil {
		t.Fatal(e)
	}
	if publication.State != "published" || publication.Counts["devices"] != 2 || publication.Counts["ports"] != 2 || publication.Counts["connections"] != 1 || publication.Counts["placements"] != 2 {
		t.Fatalf("receipt: %+v", publication)
	}
	for _, id := range ids {
		var serial, layer string
		if e = db.QueryRow(ctx, `SELECT COALESCE(e.payload->>'serial',''),(SELECT fact_layer FROM cmdb_field_source f WHERE f.asset_id=e.item_uid AND f.project_id=$1 AND f.source_ref=$3 LIMIT 1) FROM cmdb_entity e JOIN cmdb_asset_scope s ON s.asset_id=e.item_uid AND s.project_id=$1 WHERE e.item_uid=$2`, project, id, "engineering:"+publication.ID).Scan(&serial, &layer); e != nil || serial != "" || layer != "planned" {
			t.Fatalf("CMDB identity/fact layer: %q %q %v", serial, layer, e)
		}
	}
	var cmdbCable map[string]any
	var cableRaw []byte
	if e = db.QueryRow(ctx, `SELECT jsonb_object_agg(f.field_name,f.value) FROM cmdb_entity e JOIN cmdb_field_source f ON f.asset_id=e.item_uid WHERE e.item_uid=$1 AND f.project_id=$2 AND f.source_ref=$3 AND f.fact_layer='planned'`, m.Connections[0].EntityID, project, "engineering:"+publication.ID).Scan(&cableRaw); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(cableRaw, &cmdbCable); e != nil {
		t.Fatal(e)
	}
	if cmdbCable["length_m"] != float64(3.5) || cmdbCable["model"] != "Synthetic DAC" {
		t.Errorf("CMDB cable attributes lost: %+v", cmdbCable)
	}
	if facts["cableLengthM"] != float64(3.5) && facts["cableLengthM"] != "3.5" || facts["cableModel"] != "Synthetic DAC" {
		t.Errorf("review cable attributes lost: %+v", facts)
	}
	t.Logf("isolated no-SN closure: devices=2 ports=2 connections=1 placements=2, repeat adoption entities=%d/%d, publication=%s", beforeEntities, afterEntities, publication.State)
}
