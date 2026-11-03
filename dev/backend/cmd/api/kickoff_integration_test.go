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
	"keel.local/dev/internal/bom"
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

func TestKickoffBeforeAgent(t *testing.T) {
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
	// Real XLSX upload to a private test bucket. Candidate facts below are prepared
	// independently for pre-Agent verification; this does not claim Agent analysis.
	book := excelize.NewFile()
	book.SetSheetRow("Sheet1", "A1", &[]string{"type", "name", "SN", "model"})
	book.SetSheetRow("Sheet1", "A2", &[]string{"server", "A", "SYNTH-A", "fixture"})
	book.SetSheetRow("Sheet1", "A3", &[]string{"switch", "B", "SYNTH-B", "fixture"})
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
	m := engineering.EmptyPhysical(project)
	modelEvidence := []engineering.SourceEvidence{}
	v := func(kind, key string) engineering.EntityVersion {
		id := engineering.StableID(project, kind, key)
		evidence := engineering.StableID(project, "evidence", key)
		modelEvidence = append(modelEvidence, engineering.SourceEvidence{ID: evidence, EntityID: id, Sheet: "Sheet1", Cell: "A2", Field: kind})
		return engineering.EntityVersion{EntityID: id, EvidenceIDs: []string{evidence}, Maturity: engineering.Confirmed}
	}
	serialA, serialB := "SYNTH-A", "SYNTH-B"
	a := engineering.EquipmentPlan{EntityVersion: v("equipment", "a"), EquipmentType: "server", Label: "A", Serial: &serialA, SourceModelLabel: "fixture", Quantity: 1, IdentityKind: "planned_instance"}
	b := engineering.EquipmentPlan{EntityVersion: v("equipment", "b"), EquipmentType: "switch", Label: "B", Serial: &serialB, SourceModelLabel: "fixture", Quantity: 0, IdentityKind: "planned_instance"}
	room := engineering.FacilityModel{EntityVersion: v("room", "room"), Kind: "room", Label: "Room"}
	capacity, height, start := 42, 2, 1
	rack1 := engineering.FacilityModel{EntityVersion: v("rack", "r1"), Kind: "rack", Label: "R1", CapacityU: &capacity, ParentID: &room.EntityID}
	rack2 := engineering.FacilityModel{EntityVersion: v("rack", "r2"), Kind: "rack", Label: "R2", CapacityU: &capacity, ParentID: &room.EntityID}
	m.Equipment = []engineering.EquipmentPlan{a, b}
	m.Facilities = []engineering.FacilityModel{room, rack1, rack2}
	m.Placements = []engineering.RackPlacement{{EntityVersion: v("placement", "a"), EquipmentID: a.EntityID, RackID: rack1.EntityID, StartU: &start}, {EntityVersion: v("placement", "b"), EquipmentID: b.EntityID, RackID: rack2.EntityID, StartU: &start, HeightU: &height}}
	p1 := engineering.PlannedPort{EntityVersion: v("port", "a"), EquipmentID: a.EntityID, Name: "eth1", Role: "network", Medium: "DAC"}
	p2 := engineering.PlannedPort{EntityVersion: v("port", "b"), EquipmentID: b.EntityID, Name: "eth1", Role: "network", Medium: "DAC"}
	m.Ports = []engineering.PlannedPort{p1, p2}
	m.Connections = []engineering.PlannedConnection{{EntityVersion: v("connection", "ab"), APortID: p1.EntityID, ZPortID: p2.EntityID}}
	var engVersion int
	db.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1`, source).Scan(&engVersion)
	if _, e = svc.saveEngineeringDraft(ctx, source, engVersion, "human", actor, "prepared candidates; Agent not run", engineering.Extraction{Model: m, Evidence: modelEvidence}, engineering.EngineeringProposal{Summary: "Prepared test fixture"}); e != nil {
		t.Fatal(e)
	}
	save := func(table string, rows []map[string]string) {
		t.Helper()
		call("PUT", "models/"+table, map[string]any{"rows": rows, "source": "explicit synthetic input", "expectedRevision": version()}, 200)
	}
	save("rooms", []map[string]string{{"room_id": room.EntityID, "name": "Room updated"}})
	call("PUT", "models/rooms", map[string]any{"rows": []map[string]string{}, "source": "stale", "expectedRevision": 1}, 409)
	save("route_nodes", []map[string]string{{"code": "A"}, {"code": "B"}})
	save("route_edges", []map[string]string{{"code": "e", "from": "A", "to": "B", "distance_m": "6"}})
	save("rack_access", []map[string]string{{"rack_id": rack1.EntityID, "node": "A", "lead_m": "1"}, {"rack_id": rack2.EntityID, "node": "B", "lead_m": "1"}})
	save("cable_specs", []map[string]string{{"code": "s", "medium": "DAC", "length_m": "10"}})
	save("cabling_rules", []map[string]string{{"code": "r", "version": "1", "strategy": "shortest_path", "source": "independent reference", "cable_spec": "s", "slack_m": "2", "slack_percent": "0"}})
	var calculation kickoff.Calculation
	json.Unmarshal(call("POST", "calculations", map[string]any{"domain": "cabling", "expectedRevision": version()}, 201), &calculation)
	if len(calculation.Rows) != 1 || calculation.Rows[0].CalculatedM != 10 {
		t.Fatalf("calculation: %+v", calculation)
	}
	var publication kickoff.Publication
	publicationInputVersion := version()
	json.Unmarshal(call("POST", "publications", map[string]any{"deviceIds": []string{a.EntityID}, "reason": "confirm valid subset", "expectedRevision": version()}, 202), &publication)
	if publication.State != "pending" || len(publication.DeviceIDs) != 1 {
		t.Fatalf("enqueue: %+v", publication)
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var event string
	var raw []byte
	e = tx.QueryRow(ctx, `SELECT id::text,payload FROM outbox_event WHERE event_type='kickoff.published' AND project_id=$1`, project).Scan(&event, &raw)
	if e == nil {
		e = svc.projectKickoffPublication(ctx, tx, event, project, raw)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE outbox_event SET published_at=now() WHERE id=$1`, event)
	}
	if e != nil {
		tx.Rollback(ctx)
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(call("GET", "publications/"+publication.ID, nil, 200), &publication)
	if publication.State != "published" || publication.Counts["devices"] != 1 {
		t.Fatalf("receipt: %+v", publication)
	}
	var count int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1 AND asset_id=$2`, project, a.EntityID).Scan(&count); e != nil || count != 1 {
		t.Fatalf("CMDB missing: %d %v", count, e)
	}
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1 AND asset_id=$2`, project, b.EntityID).Scan(&count); e != nil || count != 0 {
		t.Fatalf("held device leaked: %d %v", count, e)
	}
	// Partial location is committed independently of geometric occupancy.
	if len(publication.PlacementIDs) != 0 || len(publication.RackSnapshot) != 1 || publication.DeviceSnapshot[0].RackID != rack1.EntityID || publication.DeviceSnapshot[0].StartU == nil {
		t.Fatalf("partial location receipt: %+v", publication)
	}
	var locationCount int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_relationship WHERE project_id=$1 AND from_uid=$2 AND to_uid=$3 AND definition_id='located_in' AND fact_layer='planned' AND valid_to IS NULL`, project, a.EntityID, rack1.EntityID).Scan(&locationCount); e != nil || locationCount != 1 {
		t.Fatalf("known rack relationship missing: %v", e)
	}
	var geometryUnknown bool
	if e = db.QueryRow(ctx, `SELECT NOT (payload ? 'u_height') AND payload->>'u_start'='1' FROM cmdb_entity WHERE item_uid=$1`, a.EntityID).Scan(&geometryUnknown); e != nil || !geometryUnknown {
		t.Fatalf("partial geometry lost or invented: %v", e)
	}
	save("racks", []map[string]string{{"rack_id": rack1.EntityID, "capacity_u": "48"}})
	var previous kickoff.Publication
	json.Unmarshal(call("GET", "publications/"+publication.ID, nil, 200), &previous)
	if previous.RackSnapshot[0].CapacityU == nil || *previous.RackSnapshot[0].CapacityU != 42 || len(previous.PlacementIDs) != 0 {
		t.Fatal("later input changed the old publication snapshot")
	}
	// Repeated confirmation of the exact scope is idempotent.
	var retry kickoff.Publication
	json.Unmarshal(call("POST", "publications", map[string]any{"deviceIds": []string{a.EntityID}, "reason": "confirm valid subset", "expectedRevision": publicationInputVersion}, 202), &retry)
	if retry.ID != publication.ID {
		t.Fatal("retry created a second publication")
	}
	call("POST", "sources/"+source+"/analyze", map[string]any{"expectedRevision": version()}, 503)
	if path := os.Getenv("KEEL_KICKOFF_TEST_ARTIFACT"); path != "" {
		state := snapshot()
		payload, _ := json.MarshalIndent(state, "", "  ")
		os.WriteFile(path, payload, 0600)
	}
	// Historical replay uses a distinct fresh project workspace, its original
	// private object and prepared typed candidate. No model provider is called.
	historyProject := "kickoff-history-" + uuid.NewString()
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'History replay fixture',$1)`, historyProject); e != nil {
		t.Fatal(e)
	}
	historical := *svc
	historical.project = historyProject
	batch := uuid.NewString()
	key := historyProject + "/original-fixture.xlsx"
	if _, e = objects.PutObject(ctx, bucket, key, bytes.NewReader(xlsx.Bytes()), int64(xlsx.Len()), minio.PutObjectOptions{}); e != nil {
		t.Fatal(e)
	}
	workbook := bom.Workbook{Sheets: []bom.Sheet{{Name: "Sheet1", Dimension: "A1:D3", Cells: []bom.Cell{{Ref: "C2", Row: 2, Column: 3, Value: "SYNTH-A"}}}}}
	if _, e = db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,workbook,import_mode) VALUES($1,$2,$1,$3,'history-fixture.xlsx','synthetic-hash','MAPPING',$4,'engineering')`, batch, historyProject, key, jsonBytes(workbook)); e != nil {
		t.Fatal(e)
	}
	legacy, e := historical.attachEngineering(ctx, batch, actor)
	if e != nil {
		t.Fatal(e)
	}
	canonical := engineering.StableID(historyProject, "equipment", "existing")
	proof := uuid.NewString()
	candidate := engineering.EmptyPhysical(historyProject)
	candidate.Equipment = []engineering.EquipmentPlan{{EntityVersion: engineering.EntityVersion{EntityID: canonical, EvidenceIDs: []string{proof}, Maturity: engineering.Confirmed}, EquipmentType: "server", Label: "A", Serial: &serialA, SourceModelLabel: "fixture", Quantity: 1, IdentityKind: "planned_instance"}}
	if _, e = historical.saveEngineeringDraft(ctx, legacy, 1, "human", actor, "prepared historical candidate", engineering.Extraction{Model: candidate, Evidence: []engineering.SourceEvidence{{ID: proof, EntityID: canonical, Sheet: "Sheet1", Cell: "C2", Field: "serial"}}}, engineering.EngineeringProposal{}); e != nil {
		t.Fatal(e)
	}
	request := httptest.NewRequest("POST", "/api/v1/projects/"+historyProject+"/kickoff/history", bytes.NewReader(jsonBytes(map[string]any{"sourceId": legacy, "expectedRevision": 1})))
	request = request.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: subject}))
	response := httptest.NewRecorder()
	historical.kickoffRoutes(response, request)
	if response.Code != 201 {
		t.Fatalf("history replay: %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest("GET", "/api/v1/projects/"+historyProject+"/kickoff/workspace", nil)
	request = request.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: subject}))
	response = httptest.NewRecorder()
	historical.kickoffRoutes(response, request)
	if response.Code != 200 {
		t.Fatalf("history snapshot: %d %s", response.Code, response.Body.String())
	}
	var historicalState struct {
		Data struct {
			Devices []kickoff.Device `json:"devices"`
			Sources []kickoffSource  `json:"sources"`
		} `json:"data"`
	}
	json.Unmarshal(response.Body.Bytes(), &historicalState)
	if len(historicalState.Data.Devices) != 1 || historicalState.Data.Devices[0].ID != canonical || len(historicalState.Data.Sources) != 1 || len(historicalState.Data.Sources[0].Regions) == 0 {
		t.Fatalf("historical identity or metadata lost: %+v", historicalState)
	}
	// Run the existing project-cleanup SQL sequence: new module foreign keys
	// must not reintroduce the previously fixed project deletion failure.
	cleanup, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, statement := range projectDeletionStatements() {
		args := []any{project}
		if !strings.Contains(statement, "$1") {
			args = nil
		}
		if _, e = cleanup.Exec(ctx, statement, args...); e != nil {
			cleanup.Rollback(ctx)
			t.Fatal(e)
		}
	}
	if e = cleanup.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(ctx, `SELECT count(*) FROM audit_event WHERE project_id=$1 AND action='kickoff.confirmed'`, project).Scan(&count); e != nil || count != 1 {
		t.Fatal("cleanup did not retain publication audit", e)
	}
	t.Log("PASS real XLSX object upload; typed candidates prepared without Agent; input adoption; stale revision rejection; deterministic 10m calculation; isolated subset; Outbox CMDB receipt; Agent explicitly unavailable")
}
