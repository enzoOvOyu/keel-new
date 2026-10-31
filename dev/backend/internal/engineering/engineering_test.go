package engineering

import (
	"reflect"
	"testing"
)

func ptr[T any](v T) *T { return &v }
func version(id string) EntityVersion {
	return EntityVersion{EntityID: id, RevisionID: "rev", ProvenanceID: "source-run", EvidenceIDs: []ID{"cell-evidence"}, Maturity: Estimated, Attributes: []AttributeValue{}}
}
func physical() PhysicalModel {
	return PhysicalModel{
		ProjectID:  "synthetic-project",
		Equipment:  []EquipmentPlan{{EntityVersion: version("device-a"), EquipmentType: "customer-appliance", SourceModelLabel: "CustomerModel", Quantity: 1, IdentityKind: "planned_instance", Label: "label-a"}, {EntityVersion: version("device-b"), EquipmentType: "switch", Quantity: 1, IdentityKind: "planned_instance", Label: "label-b"}},
		Assemblies: []DeviceAssembly{}, Facilities: []FacilityModel{{EntityVersion: version("site"), Kind: "site", Label: "site"}, {EntityVersion: version("room"), Kind: "room", Label: "room", ParentID: ptr("site")}, {EntityVersion: version("rack"), Kind: "rack", Label: "rack", ParentID: ptr("room"), CapacityU: ptr(32)}},
		Placements: []RackPlacement{{EntityVersion: version("position-a"), EquipmentID: "device-a", RackID: "rack", StartU: ptr(1), HeightU: ptr(2), MountFace: "front"}}}
}
func hasIssue(issues []ValidationIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
func TestUnknownPlacementOnlyBlocksPlacement(t *testing.T) {
	m := physical()
	m.Placements[0].HeightU = nil
	issues := ValidatePhysical(m)
	if !hasIssue(issues, "PLACEMENT_U_REQUIRED") {
		t.Fatal(issues)
	}
	for _, i := range issues {
		if i.PublishTarget != "CMDB_PLANNED_PLACEMENT" {
			t.Fatalf("unrelated equipment/facility blocked: %+v", i)
		}
	}
	if m.Placements[0].HeightU != nil {
		t.Fatal("invented device height")
	}
}
func TestPlanningGroupCannotBePlacedAsAnInstance(t *testing.T) {
	m := physical()
	m.Equipment[0].IdentityKind = "group"
	m.Equipment[0].Quantity = 9
	if !hasIssue(ValidatePhysical(m), "PLACEMENT_EQUIPMENT_UNRESOLVED") {
		t.Fatal("quantity group treated as physical instance")
	}
	m.Equipment[0].Serial = ptr("synthetic-serial")
	if !hasIssue(ValidatePhysical(m), "GROUP_SERIAL_INVALID") {
		t.Fatal("group allowed a shared serial")
	}
}
func TestAssemblyCycleAndMultiplePhysicalParents(t *testing.T) {
	m := physical()
	m.Assemblies = []DeviceAssembly{{EntityVersion: version("ab"), ParentID: "device-a", ChildID: "device-b", RelationType: "contains", Quantity: 1}, {EntityVersion: version("ba"), ParentID: "device-b", ChildID: "device-a", RelationType: "contains", Quantity: 1}}
	if !hasIssue(ValidatePhysical(m), "ASSEMBLY_CYCLE") {
		t.Fatal("cycle accepted")
	}
	m.Equipment = append(m.Equipment, EquipmentPlan{EntityVersion: version("device-c"), EquipmentType: "modular-chassis", Label: "c", Quantity: 1, IdentityKind: "planned_instance"})
	m.Assemblies[1].ParentID = "device-c"
	m.Assemblies[1].ChildID = "device-b"
	if !hasIssue(ValidatePhysical(m), "MULTIPLE_PHYSICAL_PARENTS") {
		t.Fatal("two parents accepted")
	}
}
func TestRackBoundsAndFrontRearDepth(t *testing.T) {
	m := physical()
	m.Placements[0].StartU = ptr(32)
	if !hasIssue(ValidatePhysical(m), "PLACEMENT_OUT_OF_BOUNDS") {
		t.Fatal("overflow accepted")
	}
	m.Placements[0].StartU = ptr(1)
	m.Placements = append(m.Placements, RackPlacement{EntityVersion: version("position-b"), EquipmentID: "device-b", RackID: "rack", StartU: ptr(1), HeightU: ptr(2), MountFace: "rear"})
	if !hasIssue(ValidatePhysical(m), "PLACEMENT_DEPTH_UNRESOLVED") {
		t.Fatal("opposite faces assumed compatible without depth")
	}
	m.Facilities[2].DepthMM = ptr("1000")
	m.Placements[0].DepthMM = ptr("500.25")
	m.Placements[1].DepthMM = ptr("499.75")
	if issues := ValidatePhysical(m); len(issues) != 0 {
		t.Fatal(issues)
	}
	m.Placements[1].DepthMM = ptr("499.76")
	if !hasIssue(ValidatePhysical(m), "PLACEMENT_DEPTH_UNRESOLVED") {
		t.Fatal("depth overflow accepted")
	}
}
func TestReviewAndProjectionAreSeparateGates(t *testing.T) {
	e := TransitionEvidence{ActorType: "agent", RevisionID: "r1", ReviewedRevisionID: "r1", Digest: "hash1", ReviewedDigest: "hash1", InputsCurrent: true}
	if Transition(InReview, Approved, e) == nil {
		t.Fatal("AI approved")
	}
	e.ActorType = "human"
	e.ReviewedDigest = "old"
	if Transition(InReview, Approved, e) == nil {
		t.Fatal("obsolete review accepted")
	}
	e.ReviewedDigest = "hash1"
	if err := Transition(InReview, Approved, e); err != nil {
		t.Fatal(err)
	}
	if Transition(Approved, Published, e) == nil {
		t.Fatal("pending outbox treated as published")
	}
	e.ReceiptID = "consumer-receipt"
	if err := Transition(Approved, Published, e); err != nil {
		t.Fatal(err)
	}
	e.InputsCurrent = false
	if Transition(Approved, Published, e) == nil {
		t.Fatal("stale input published")
	}
	if Transition(Detected, NotApplicable, TransitionEvidence{}) == nil {
		t.Fatal("missing data marked not applicable")
	}
}
func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r, e := NewRegistry([]EngineeringModule{
		{ID: "a", Version: 1, Domain: EquipmentDomain, Applicability: "evidence", Inputs: []ModuleIO{{"source", "SemanticFact", true}}, Outputs: []ModuleIO{{"equipment", "EquipmentPlan", false}}},
		{ID: "b", Version: 1, Domain: FacilityDomain, Applicability: "evidence", Inputs: []ModuleIO{{"equipment", "EquipmentPlan", true}}, Outputs: []ModuleIO{{"placement", "RackPlacement", false}}},
		{ID: "c", Version: 1, Domain: PowerDomain, Applicability: "evidence", Inputs: []ModuleIO{{"source", "SemanticFact", true}}, Outputs: []ModuleIO{{"power", "PowerPlan", false}}},
	})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestIndependentPlanBranchesAndMissingInput(t *testing.T) {
	r := testRegistry(t)
	p := ImportPlan{Modules: []ModuleInstance{
		{ID: "equipment", DefinitionID: "a", DefinitionVersion: 1, State: Detected, Inputs: []InputBinding{{Name: "source", Required: true, Available: true, RevisionID: ptr("s1"), Digest: ptr("h1")}}},
		{ID: "placement", DefinitionID: "b", DefinitionVersion: 1, State: Detected, Inputs: []InputBinding{{Name: "equipment", ProducerInstanceID: ptr("equipment"), Output: "equipment", Required: true}}},
		{ID: "power", DefinitionID: "c", DefinitionVersion: 1, State: NeedsInput}}}
	batches, blocked, e := r.PlanBatches(p)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(batches, [][]ID{{"equipment"}, {"placement"}}) || len(blocked["power"]) == 0 {
		t.Fatalf("%v / %v", batches, blocked)
	}
	p.Modules[0].Inputs = nil
	batches, blocked, e = r.PlanBatches(p)
	if e != nil || len(batches) != 0 || len(blocked["placement"]) == 0 {
		t.Fatalf("missing input did not propagate: %v %v %v", batches, blocked, e)
	}
	p.Modules[0].Inputs = []InputBinding{{Name: "source", Required: false, Available: true}}
	if _, _, e = r.PlanBatches(p); e == nil {
		t.Fatal("AI downgraded required input")
	}
}
func TestPlanRejectsCyclesAndTypeMismatch(t *testing.T) {
	r, e := NewRegistry([]EngineeringModule{{ID: "loop", Version: 1, Domain: EquipmentDomain, Applicability: "evidence", Inputs: []ModuleIO{{"in", "EquipmentPlan", true}}, Outputs: []ModuleIO{{"out", "EquipmentPlan", false}}}})
	if e != nil {
		t.Fatal(e)
	}
	p := ImportPlan{Modules: []ModuleInstance{{ID: "x", DefinitionID: "loop", DefinitionVersion: 1, State: NeedsInput, Inputs: []InputBinding{{Name: "in", ProducerInstanceID: ptr("y"), Output: "out", Required: true}}}, {ID: "y", DefinitionID: "loop", DefinitionVersion: 1, State: Detected, Inputs: []InputBinding{{Name: "in", ProducerInstanceID: ptr("x"), Output: "out", Required: true}}}}}
	if _, _, e = r.PlanBatches(p); e == nil {
		t.Fatal("cycle accepted")
	}
	p.Modules[1].Inputs = nil
	p.Modules[0].Inputs[0].Output = "missing"
	if _, _, e = r.PlanBatches(p); e == nil {
		t.Fatal("unknown output accepted")
	}
}
func TestInvalidateOnlyConsumersAndRegistryIsImmutable(t *testing.T) {
	p := ImportPlan{Modules: []ModuleInstance{{ID: "a"}, {ID: "b", Inputs: []InputBinding{{ProducerInstanceID: ptr("a")}}}, {ID: "c"}, {ID: "d", Inputs: []InputBinding{{ProducerInstanceID: ptr("b")}}}}}
	if got := Invalidate(p, []ID{"a"}); !reflect.DeepEqual(got, []ID{"a", "b", "d"}) {
		t.Fatal(got)
	}
	r := DefaultRegistry()
	d, _ := r.Definition("equipment")
	d.PublishTargets[0] = "OBSERVED_OVERWRITE"
	fresh, _ := r.Definition("equipment")
	if fresh.PublishTargets[0] == "OBSERVED_OVERWRITE" {
		t.Fatal("caller mutated registry")
	}
}
func TestTypedValuesPreserveFalseAndRejectMixedSlots(t *testing.T) {
	if e := (TypedValue{Kind: "boolean", Boolean: ptr(false)}).Validate(); e != nil {
		t.Fatal(e)
	}
	for _, v := range []TypedValue{{Kind: "boolean", Boolean: ptr(false), Text: ptr("false")}, {Kind: "decimal", Decimal: ptr("NaN"), Unit: ptr("W")}, {Kind: "decimal", Decimal: ptr("1.5")}, {Kind: "text", Text: ptr("1"), Unit: ptr("U")}} {
		if v.Validate() == nil {
			t.Fatalf("bad value accepted: %+v", v)
		}
	}
	if e := (TypedValue{Kind: "decimal", Decimal: ptr("0.00"), Unit: ptr("W")}).Validate(); e != nil {
		t.Fatal(e)
	}
}
