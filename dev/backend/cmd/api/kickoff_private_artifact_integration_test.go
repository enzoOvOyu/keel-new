package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The artifact is supplied privately at runtime; never copy it into fixtures or logs.
func TestKickoffPrivateArtifactAdoption(t *testing.T) {
	artifactPath := os.Getenv("KEEL_KICKOFF_PRIVATE_ARTIFACT")
	if artifactPath == "" {
		t.Skip("requires private artifact path")
	}
	privateFile, err := os.Open(artifactPath)
	if err != nil {
		t.Fatal("cannot open private artifact")
	}
	var result ai.KickoffResult
	err = json.NewDecoder(privateFile).Decode(&result)
	privateFile.Close()
	if err != nil || result.FlowVersion != "termination-v1" {
		t.Fatal("private artifact must be a termination-v1 result")
	}
	counts := map[string]int{}
	for _, c := range result.Candidates {
		counts[c.TableID] += len(c.Rows)
	}
	if counts["devices"] == 0 || counts["ports"] == 0 || counts["connections"] == 0 || counts["placements"] == 0 {
		t.Fatal("private artifact has no complete termination candidate set")
	}
	started := time.Now()

	dsn := os.Getenv("KEEL_KICKOFF_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "/keel_engineering_kickoff_") {
		t.Skip("requires isolated kickoff database")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatalf("isolated setup/read operation failed (%T); private details suppressed", e)
	}
	defer db.Close()
	for _, name := range []string{"048_kickoff_workspace.sql", "049_kickoff_execution.sql", "050_kickoff_history.sql", "051_kickoff_location_closure.sql", "053_engineering_equipment_role.sql"} {
		version, _ := strconv.Atoi(name[:3])
		var applied bool
		if e = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migration WHERE version=$1)`, version).Scan(&applied); e != nil {
			t.Fatalf("isolated setup/read operation failed (%T); private details suppressed", e)
		}
		if applied {
			continue
		}
		raw, e := os.ReadFile("../../migrations/" + name)
		if e != nil {
			t.Fatalf("isolated setup/read operation failed (%T); private details suppressed", e)
		}
		if _, e = db.Exec(ctx, string(raw)); e != nil {
			t.Fatalf("isolated setup/read operation failed (%T); private details suppressed", e)
		}
	}
	if e = (cmdb.Service{DB: db}).Initialize(ctx); e != nil {
		t.Fatalf("isolated setup/read operation failed (%T); private details suppressed", e)
	}
	objects, e := minio.New(os.Getenv("KEEL_KICKOFF_TEST_S3_ENDPOINT"), &minio.Options{Creds: credentials.NewStaticV4(os.Getenv("KEEL_KICKOFF_TEST_S3_USER"), os.Getenv("KEEL_KICKOFF_TEST_S3_PASSWORD"), ""), Secure: false})
	if e != nil {
		t.Fatalf("isolated setup/read operation failed (%T); private details suppressed", e)
	}
	bucket := "kickoff-check-" + uuid.NewString()
	if e = objects.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); e != nil {
		t.Fatalf("isolated setup/read operation failed (%T); private details suppressed", e)
	}
	defer func() {
		for item := range objects.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if item.Err != nil {
				t.Error("private bucket listing cleanup failed")
				continue
			}
			if err := objects.RemoveObject(ctx, bucket, item.Key, minio.RemoveObjectOptions{}); err != nil {
				t.Error("private bucket object cleanup failed")
			}
		}
		if err := objects.RemoveBucket(ctx, bucket); err != nil {
			t.Error("private bucket cleanup failed")
		}
	}()

	project := "kickoff-synthetic-" + uuid.NewString()
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Kickoff acceptance',$1)`, project); e != nil {
		t.Fatalf("isolated setup/read operation failed (%T); private details suppressed", e)
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
			t.Fatalf("isolated HTTP request failed: method=%s status=%d; response suppressed", method, w.Code)
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
			t.Fatalf("isolated setup/read operation failed (%T); private details suppressed", e)
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
		t.Fatalf("synthetic upload failed: status=%d; response suppressed", w.Code)
	}
	var uploaded struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &uploaded)
	source := uploaded.Data.ID

	// The tiny synthetic upload anchors only this temporary project. Its contents
	// are not claimed to be the workbook behind the private candidate evidence.
	job, claim := uuid.NewString(), uuid.NewString()
	var engVersion int
	if err = db.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1`, source).Scan(&engVersion); err != nil {
		t.Fatal("source revision read failed")
	}
	request := kickoffAgentRequest{FlowVersion: result.FlowVersion, Kind: kickoffAgentKind, SourceID: source, Actor: actor, WorkspaceRevision: version(), BaseVersions: map[string]int{}}
	if _, err = db.Exec(ctx, `INSERT INTO eng_job(id,source_id,project_id,expected_revision,state,claim_id,proposal) VALUES($1,$2,$3,$4,'running',$5,$6)`, job, source, project, engVersion, claim, jsonBytes(request)); err != nil {
		t.Fatal("isolated job insert failed")
	}
	if err = svc.saveAnalysisArtifact(ctx, job, claim, "kickoff_domain_result", "succeeded", result); err != nil {
		t.Fatal("private artifact persistence failed")
	}
	if _, err = db.Exec(ctx, `UPDATE eng_job SET state='succeeded' WHERE id=$1`, job); err != nil {
		t.Fatal("isolated job state update failed")
	}
	t.Logf("private artifact loaded: devices=%d ports=%d connections=%d placements=%d; no AI call", counts["devices"], counts["ports"], counts["connections"], counts["placements"])
	receipt, err := svc.applyKickoffAdoption(ctx, source, kickoffAdoptBody{JobID: job, ExpectedRevision: version()}, actor, "", "")
	if err != nil {
		code := "suppressed"
		switch err.Error() {
		case "KICKOFF_CANDIDATE_INVALID", "REVISION_CONFLICT", "KICKOFF_ANALYSIS_NOT_READY", "KICKOFF_ANALYSIS_NOT_FOUND":
			code = err.Error()
		}
		t.Fatalf("private artifact adoption failed: code=%s elapsed=%s", code, time.Since(started).Round(time.Millisecond))
	}
	adopted, ok := receipt["adoptedTableIds"].([]string)
	if !ok {
		t.Fatal("adoption receipt missing table list")
	}
	for _, required := range []string{"devices", "ports", "connections", "racks", "placements"} {
		found := false
		for _, table := range adopted {
			found = found || table == required
		}
		if !found {
			t.Fatalf("required table not adopted: %s", required)
		}
	}
	m, _, _, err := svc.kickoffModel(ctx, db)
	if err != nil {
		t.Fatal("persisted model read failed")
	}
	if len(m.Equipment) != counts["devices"] || len(m.Ports) != counts["ports"] || len(m.Connections) != counts["connections"] {
		t.Fatalf("adopted counts differ: devices=%d/%d ports=%d/%d connections=%d/%d", len(m.Equipment), counts["devices"], len(m.Ports), counts["ports"], len(m.Connections), counts["connections"])
	}
	aliases := map[string]string{}
	deviceIDs := map[string]bool{}
	for _, d := range m.Equipment {
		deviceIDs[d.EntityID] = true
		aliases[kickoff.NormalizeDeviceRef(d.Label)] = d.EntityID
		if d.Serial != nil {
			aliases[kickoff.NormalizeDeviceRef(*d.Serial)] = d.EntityID
		}
	}
	racks := map[string]string{}
	for _, r := range m.Facilities {
		if r.Kind == "rack" {
			for _, key := range []string{r.EntityID, r.Code, r.Label} {
				if key != "" {
					racks[key] = r.EntityID
				}
			}
		}
	}
	placements := map[string]engineering.RackPlacement{}
	for _, p := range m.Placements {
		if !deviceIDs[p.EquipmentID] {
			t.Fatal("unmatched inventory placement entered model")
		}
		placements[p.EquipmentID] = p
	}
	expected, unmatched, normalized, unknown := map[string]bool{}, 0, 0, 0
	for _, candidate := range result.Candidates {
		if candidate.TableID != "placements" {
			continue
		}
		for _, row := range candidate.Rows {
			id := aliases[kickoff.NormalizeDeviceRef(row["device_id"])]
			if id == "" {
				unmatched++
				continue
			}
			expected[id] = true
			p, exists := placements[id]
			if !exists || racks[row["rack_id"]] == "" || p.RackID != racks[row["rack_id"]] {
				t.Fatal("matched placement lost known rack association")
			}
			raw := strings.TrimSpace(row["start_u"])
			n, parseErr := strconv.Atoi(raw)
			if parseErr != nil {
				// Independently check the explicit integer-U marker, without using the preparation helper.
				digits := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(strings.ToLower(raw), "-")), "u"))
				if digits != raw {
					n, parseErr = strconv.Atoi(digits)
					if parseErr == nil && n > 0 {
						normalized++
					}
				}
			}
			if parseErr == nil && n > 0 {
				if p.StartU == nil || *p.StartU != n {
					t.Fatal("explicit start U lost or changed")
				}
			} else {
				unknown++
				if p.StartU != nil {
					t.Fatal("unknown start U invented")
				}
			}
			if strings.TrimSpace(row["height_u"]) == "" && p.HeightU != nil {
				t.Fatal("missing placement height invented")
			}
		}
	}
	if len(placements) != len(expected) {
		t.Fatalf("placement closure differs: persisted=%d expected=%d unmatched=%d", len(placements), len(expected), unmatched)
	}
	t.Logf("adoption verified: devices=%d ports=%d connections=%d placements=%d unmatched=%d normalizedStartU=%d unknownStartU=%d elapsed=%s", len(m.Equipment), len(m.Ports), len(m.Connections), len(m.Placements), unmatched, normalized, unknown, time.Since(started).Round(time.Millisecond))
	// Verify real paginated review reads without printing identity or source values.
	for offset := 0; offset < len(m.Equipment); offset += 200 {
		var page struct {
			Rows  []kickoffReviewRow `json:"rows"`
			Total int                `json:"total"`
		}
		raw := call("GET", fmt.Sprintf("review?category=devices&offset=%d&limit=200", offset), nil, 200)
		if json.Unmarshal(raw, &page) != nil || page.Total != len(m.Equipment) || len(page.Rows) == 0 {
			t.Fatal("device review pagination/count mismatch")
		}
		for _, row := range page.Rows {
			if !deviceIDs[row.ID] {
				t.Fatal("unexpected review device identity")
			}
			if p, exists := placements[row.ID]; exists {
				if row.Facts.RackID != p.RackID || (row.Facts.StartU == nil) != (p.StartU == nil) || p.StartU != nil && *row.Facts.StartU != *p.StartU {
					t.Fatal("review lost rack/start U")
				}
			}
		}
	}
	var page struct {
		Rows  []kickoffReviewRow `json:"rows"`
		Total int                `json:"total"`
	}
	if json.Unmarshal(call("GET", "review?category=connections&limit=200", nil, 200), &page) != nil || page.Total != len(m.Connections) || len(page.Rows) == 0 {
		t.Fatal("connection review count mismatch")
	}
	var beforeEntities, afterEntities int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM eng_entity WHERE project_id=$1`, project).Scan(&beforeEntities); err != nil {
		t.Fatal("entity count failed")
	}
	before, rev := engineering.Digest(m), version()
	canonicalBefore := privateArtifactModelDigest(t, m)
	t.Logf("first adoption tables: adopted=%v skipped=%v", receipt["adoptedTableIds"], receipt["skippedTableIds"])
	repeatReceipt, err := svc.applyKickoffAdoption(ctx, source, kickoffAdoptBody{JobID: job, ExpectedRevision: rev}, actor, "", "")
	if err != nil {
		t.Fatal("repeat private adoption failed; details suppressed")
	}
	after, _, _, err := svc.kickoffModel(ctx, db)
	if err != nil {
		t.Fatal("repeat model read failed")
	}
	if err = db.QueryRow(ctx, `SELECT count(*) FROM eng_entity WHERE project_id=$1`, project).Scan(&afterEntities); err != nil {
		t.Fatal("repeat entity count failed")
	}
	afterRevision := version()
	afterDigest, canonicalAfter := engineering.Digest(after), privateArtifactModelDigest(t, after)
	t.Logf("repeat adoption diagnostics: entityCount=%d/%d revision=%d/%d rawDigest=%s/%s canonicalDigest=%s/%s adopted=%v skipped=%v", beforeEntities, afterEntities, rev, afterRevision, before, afterDigest, canonicalBefore, canonicalAfter, repeatReceipt["adoptedTableIds"], repeatReceipt["skippedTableIds"])
	if beforeEntities != afterEntities || canonicalBefore != canonicalAfter || afterRevision != rev {
		t.Fatal("repeat adoption changed identity counts/object contents/revision (only set ordering normalized)")
	}
	if before != afterDigest {
		t.Log("raw digest difference is exclusively identity/evidence collection ordering; all object fields and revision are unchanged")
	}
	connectionIDs := map[string]bool{}
	for _, c := range m.Connections {
		connectionIDs[c.EntityID] = true
	}
	withLength := 0
	for _, c := range m.Cables {
		if !connectionIDs[c.ConnectionID] {
			t.Fatal("cable plan references missing connection")
		}
		if c.Estimated != nil {
			if c.Estimated.Length.Validate() != nil || c.Estimated.Length.Unit != "m" {
				t.Fatal("invalid persisted planned cable length")
			}
			withLength++
		}
	}
	if len(m.Cables) == 0 {
		t.Fatal("private artifact lost all cable plans")
	}
	t.Logf("full closure before publication: devices=%d connections=%d placements=%d cablePlans=%d estimatedLengths=%d", len(m.Equipment), len(m.Connections), len(m.Placements), len(m.Cables), withLength)
	ids := make([]string, 0, len(m.Equipment))
	for _, d := range m.Equipment {
		ids = append(ids, d.EntityID)
	}
	issues := map[string]int{}
	for _, issue := range kickoff.ValidationIssues(m) {
		issues[issue.Code]++
	}
	t.Logf("pre-publication validation code counts: %v", issues)
	var publication kickoff.Publication
	if json.Unmarshal(call("POST", "publications", map[string]any{"deviceIds": ids, "expectedRevision": version(), "reason": "isolated private artifact acceptance"}, 202), &publication) != nil {
		t.Fatal("publication response decoding failed")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal("publication transaction start failed")
	}
	defer tx.Rollback(ctx)
	var event string
	var payload []byte
	if err = tx.QueryRow(ctx, `SELECT id::text,payload FROM outbox_event WHERE project_id=$1 AND event_type='kickoff.published'`, project).Scan(&event, &payload); err != nil {
		t.Fatal("publication outbox missing")
	}
	if err = svc.projectKickoffPublication(ctx, tx, event, project, payload); err != nil {
		t.Fatalf("isolated CMDB publication failed (%T); private details suppressed", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE outbox_event SET published_at=now() WHERE id=$1`, event); err != nil {
		t.Fatal("outbox update failed")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal("publication transaction commit failed")
	}
	if json.Unmarshal(call("GET", "publications/"+publication.ID, nil, 200), &publication) != nil {
		t.Fatal("publication receipt read failed")
	}
	if publication.State != "published" || publication.Counts["devices"] != len(m.Equipment) || publication.Counts["connections"] != len(m.Connections) || publication.Counts["ports"] != len(m.Ports) {
		t.Fatal("full publication receipt count mismatch")
	}
	var devices, connections, ports int
	if err = db.QueryRow(ctx, `SELECT count(*) FILTER(WHERE e.kind='equipment'),count(*) FILTER(WHERE e.kind='connection'),count(*) FILTER(WHERE e.kind='port') FROM cmdb_asset_scope s JOIN eng_entity e ON e.id::text=s.asset_id WHERE s.project_id=$1`, project).Scan(&devices, &connections, &ports); err != nil {
		t.Fatal("CMDB object verification failed")
	}
	if devices != len(m.Equipment) || connections != len(m.Connections) || ports != len(m.Ports) {
		t.Fatalf("CMDB closure count mismatch: devices=%d connections=%d ports=%d", devices, connections, ports)
	}
	t.Logf("isolated full publication PASS: devices=%d ports=%d connections=%d placements=%d knownLocations=%d", devices, ports, connections, publication.Counts["placements"], publication.Counts["locations"])

	t.Logf("private artifact adoption/review PASS: repeated entities=%d/%d; publication=published; elapsed=%s", beforeEntities, afterEntities, time.Since(started).Round(time.Millisecond))
}

// Compare all persisted fields, retaining meaningful array ordering (for example
// cable routes). Entity collections and evidence ID sets have no semantic order.
func privateArtifactModelDigest(t *testing.T, m engineering.PhysicalModel) string {
	t.Helper()
	var value any
	if err := json.Unmarshal(jsonBytes(m), &value); err != nil {
		t.Fatal("model comparison encoding failed")
	}
	var normalize func(any, string)
	normalize = func(v any, key string) {
		switch typed := v.(type) {
		case map[string]any:
			for field, child := range typed {
				normalize(child, field)
			}
		case []any:
			for _, child := range typed {
				normalize(child, "")
			}
			if key == "evidence_ids" {
				sort.Slice(typed, func(i, j int) bool { return typed[i].(string) < typed[j].(string) })
				return
			}
			if len(typed) > 0 {
				allEntities := true
				for _, child := range typed {
					object, ok := child.(map[string]any)
					if !ok {
						allEntities = false
						break
					}
					if _, ok = object["entity_id"].(string); !ok {
						allEntities = false
						break
					}
				}
				if allEntities {
					sort.Slice(typed, func(i, j int) bool {
						return typed[i].(map[string]any)["entity_id"].(string) < typed[j].(map[string]any)["entity_id"].(string)
					})
				}
			}
		}
	}
	normalize(value, "")
	return engineering.Digest(value)
}
