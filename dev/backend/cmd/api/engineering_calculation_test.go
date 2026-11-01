package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEngineeringCalculationPersistenceAndRuleReview(t *testing.T) {
	dsn := os.Getenv("KEEL_ENGINEERING_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "/keel_engineering_") {
		t.Skip("explicitly isolated engineering database required")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	migration, e := os.ReadFile("../../migrations/028_engineering_calculation.sql")
	if e != nil {
		t.Fatal(e)
	}
	var migrated bool
	if e = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migration WHERE version=28)`).Scan(&migrated); e != nil {
		t.Fatal(e)
	}
	if !migrated {
		if _, e = db.Exec(ctx, string(migration)); e != nil {
			t.Fatal(e)
		}
	}
	project := "engineering-calculation-synthetic-" + uuid.NewString()
	s := &Server{db: db, project: project}
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic calculation check',$1)`, project); e != nil {
		t.Fatal(e)
	}
	batch := uuid.NewString()
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "Arbitrary", Cells: []bom.Cell{{Sheet: "Arbitrary", Ref: "A1", Row: 1, Column: 1, Formula: "2+3", Value: "0"}}}}}
	if _, e = db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,workbook) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic-hash','MAPPING',$5)`, batch, project, uuid.NewString(), project+"/synthetic", jsonBytes(wb)); e != nil {
		t.Fatal(e)
	}
	source, e := s.attachEngineering(ctx, batch, "synthetic-user")
	if e != nil {
		t.Fatal(e)
	}
	workspace, e := s.engineeringWorkspace(ctx, source)
	if e != nil || workspace.HeadRevisionID == nil || workspace.Revision != 1 {
		t.Fatalf("source anchor failed: %+v %v", workspace, e)
	}
	value := func(n, u string) engineering.TypedValue {
		return engineering.TypedValue{Kind: "decimal", Decimal: &n, Unit: &u}
	}
	input := "load"
	rule := engineering.CalculationRule{ID: uuid.NewString(), Version: 1, Domain: engineering.PowerDomain, ScopeType: "project", ScopeID: project, Inputs: []engineering.RuleIO{{Name: input, ValueType: "decimal", Unit: "W", Required: true}}, Outputs: []engineering.RuleIO{{Name: "power", ValueType: "decimal", Unit: "kW", Required: true}}, Expression: engineering.RuleExpression{Operator: "input", Input: &input, Arguments: []engineering.RuleExpression{}}, Dependencies: []engineering.VersionReference{}, ValidationCases: []engineering.RuleCase{{ID: "manual-reference", Inputs: []engineering.NamedValue{{Name: input, Value: value("1200", "W")}}, Expected: []engineering.NamedValue{{Name: "power", Value: value("1.2", "kW")}}, Tolerance: "0"}}}
	view, e := s.saveEngineeringRule(ctx, engineering.RuleDraft{Rule: rule, Reason: "hand-authored synthetic case"}, "synthetic-author")
	if e != nil {
		t.Fatal(e)
	}
	q := engineering.CalculationRequest{ExpectedRevision: 1, Rule: &engineering.VersionReference{ID: view.Rule.ID, Version: 1}, RequestedCells: []engineering.FormulaCell{}, Inputs: rule.ValidationCases[0].Inputs}
	if _, e = s.queueEngineeringCalculation(ctx, source, q, "synthetic-user"); e == nil {
		t.Fatal("unapproved rule queued")
	}
	if _, e = s.reviewEngineeringRule(ctx, rule.ID, 1, engineering.RuleReview{ExpectedDigest: "wrong", Decision: "approve", Reason: "test"}, "synthetic-reviewer"); e == nil {
		t.Fatal("wrong digest approved")
	}
	view, e = s.reviewEngineeringRule(ctx, rule.ID, 1, engineering.RuleReview{ExpectedDigest: view.Digest, Decision: "approve", Reason: "reference verified"}, "synthetic-reviewer")
	if e != nil {
		t.Fatal(e)
	}
	runID, e := s.queueEngineeringCalculation(ctx, source, q, "synthetic-user")
	if e != nil {
		t.Fatal(e)
	}
	same, e := s.queueEngineeringCalculation(ctx, source, q, "synthetic-user")
	if e != nil || same != runID {
		t.Fatalf("duplicate active calculation: %s %v", same, e)
	}
	s.runEngineeringCalculation(ctx)
	result, e := s.engineeringCalculation(ctx, runID, 0, 50)
	if e != nil {
		t.Fatal(e)
	}
	if result.Run.Status != "succeeded" || len(result.Run.Outputs) != 1 || *result.Run.Outputs[0].Value.Decimal != "1.2" {
		t.Fatalf("wrong stored calculation: %+v", result)
	}
	if _, e = s.saveEngineeringRule(ctx, engineering.RuleDraft{ExpectedVersion: 0, Rule: rule, Reason: "stale version"}, "synthetic-author"); e == nil {
		t.Fatal("stale rule write accepted")
	}
	// Persist per-cell evidence independently of the core provider test, which
	// invokes real isolated Calc against a generated workbook.
	formulaID, e := s.queueEngineeringCalculation(ctx, source, engineering.CalculationRequest{ExpectedRevision: 1, RequestedCells: []engineering.FormulaCell{}, Inputs: []engineering.NamedValue{}}, "synthetic-user")
	if e != nil {
		t.Fatal(e)
	}
	claim := uuid.NewString()
	if _, e = db.Exec(ctx, `UPDATE eng_calculation_run SET status='running',claim_id=$2 WHERE id=$1`, formulaID, claim); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	v := value("5", "source")
	cached := value("0", "source")
	run := engineering.CalculationRun{ID: formulaID, Status: "succeeded", ProviderVersion: "synthetic-store-check", Compatibility: "compatible", FinishedAt: &now, Outputs: []engineering.NamedValue{}, Errors: []engineering.CalculationError{}, Formulas: []engineering.FormulaEvidence{{Cell: engineering.FormulaCell{SheetID: "Arbitrary", Address: "A1"}, Expression: "2+3", CachedValue: &cached, RecalculatedValue: &v, DependencyStatus: "complete", Compatibility: "compatible", Dependencies: []engineering.FormulaDependency{}}}}
	if e = s.saveEngineeringCalculation(ctx, claim, run); e != nil {
		t.Fatal(e)
	}
	page, e := s.engineeringCalculation(ctx, formulaID, 0, 50)
	if e != nil || page.TotalFormulas != 1 || *page.Run.Formulas[0].CachedValue.Decimal != "0" {
		t.Fatalf("cache/result separation failed: %+v %v", page, e)
	}
	effective, refs, e := s.engineeringCalculatedWorkbook(ctx, source, wb)
	if e != nil || effective.Sheets[0].Cells[0].Value != "5" || wb.Sheets[0].Cells[0].Value != "0" || refs["Arbitrary!A1"] != formulaID {
		t.Fatalf("source or provenance incorrect: %v", e)
	}
	other := *s
	other.project = "different-project"
	if _, e = other.engineeringCalculation(ctx, runID, 0, 50); e == nil {
		t.Fatal("cross-project calculation visible")
	}
	entity, evidenceID := uuid.NewString(), uuid.NewString()
	model := engineering.EmptyPhysical(project)
	model.Equipment = append(model.Equipment, engineering.EquipmentPlan{EntityVersion: engineering.EntityVersion{EntityID: entity, RevisionID: *workspace.HeadRevisionID, ProvenanceID: source, EvidenceIDs: []string{evidenceID}, Maturity: engineering.Estimated, Attributes: []engineering.AttributeValue{}}, EquipmentType: "server", Quantity: 5, IdentityKind: "group", Label: "Synthetic group"})
	x := engineering.Extraction{Model: model, Evidence: []engineering.SourceEvidence{{ID: evidenceID, EntityID: entity, Sheet: "Arbitrary", Cell: "A1", Field: "quantity", Formula: true, CalculationRunID: &formulaID}}, Issues: []engineering.ValidationIssue{{ID: uuid.NewString(), EntityID: entity, Code: "SOURCE_CONFLICT", Severity: "BLOCK", ModuleID: "workbook_intelligence", Field: "quantity", Message: "Synthetic unresolved source conflict", PublishTarget: engineering.EquipmentTarget}}}
	proposal := engineering.EngineeringProposal{Summary: "synthetic", Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	if _, e = s.saveEngineeringDraft(ctx, source, 1, "agent", "synthetic-agent", "source proposal", x, proposal); e != nil {
		t.Fatal(e)
	}
	x.Model.Equipment[0].Label = "Manually renamed group"
	x.Issues = nil
	if _, e = s.saveEngineeringDraft(ctx, source, 2, "human", "synthetic-user", "label only", x, proposal); e != nil {
		t.Fatal(e)
	}
	current, e := s.engineeringWorkspace(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	conflict := false
	for _, issue := range current.Issues {
		if issue.Code == "SOURCE_CONFLICT" && issue.Field == "quantity" {
			conflict = true
		}
	}
	if !conflict {
		t.Fatal("unrelated manual change cleared source conflict")
	}
	_, evidence, e := engineeringModel(ctx, db, *current.HeadRevisionID, project)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, ref := range evidence {
		if ref.CalculationRunID != nil && *ref.CalculationRunID == formulaID {
			found = true
		}
	}
	if !found {
		t.Fatal("calculation provenance lost")
	}
}
