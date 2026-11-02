package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/identity"
)

func TestModelingDraftAdoptionAndHandoff(t *testing.T) {
	dsn := os.Getenv("KEEL_ENGINEERING_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "/keel_engineering_") || os.Getenv("S3_BUCKET") == "" {
		t.Skip("isolated engineering DB and bucket required")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	prepareEngineeringDB(t, db)
	client, err := minio.New(os.Getenv("S3_ENDPOINT"), &minio.Options{Creds: credentials.NewStaticV4(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""), Secure: false})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db, project: "modeling-test-" + uuid.NewString(), objects: client, bucket: os.Getenv("S3_BUCKET")}
	if _, err = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Isolated modeling closure',$1)`, s.project); err != nil {
		t.Fatal(err)
	}
	batch := uuid.NewString()
	if _, err = db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,import_mode,workbook) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic','MAPPING','engineering',$5)`, batch, s.project, uuid.NewString(), s.project+"/source", jsonBytes(bom.Workbook{Sheets: []bom.Sheet{}})); err != nil {
		t.Fatal(err)
	}
	source, err := s.attachEngineering(ctx, batch, "synthetic-author")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for object := range client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.project + "/", Recursive: true}) {
			if object.Err == nil {
				_ = client.RemoveObject(ctx, s.bucket, object.Key, minio.RemoveObjectOptions{})
			}
		}
		db.Exec(ctx, `DELETE FROM eng_source WHERE project_id=$1`, s.project)
	})
	get := func() engineering.Workspace {
		t.Helper()
		v, e := s.engineeringWorkspace(ctx, source)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	unique := uuid.New()
	serialA, serialB := "00001-"+source, "00002-"+source
	macA := fmt.Sprintf("02:%02x:%02x:%02x:%02x:01", unique[0], unique[1], unique[2], unique[3])
	macB := fmt.Sprintf("02:%02x:%02x:%02x:%02x:02", unique[0], unique[1], unique[2], unique[3])
	csvInput := func(table string, rows ...[]string) engineering.InputPreviewRequest {
		var b strings.Builder
		w := csv.NewWriter(&b)
		for _, v := range engineering.StepInputCatalog().Tables {
			if v.ID == table {
				head := []string{}
				for _, f := range v.Fields {
					head = append(head, f.Label)
				}
				w.Write(head)
				for _, row := range rows {
					for len(row) < len(head) {
						row = append(row, "")
					}
					for i, cell := range row {
						switch cell {
						case "00001":
							row[i] = serialA
						case "00002":
							row[i] = serialB
						case "02:00:00:00:00:01":
							row[i] = macA
						case "02:00:00:00:00:02":
							row[i] = macB
						}
					}
					w.Write(row)
				}
			}
		}
		w.Flush()
		return engineering.InputPreviewRequest{TableID: table, CSV: b.String()}
	}
	partial := csvInput("terminations", []string{"00001", "02:00:00:00:00:01", "eth1", "00002", "R1-U10", "", "eth2", "客户确认端接"})
	edit := engineering.ModelingInputEdit{ExpectedRevision: 1, InputVersion: 0, InputPreviewRequest: partial, Reason: "Independent synthetic reference"}
	if err = s.saveModelingInputs(ctx, get(), []engineering.ModelingInputEdit{edit}, "author"); err != nil {
		t.Fatal(err)
	}
	restarted := *s
	items, err := restarted.modelingInputs(ctx, get(), true)
	if err != nil || len(items) != 1 || items[0].Rows != 1 || len(items[0].Issues) == 0 || !strings.Contains(items[0].CSV, "00001") {
		t.Fatalf("draft did not survive reload: %v %+v", err, items)
	}
	if get().Revision != 1 {
		t.Fatal("saving partial input changed formal model")
	}
	if err = s.saveModelingInputs(ctx, get(), []engineering.ModelingInputEdit{edit}, "author"); err == nil || err.Error() != "REVISION_CONFLICT" {
		t.Fatal("stale input version overwrote draft", err)
	}
	malformed := edit
	malformed.InputVersion = 1
	malformed.CSV += "too,many,columns,1,2,3,4,5,6,7,8,9\n"
	if err = s.saveModelingInputs(ctx, get(), []engineering.ModelingInputEdit{malformed}, "author"); err == nil {
		t.Fatal("malformed source rows silently discarded")
	}
	edit.InputVersion = 1
	edit.CSV = strings.Replace(partial.CSV, ",,eth2", ","+macB+",eth2", 1)
	if err = s.saveModelingInputs(ctx, get(), []engineering.ModelingInputEdit{edit}, "author"); err != nil {
		t.Fatal(err)
	}
	if err = s.adoptModelingInputs(ctx, get(), engineering.ModelingAdoption{ExpectedRevision: 1, Step: 1, Inputs: []engineering.ModelingInputVersion{{TableID: "terminations", Version: 2}}, Reason: "Explicit endpoints confirmed"}, "author"); err != nil {
		t.Fatal(err)
	}
	// The new editor's multi-table adoption must use the existing domain merge.
	placement := []engineering.InputPreviewRequest{csvInput("rooms", []string{"S1", "Site", "ROOM1", "Room", "axis", "m"}), csvInput("racks", []string{"ROOM1", "R1", "Rack", "42", "800", "1200", "2200", "", "1", "2", "0", "m", "90"}), csvInput("equipment_specs", []string{"00001", "server", "Synthetic server", "2"}, []string{"00002", "switch", "Synthetic switch", "1"}), csvInput("placements", []string{"00001", "ROOM1", "R1", "u", "3", "2", "front", ""}, []string{"00002", "ROOM1", "R1", "u", "10", "1", "front", ""})}
	edits := []engineering.ModelingInputEdit{}
	versions := []engineering.ModelingInputVersion{}
	for _, v := range placement {
		edits = append(edits, engineering.ModelingInputEdit{ExpectedRevision: 2, InputPreviewRequest: v, Reason: "Synthetic placement"})
		versions = append(versions, engineering.ModelingInputVersion{TableID: v.TableID, Version: 1})
	}
	if err = s.saveModelingInputs(ctx, get(), edits, "author"); err != nil {
		t.Fatal(err)
	}
	if err = s.adoptModelingInputs(ctx, get(), engineering.ModelingAdoption{ExpectedRevision: 2, Step: 2, Inputs: versions, Reason: "Placement confirmed"}, "author"); err != nil {
		t.Fatal(err)
	}
	port := csvInput("port_specs", []string{"00001", "eth1", "fiber", "LC", "100", "Gbps", "", ""}, []string{"00002", "eth2", "fiber", "LC", "100", "Gbps", "", ""})
	if err = s.saveModelingInputs(ctx, get(), []engineering.ModelingInputEdit{{ExpectedRevision: 3, InputPreviewRequest: port, Reason: "Port reference"}}, "author"); err != nil {
		t.Fatal(err)
	}
	if err = s.adoptModelingInputs(ctx, get(), engineering.ModelingAdoption{ExpectedRevision: 3, Step: 3, Inputs: []engineering.ModelingInputVersion{{TableID: "port_specs", Version: 1}}, Reason: "Ports confirmed"}, "author"); err != nil {
		t.Fatal(err)
	}
	w := get()
	m, _, err := engineeringModel(ctx, db, *w.HeadRevisionID, s.project)
	if err != nil || len(m.Equipment) != 2 || len(m.Placements) != 2 || len(m.Connections) != 1 {
		t.Fatalf("merge lost upstream data: %v", err)
	}
	packageResponse := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/?step=2", nil)
	request = request.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: identity.Subject{Username: "synthetic-author"}}))
	s.modelingRoutes(packageResponse, request, w, "modeling-package")
	_, basis, err := engineering.ReadModelingPackage(bytes.NewReader(packageResponse.Body.Bytes()), 2)
	if err != nil || basis.Revision != 4 || basis.SourceID != source {
		t.Fatalf("prefilled package missing revision: %v", err)
	}
	// No handoff is possible merely because a model exists.
	before, err := s.kickoffHandoff(ctx, db, w)
	if err != nil || len(before.Blockers) == 0 {
		t.Fatal("unpublished model was ready", err)
	}
	publishEngineeringTestTargets(t, s, source, w.Revision, []string{engineering.EquipmentTarget, engineering.FacilityTarget, engineering.PlacementTarget})
	publishEngineeringTestTargets(t, s, source, w.Revision, []string{engineering.ConnectivityTarget})
	w = get()
	ready, err := s.kickoffHandoff(ctx, db, w)
	if err != nil || len(ready.Blockers) > 0 {
		t.Fatalf("published physical handoff blocked: %+v %v", ready.Blockers, err)
	}
	body := jsonBytes(engineering.KickoffHandoffRequest{ExpectedRevision: w.Revision, Reason: "Synthetic handoff"})
	request = httptest.NewRequest("POST", "/", bytes.NewReader(body))
	request = request.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: identity.Subject{Username: "synthetic-reviewer"}}))
	response := httptest.NewRecorder()
	s.kickoffHandoffRoute(response, request, w)
	if response.Code != 200 {
		t.Fatalf("handoff failed: %d %s", response.Code, response.Body.String())
	}
	got, err := s.kickoffHandoff(ctx, db, w)
	if err != nil || got.State != "HANDED_OFF" || len(got.EquipmentIDs) != 2 || got.NextStageState != "NOT_STARTED" {
		t.Fatalf("handoff manufactured downstream pass: %+v %v", got, err)
	}
	// New manual input invalidates readiness while retaining the old receipt.
	edit.ExpectedRevision = 4
	edit.InputVersion = 2
	if err = s.saveModelingInputs(ctx, w, []engineering.ModelingInputEdit{edit}, "author"); err != nil {
		t.Fatal(err)
	}
	got, err = s.kickoffHandoff(ctx, db, w)
	if err != nil || got.State != "STALE" || got.ID == "" {
		t.Fatal("stale handoff lost prior receipt", err)
	}
}
