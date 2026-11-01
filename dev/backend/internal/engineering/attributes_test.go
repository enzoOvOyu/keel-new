package engineering

import (
	"github.com/google/uuid"
	"keel.local/dev/internal/bom"
	"testing"
)

func TestApprovedTypedAttributeExtraction(t *testing.T) {
	id := uuid.NewString()
	unit := "GiB"
	d := AttributeDefinition{ID: id, Version: 1, EntityKind: "equipment", Key: "GPU memory", ValueType: "decimal", Unit: &unit, EnumValues: []string{}, ApprovalStatus: "approved"}
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "arbitrary inventory", Cells: []bom.Cell{{Sheet: "arbitrary inventory", Ref: "A1", Row: 1, Column: 1, Value: "Synthetic device"}, {Sheet: "arbitrary inventory", Ref: "B1", Row: 1, Column: 2, Value: "80"}}}}}
	p := EngineeringProposal{Summary: "typed extra specification", Regions: []RegionMapping{{Sheet: "arbitrary inventory", Kind: "equipment", Bounds: Bounds{StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 2}, Columns: []ColumnBinding{{Field: "label", Column: 1}, {Field: "attribute:" + id + "@1", Column: 2, Unit: "GiB"}}}}}
	x, e := ExtractPhysical(w, p, "project", "source", "revision", d)
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Model.Equipment) != 1 || len(x.Model.Equipment[0].Attributes) != 1 || *x.Model.Equipment[0].Attributes[0].Value.Decimal != "80" {
		t.Fatal("source extension not extracted", x)
	}
	if x.MappedCells != 2 {
		t.Fatal("attribute source absent from coverage")
	}
	d.ApprovalStatus = "draft"
	x, e = ExtractPhysical(w, p, "project", "source", "revision", d)
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Model.Equipment[0].Attributes) != 0 || !hasIssue(x.Issues, "ATTRIBUTE_DEFINITION_UNAVAILABLE") {
		t.Fatal("unapproved schema used")
	}
	approved := AttributeDefinition{ID: id, Version: 1, EntityKind: "equipment", Key: "feature", ValueType: "boolean", ApprovalStatus: "approved"}
	no := false
	if e = ValidateAttribute(AttributeValue{DefinitionID: id, DefinitionVersion: 1, Value: TypedValue{Kind: "boolean", Boolean: &no}}, approved, "equipment"); e != nil {
		t.Fatal("false treated as unknown", e)
	}
	if e = ValidateAttribute(AttributeValue{DefinitionID: id, DefinitionVersion: 1, Value: TypedValue{Kind: "boolean", Boolean: &no}}, approved, "port"); e == nil {
		t.Fatal("attribute used on wrong kind")
	}
}
