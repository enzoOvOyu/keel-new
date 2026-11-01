package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/cmdb"
	"keel.local/dev/internal/engineering"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestEngineeringReimportReviewedRemoval(t *testing.T) {
	dsn := os.Getenv("KEEL_ENGINEERING_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "/keel_engineering_") {
		t.Skip("requires isolated engineering database")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	prepareEngineeringDB(t, db)
	s := &Server{db: db, project: "engineering-reimport-" + uuid.NewString()}
	if e = (cmdb.Service{DB: db}).Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic reimport',$1)`, s.project); e != nil {
		t.Fatal(e)
	}
	original := func() string {
		id := uuid.NewString()
		if _, err := db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,workbook) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic-sha','MAPPING',$5)`, id, s.project, uuid.NewString(), s.project+"/"+id, jsonBytes(bom.Workbook{Sheets: []bom.Sheet{}})); err != nil {
			t.Fatal(err)
		}
		return id
	}
	source, e := s.attachEngineering(ctx, original(), "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	a, z, pa, pz, cable := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	version := func(id string) engineering.EntityVersion {
		return engineering.EntityVersion{EntityID: id, Maturity: engineering.Estimated}
	}
	m := engineering.EmptyPhysical(s.project)
	m.Equipment = []engineering.EquipmentPlan{{EntityVersion: version(a), EquipmentType: "switch", SourceModelLabel: "Confirmed-A", Label: "Synthetic-A", Quantity: 1, IdentityKind: "planned_instance"}, {EntityVersion: version(z), EquipmentType: "switch", SourceModelLabel: "Confirmed-Z", Label: "Synthetic-Z", Quantity: 1, IdentityKind: "planned_instance"}}
	m.Ports = []engineering.PlannedPort{{EntityVersion: version(pa), EquipmentID: a, Name: "port-a"}, {EntityVersion: version(pz), EquipmentID: z, Name: "port-z"}}
	m.Connections = []engineering.PlannedConnection{{EntityVersion: version(cable), APortID: pa, ZPortID: pz, PatchPortIDs: []string{}}}
	p := engineering.EngineeringProposal{Summary: "Synthetic original", Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	if _, e = s.saveEngineeringDraft(ctx, source, 1, "human", "synthetic-human", "Confirmed source", engineering.Extraction{Model: m}, p); e != nil {
		t.Fatal(e)
	}
	publishEngineeringTestTargets(t, s, source, 2, []string{engineering.EquipmentTarget, engineering.ConnectivityTarget})
	var observed []byte
	if e = db.QueryRow(ctx, `SELECT payload FROM cmdb_entity WHERE item_uid=$1`, a).Scan(&observed); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, `INSERT INTO cmdb_field_source(id,asset_id,field_name,value,source_type,source_ref,fact_layer,revision,operator,approval_status,support,freshness,evidence_refs,project_id) VALUES($1,$2,'hostname','"Synthetic-A"','collection','synthetic-observed','observed','1','synthetic','approved','supported','fresh','[]',$3)`, uuid.NewString(), a, s.project); e != nil {
		t.Fatal(e)
	}
	w, e := s.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	next, e := s.reimportEngineering(ctx, w, engineering.ReimportRequest{ExpectedRevision: 2, ImportID: original()}, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	m, sourceEvidence, e := engineeringModel(ctx, db, *w.HeadRevisionID, s.project)
	if e != nil {
		t.Fatal(e)
	}
	for i := range m.Equipment {
		if m.Equipment[i].EntityID == a {
			m.Equipment[i].SourceModelLabel = "AI replacement suggestion"
		}
	}
	m.Connections = []engineering.PlannedConnection{}
	if _, e = s.saveEngineeringDraft(ctx, next, 1, "agent", "synthetic-agent", "New proposed source", engineering.Extraction{Model: m, Evidence: sourceEvidence}, p); e != nil {
		t.Fatal(e)
	}
	nw, e := s.engineeringWorkspace(ctx, next)
	if e != nil {
		t.Fatal(e)
	}
	if nw.BaseSourceID == nil || *nw.BaseSourceID != source {
		t.Fatal("missing immutable source lineage")
	}
	blocked := map[string]bool{}
	for _, i := range nw.Issues {
		if i.Code == "REIMPORT_DIFFERENCE_REVIEW_REQUIRED" {
			blocked[i.PublishTarget] = true
		}
	}
	if !blocked[engineering.EquipmentTarget] || !blocked[engineering.ConnectivityTarget] {
		t.Fatal("changed/removal proposals not held for difference review", nw.Issues)
	}
	if e = s.reviewEngineeringDifferences(ctx, next, engineering.DifferenceReview{ExpectedRevision: 2, Reason: "Retain confirmed equipment; accept cable removal", Resolutions: []engineering.DifferenceResolution{{Target: engineering.EquipmentTarget, Decision: "retain_base"}, {Target: engineering.ConnectivityTarget, Decision: "accept_incoming"}}}, "synthetic-reviewer"); e != nil {
		t.Fatal(e)
	}
	nw, e = s.engineeringWorkspace(ctx, next)
	if e != nil {
		t.Fatal(e)
	}
	for _, i := range nw.Issues {
		if i.Severity == "BLOCK" {
			t.Fatal("reviewed differences still blocked", i)
		}
	}
	current, ev, e := engineeringModel(ctx, db, *nw.HeadRevisionID, s.project)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, v := range current.Equipment {
		if v.EntityID == a && v.SourceModelLabel == "Confirmed-A" {
			found = true
		}
	}
	if !found {
		t.Fatal("human confirmed base overwritten")
	}
	hasOriginalEvidence := false
	for _, v := range ev {
		if v.EntityID == a && v.SourceID == source {
			hasOriginalEvidence = true
		}
	}
	if !hasOriginalEvidence {
		t.Fatal("retained value lost original workbook provenance")
	}
	publishEngineeringTestTargets(t, s, next, 3, []string{engineering.EquipmentTarget, engineering.ConnectivityTarget})
	var lifecycle string
	if e = db.QueryRow(ctx, `SELECT lifecycle_state FROM cmdb_entity WHERE item_uid=$1`, cable).Scan(&lifecycle); e != nil || lifecycle != "retired" {
		t.Fatal("withdrawn cable remains active", e, lifecycle)
	}
	var count int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_relationship WHERE from_uid=$1 AND definition_id IN('cable_endpoint_a','cable_endpoint_b') AND valid_to IS NULL`, cable).Scan(&count); e != nil || count != 0 {
		t.Fatal("obsolete topology endpoints remain", e, count)
	}
	var after []byte
	if e = db.QueryRow(ctx, `SELECT payload FROM cmdb_entity WHERE item_uid=$1`, a).Scan(&after); e != nil || string(after) != string(observed) {
		t.Fatal("observed payload changed", e)
	}
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_field_source WHERE asset_id=$1 AND superseded_at IS NOT NULL`, cable).Scan(&count); e != nil || count == 0 {
		t.Fatal("history evidence was not retained", e)
	}
	var state string
	if e = db.QueryRow(ctx, `SELECT state FROM eng_target WHERE source_id=$1 AND target=$2`, source, engineering.ConnectivityTarget).Scan(&state); e != nil || state != "STALE" {
		t.Fatal("old physical publication still consumable", e, state)
	}
	visual, e := (cmdb.Service{DB: db}).VisualConnections(ctx, cmdb.Scope{Projects: []string{s.project}}, url.Values{"limit": {"200"}})
	if e != nil || len(visual.Cables) != 0 {
		t.Fatal("withdrawn cable remains in topology", e, len(visual.Cables))
	}
	current.Ports = []engineering.PlannedPort{}
	if _, e = s.saveEngineeringDraft(ctx, next, 3, "human", "synthetic-human", "Remove unused planning ports", engineering.Extraction{Model: current, Evidence: ev}, p); e != nil {
		t.Fatal(e)
	}
	if e = s.reviewEngineeringDifferences(ctx, next, engineering.DifferenceReview{ExpectedRevision: 4, Reason: "Accept removal of remaining unused ports", Resolutions: []engineering.DifferenceResolution{{Target: engineering.ConnectivityTarget, Decision: "accept_incoming"}}}, "synthetic-reviewer"); e != nil {
		t.Fatal(e)
	}
	publishEngineeringTestTargets(t, s, next, 5, []string{engineering.ConnectivityTarget})
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_entity WHERE item_uid=ANY($1) AND lifecycle_state='retired'`, []string{pa, pz}).Scan(&count); e != nil || count != 2 {
		t.Fatal("empty reviewed target did not retire ports", e, count)
	}

}
