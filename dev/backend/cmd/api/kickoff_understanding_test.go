package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestHistoricalIdentityAmbiguity(t *testing.T) {
	serial := "synthetic-sn"
	mac := "02:00:00:00:00:01"
	m := engineering.EmptyPhysical("test")
	m.Equipment = []engineering.EquipmentPlan{{EntityVersion: engineering.EntityVersion{EntityID: "d1"}, Serial: &serial}, {EntityVersion: engineering.EntityVersion{EntityID: "d2"}, Serial: &serial}}
	m.Ports = []engineering.PlannedPort{{EntityVersion: engineering.EntityVersion{EntityID: "p1"}, MAC: &mac}}
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "arbitrary", Cells: []bom.Cell{{Ref: "A1", Value: serial}, {Ref: "B1", Value: mac}, {Ref: "C1", Value: mac, Sensitive: true}}}}}
	got := restoreIdentityEvidence(w, m)
	if len(got) != 1 || got[0].Cell != "B1" || got[0].Field != "mac_candidate" {
		t.Fatal("ambiguous or sensitive identity mapped")
	}
}
func TestKickoffHistoricalUnderstandingExisting(t *testing.T) {
	if os.Getenv("KEEL_UNDERSTANDING_EXISTING") != "1" {
		t.Skip("explicit existing data opt-in")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("private DB config invalid")
	}
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("DB unavailable")
	}
	defer db.Close()
	objects, err := minio.New(os.Getenv("S3_ENDPOINT"), &minio.Options{Creds: credentials.NewStaticV4(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""), Secure: false})
	if err != nil {
		t.Fatal("storage unavailable")
	}
	s := &Server{db: db, objects: objects, bucket: os.Getenv("S3_BUCKET"), project: os.Getenv("KEEL_KICKOFF_PROJECT")}
	var source string
	var before int
	if err = db.QueryRow(ctx, `SELECT source_id::text,revision FROM kickoff_workspace WHERE project_id=$1`, s.project).Scan(&source, &before); err != nil {
		t.Fatal("workspace unavailable")
	}
	head, err := s.understandingHead(ctx, source)
	if err != nil {
		t.Fatal("source unavailable")
	}
	start := time.Now()
	report, err := s.buildUnderstanding(ctx, source, head, understandingKey(s.project, source, head))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Regions) == 0 || report.State != "partial" {
		t.Fatal("missing partial report")
	}
	for _, step := range report.Steps {
		if step.ID == "relationships" {
			t.Logf("relationships completed=%d total=%d note=%s", step.Completed, step.Total, step.Note)
		}
	}
	count := map[string]int{}
	for _, v := range report.Regions {
		count["regions"]++
		count["objects"] += v.ObjectCount
		count["cells"] += v.TotalCells
		count["locatedCells"] += v.CoveredCells
		count["fieldEvidence"] += v.FieldEvidenceCount
		w := httptest.NewRecorder()
		s.kickoffRegionPreview(w, httptest.NewRequest("GET", "/preview?limit=2&revisionId="+head, nil), source, v.ID)
		if w.Code != 200 {
			t.Fatalf("preview=%d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	s.kickoffRegionPreview(w, httptest.NewRequest("GET", "/preview?revisionId=stale", nil), source, report.Regions[0].ID)
	if w.Code != 409 {
		t.Fatal("stale revision accepted")
	}
	w = httptest.NewRecorder()
	s.kickoffUnderstanding(w, httptest.NewRequest("POST", "/reuse-history", nil), source, true)
	if w.Code != 200 {
		t.Fatal("idempotent replay failed")
	}
	var after int
	if err = db.QueryRow(ctx, `SELECT revision FROM kickoff_workspace WHERE project_id=$1`, s.project).Scan(&after); err != nil || before != after {
		t.Fatal("workspace modified")
	}
	t.Logf("elapsed=%s aggregate=%v unchanged=%t", time.Since(start).Round(time.Millisecond), count, before == after)
}

func TestHistoricalConnectionExactPair(t *testing.T) {
	a, b := "02:00:00:00:00:01", "02:00:00:00:00:02"
	m := engineering.EmptyPhysical("test")
	m.Ports = []engineering.PlannedPort{{EntityVersion: engineering.EntityVersion{EntityID: "pa"}, MAC: &a}, {EntityVersion: engineering.EntityVersion{EntityID: "pb"}, MAC: &b}}
	m.Connections = []engineering.PlannedConnection{{EntityVersion: engineering.EntityVersion{EntityID: "link"}, APortID: "pa", ZPortID: "pb"}}
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "renamed", Cells: []bom.Cell{{Ref: "A8", Value: a}, {Ref: "F8", Value: b}}}}}
	got := restoreIdentityEvidence(w, m)
	links := 0
	for _, ev := range got {
		if ev.Field == "connection_candidate" {
			links++
			if ev.EntityID != "link" {
				t.Fatal("wrong link")
			}
		}
	}
	if links != 1 {
		t.Fatal("exact pair not recovered")
	}
	w.Sheets[0].Cells[1].Ref = "F9"
	got = restoreIdentityEvidence(w, m)
	for _, ev := range got {
		if ev.Field == "connection_candidate" {
			t.Fatal("cross-row guessed link")
		}
	}
}

func TestUnderstandingCompletionUsesEvidenceScope(t *testing.T) {
	for _, v := range []struct {
		done, total int
		candidate   bool
		want        string
	}{{2, 2, false, "complete"}, {1, 2, false, "partial"}, {0, 0, false, "pending"}, {0, 0, true, "partial"}} {
		if got := understandingCompletion(v.done, v.total, v.candidate); got != v.want {
			t.Fatalf("completion=%s want=%s", got, v.want)
		}
	}
}
