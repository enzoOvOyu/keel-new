package engineering

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/xuri/excelize/v2"
	"io"
	"keel.local/dev/internal/bom"
	"os"
	"testing"
)

func TestFormulaGraphRangesNamesCyclesAndDynamic(t *testing.T) {
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "Room A", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Formula: "B1+1"}, {Ref: "B1", Row: 1, Column: 2, Formula: "A1+1"}, {Ref: "C1", Row: 1, Column: 3, Formula: `SUM('Other name'!$A$1:$A$3)+NamedInput+LEN("A99")`}, {Ref: "D1", Row: 1, Column: 4, Formula: `INDIRECT("A1")`}}}, {Name: "Other name"}}}
	graph := FormulaGraph(w, []excelize.DefinedName{{Name: "NamedInput", RefersTo: "'Other name'!$B$4", Scope: "Workbook"}})
	if len(graph) != 4 || graph[0].DependencyStatus != "cycle" || graph[1].DependencyStatus != "cycle" || graph[3].DependencyStatus != "partial" {
		t.Fatalf("wrong graph: %+v", graph)
	}
	if graph[2].DependencyStatus != "complete" || len(graph[2].Dependencies) != 2 || graph[2].Dependencies[0].ToSheetID != "Other name" {
		t.Fatalf("range/name or quoted text mishandled: %+v", graph[2])
	}
}
func numberValue(value, unit string) TypedValue {
	return TypedValue{Kind: "decimal", Decimal: &value, Unit: &unit}
}
func TestRuleUnitsCasesAndApproval(t *testing.T) {
	rule := CalculationRule{ID: "synthetic-rule", Version: 1, Domain: ConnectivityDomain, ScopeType: "project", ScopeID: "p", Inputs: []RuleIO{{Name: "distance", ValueType: "decimal", Unit: "m", Required: true}}, Outputs: []RuleIO{{Name: "length", ValueType: "decimal", Unit: "m", Required: true}}, Expression: RuleExpression{Operator: "select_ceiling", Arguments: []RuleExpression{{Operator: "input", Input: ptr("distance")}, {Operator: "literal", Literal: &Quantity{"3", "m"}}, {Operator: "literal", Literal: &Quantity{"5", "m"}}}}, ValidationCases: []RuleCase{{ID: "hand-authored", Inputs: []NamedValue{{Name: "distance", Value: numberValue("350", "cm")}}, Expected: []NamedValue{{Name: "length", Value: numberValue("5", "m")}}, Tolerance: "0"}}}
	if e := ValidateRule(rule); e != nil {
		t.Fatal(e)
	}
	if _, e := RunRule(rule, "p", "rev", rule.ValidationCases[0].Inputs); e == nil {
		t.Fatal("unapproved rule executed")
	}
	rule.ApprovalStatus = "approved"
	run, e := RunRule(rule, "p", "rev", rule.ValidationCases[0].Inputs)
	if e != nil || *run.Outputs[0].Value.Decimal != "5" {
		t.Fatalf("wrong standard size: %+v %v", run, e)
	}
	if _, e = RunRule(rule, "other-project", "rev", rule.ValidationCases[0].Inputs); e == nil {
		t.Fatal("cross-project rule accepted")
	}
	rule.ValidationCases[0].Inputs[0].Value = numberValue("350", "W")
	if ValidateRule(rule) == nil {
		t.Fatal("incompatible units accepted")
	}
	rule.ValidationCases[0].Inputs[0].Value = numberValue("6", "m")
	if ValidateRule(rule) == nil {
		t.Fatal("unsupported standard size fabricated")
	}
	rule.Expression = RuleExpression{Operator: "javascript"}
	if ValidateRule(rule) == nil {
		t.Fatal("arbitrary operator accepted")
	}
}

func TestIsolatedCalcReference(t *testing.T) {
	image := os.Getenv("KEEL_CALC_TEST_IMAGE")
	if image == "" {
		t.Skip("requires explicitly configured isolated calculation image")
	}
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Inputs renamed")
	f.SetCellValue("Inputs renamed", "A1", 2)
	f.SetCellValue("Inputs renamed", "A2", 3)
	f.NewSheet("Results")
	f.SetCellFormula("Results", "B1", "SUM('Inputs renamed'!A1:A2)*4")
	f.SetCellFormula("Results", "B2", "1/0")
	buf, e := f.WriteToBuffer()
	if e != nil {
		t.Fatal(e)
	}
	data := buf.Bytes()
	hash := sha256.Sum256(data)
	provider := LibreOfficeProvider{Image: image}
	run, e := provider.Calculate(context.Background(), SpreadsheetCalculationRequest{Source: WorkbookSource{ProjectID: "synthetic", SHA256: hex.EncodeToString(hash[:])}, Revision: ModelRevision{ID: "synthetic-revision"}, OpenSource: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }})
	if e != nil {
		t.Fatalf("provider: %v %+v", e, run.Errors)
	}
	if run.Status != "partial" || len(run.Formulas) != 2 || len(run.Errors) != 1 {
		t.Fatalf("formula error not explicit: %+v", run)
	}
	got := run.Formulas[0]
	if got.RecalculatedValue == nil || got.RecalculatedValue.Decimal == nil || *got.RecalculatedValue.Decimal != "20" {
		t.Fatalf("reference 20 mismatch: %+v", got)
	}
	if got.CachedValue != nil && got.CachedValue.Decimal != nil && *got.CachedValue.Decimal == "20" {
		t.Fatal("original cache mutated")
	}
	if sha256.Sum256(data) != hash {
		t.Fatal("source mutated")
	}
}
