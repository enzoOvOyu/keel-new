package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/cmdb"
	"keel.local/dev/internal/engineering"
	"os"
	"strings"
	"testing"
)

func TestEngineeringTypedAttributesApprovalAndPublication(t *testing.T) {
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
	s := &Server{db: db, project: "engineering-attributes-" + uuid.NewString()}
	if e = (cmdb.Service{DB: db}).Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic attributes',$1)`, s.project); e != nil {
		t.Fatal(e)
	}
	d, e := s.saveAttributeDefinition(ctx, engineering.AttributeDraft{Definition: engineering.AttributeDefinition{EntityKind: "equipment", Key: "Supports synthetic feature", ValueType: "boolean", EnumValues: []string{}, Required: true}, Reason: "Explicit project requirement"}, "synthetic-author")
	if e != nil {
		t.Fatal(e)
	}
	original := uuid.NewString()
	if _, e = db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,workbook) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic-sha','MAPPING',$5)`, original, s.project, uuid.NewString(), s.project+"/synthetic.xlsx", jsonBytes(bom.Workbook{Sheets: []bom.Sheet{}})); e != nil {
		t.Fatal(e)
	}
	source, e := s.attachEngineering(ctx, original, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	no := false
	m := engineering.EmptyPhysical(s.project)
	m.Equipment = []engineering.EquipmentPlan{{EntityVersion: engineering.EntityVersion{EntityID: uuid.NewString(), Maturity: engineering.Estimated, Attributes: []engineering.AttributeValue{{DefinitionID: d.ID, DefinitionVersion: 1, Value: engineering.TypedValue{Kind: "boolean", Boolean: &no}}}}, Label: "Synthetic device", EquipmentType: "switch", Quantity: 1, IdentityKind: "planned_instance"}}
	p := engineering.EngineeringProposal{Summary: "Synthetic extension values", Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	if _, e = s.saveEngineeringDraft(ctx, source, 1, "human", "synthetic-human", "Apply definition", engineering.Extraction{Model: m}, p); e == nil || e.Error() != "ATTRIBUTE_DEFINITION_NOT_APPROVED" {
		t.Fatal("unapproved definition applied", e)
	}
	if _, e = s.reviewAttributeDefinition(ctx, d.ID, engineering.AttributeReview{Version: 1, Decision: "approve", Reason: "Reviewed explicit project schema"}, "synthetic-reviewer"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.saveEngineeringDraft(ctx, source, 1, "human", "synthetic-human", "Confirmed false is a known value", engineering.Extraction{Model: m}, p); e != nil {
		t.Fatal(e)
	}
	w, e := s.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	loaded, _, e := engineeringModel(ctx, db, *w.HeadRevisionID, s.project)
	if e != nil {
		t.Fatal(e)
	}
	value := loaded.Equipment[0].Attributes
	if len(value) != 1 || value[0].Value.Boolean == nil || *value[0].Value.Boolean {
		t.Fatal("typed false lost", value)
	}
	publishEngineeringTestTargets(t, s, source, 2, []string{engineering.EquipmentTarget})
	var n int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_field_source WHERE asset_id=$1 AND field_name=$2 AND value->>'kind'='boolean' AND (value->>'boolean')::boolean=false`, m.Equipment[0].EntityID, "engineering.attribute."+d.ID+"@1").Scan(&n); e != nil || n != 1 {
		t.Fatal("typed planning attribute not linked to CMDB", e, n)
	}
	missing := loaded
	missing.Equipment[0].Attributes = []engineering.AttributeValue{}
	if _, e = s.saveEngineeringDraft(ctx, source, 2, "human", "synthetic-human", "Explicit removal must require review", engineering.Extraction{Model: missing}, p); e != nil {
		t.Fatal(e)
	}
	w, e = s.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	required := false
	for _, issue := range w.Issues {
		required = required || issue.Code == "ATTRIBUTE_REQUIRED"
	}
	if !required {
		t.Fatal("required project field missing without issue")
	}
}
