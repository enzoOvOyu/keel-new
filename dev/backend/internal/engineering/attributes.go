package engineering

import (
	"fmt"
	"github.com/google/uuid"
	"strconv"
	"strings"
)

type AttributeDraft struct {
	ExpectedVersion int                 `json:"expected_version"`
	Definition      AttributeDefinition `json:"definition"`
	Reason          string              `json:"reason"`
}
type AttributeReview struct {
	Version  int    `json:"version"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}
type ModelObject struct {
	Kind, Target, Label string
	Version             *EntityVersion
}

func ModelObjects(m *PhysicalModel) []ModelObject {
	out := []ModelObject{}
	for i := range m.Equipment {
		v := &m.Equipment[i]
		out = append(out, ModelObject{"equipment", EquipmentTarget, v.Label, &v.EntityVersion})
	}
	for i := range m.Assemblies {
		v := &m.Assemblies[i]
		out = append(out, ModelObject{"assembly", EquipmentTarget, v.RelationType, &v.EntityVersion})
	}
	for i := range m.Facilities {
		v := &m.Facilities[i]
		out = append(out, ModelObject{"facility", FacilityTarget, v.Label, &v.EntityVersion})
	}
	for i := range m.Placements {
		v := &m.Placements[i]
		out = append(out, ModelObject{"placement", PlacementTarget, v.EquipmentID, &v.EntityVersion})
	}
	for i := range m.Ports {
		v := &m.Ports[i]
		out = append(out, ModelObject{"port", ConnectivityTarget, v.Name, &v.EntityVersion})
	}
	for i := range m.Connections {
		v := &m.Connections[i]
		out = append(out, ModelObject{"connection", ConnectivityTarget, v.APortID + " → " + v.ZPortID, &v.EntityVersion})
	}
	for i := range m.Cables {
		v := &m.Cables[i]
		out = append(out, ModelObject{"cable_plan", CablingTarget, v.Spec.SourceModelLabel, &v.EntityVersion})
	}
	for i := range m.Optics {
		v := &m.Optics[i]
		out = append(out, ModelObject{"optic_demand", CablingTarget, v.SourceModelLabel, &v.EntityVersion})
	}
	for i := range m.PowerPlans {
		v := &m.PowerPlans[i]
		out = append(out, ModelObject{"power_plan", PowerTarget, "电力规划", &v.EntityVersion})
	}
	for i := range m.Materials {
		v := &m.Materials[i]
		out = append(out, ModelObject{"material_estimate", MaterialTarget, v.SourceModelLabel, &v.EntityVersion})
	}
	for i := range m.Workloads {
		v := &m.Workloads[i]
		out = append(out, ModelObject{"workload_estimate", WorkloadTarget, v.WorkType, &v.EntityVersion})
	}
	for i := range m.NetworkPlans {
		v := &m.NetworkPlans[i]
		out = append(out, ModelObject{"network_plan", NetworkTarget, "网络规划", &v.EntityVersion})
	}
	return out
}
func ValidAttributeKind(kind string) bool {
	return strings.Contains(" equipment assembly facility placement port connection cable_plan optic_demand power_plan material_estimate workload_estimate network_plan ", " "+kind+" ")
}
func ValidateAttributeDefinition(d AttributeDefinition) error {
	if !ValidAttributeKind(d.EntityKind) || strings.TrimSpace(d.Key) == "" || len(d.Key) > 100 {
		return fmt.Errorf("ATTRIBUTE_DEFINITION_INVALID")
	}
	switch d.ValueType {
	case "text", "boolean", "reference":
		if d.Unit != nil {
			return fmt.Errorf("ATTRIBUTE_UNIT_INVALID")
		}
	case "decimal":
		if d.Unit == nil || strings.TrimSpace(*d.Unit) == "" {
			return fmt.Errorf("ATTRIBUTE_UNIT_REQUIRED")
		}
	default:
		return fmt.Errorf("ATTRIBUTE_TYPE_INVALID")
	}
	if d.ValueType != "text" && len(d.EnumValues) > 0 {
		return fmt.Errorf("ATTRIBUTE_ENUM_REQUIRES_TEXT")
	}
	return nil
}
func ValidateAttribute(value AttributeValue, d AttributeDefinition, kind string) error {
	if d.ID != value.DefinitionID || d.Version != value.DefinitionVersion || d.ApprovalStatus != "approved" || d.EntityKind != kind {
		return fmt.Errorf("ATTRIBUTE_DEFINITION_NOT_APPROVED")
	}
	if value.Value.Validate() != nil || value.Value.Kind != d.ValueType {
		return fmt.Errorf("ATTRIBUTE_VALUE_TYPE_INVALID")
	}
	if d.Unit != nil && (value.Value.Unit == nil || *d.Unit != *value.Value.Unit) {
		return fmt.Errorf("ATTRIBUTE_UNIT_MISMATCH")
	}
	if len(d.EnumValues) > 0 {
		match := false
		for _, v := range d.EnumValues {
			match = match || value.Value.Text != nil && *value.Value.Text == v
		}
		if !match {
			return fmt.Errorf("ATTRIBUTE_ENUM_INVALID")
		}
	}
	return nil
}
func AttributeBinding(field string) (string, int, bool) {
	if !strings.HasPrefix(field, "attribute:") {
		return "", 0, false
	}
	parts := strings.Split(strings.TrimPrefix(field, "attribute:"), "@")
	if len(parts) != 2 {
		return "", 0, false
	}
	if _, e := uuid.Parse(parts[0]); e != nil {
		return "", 0, false
	}
	version, e := strconv.Atoi(parts[1])
	return parts[0], version, e == nil && version > 0
}
func AttributeIssues(m PhysicalModel, definitions []AttributeDefinition) []ValidationIssue {
	issues := []ValidationIssue{}
	for _, object := range ModelObjects(&m) {
		add := func(code, field, message string) {
			issues = append(issues, ValidationIssue{ID: StableID(m.ProjectID, "attribute-issue", object.Version.EntityID+code+field), EntityID: object.Version.EntityID, ModuleID: "validation_release", Code: code, Field: field, Severity: "BLOCK", Message: message, PublishTarget: object.Target, EvidenceIDs: object.Version.EvidenceIDs})
		}
		for _, definition := range definitions {
			newer := false
			for _, d := range definitions {
				if d.ID == definition.ID && d.ApprovalStatus == "approved" && d.Version > definition.Version {
					newer = true
				}
			}
			if newer || definition.ApprovalStatus != "approved" || definition.EntityKind != object.Kind || !definition.Required {
				continue
			}
			found := false
			for _, v := range object.Version.Attributes {
				found = found || v.DefinitionID == definition.ID
			}
			if !found {
				add("ATTRIBUTE_REQUIRED", definition.Key, "缺少项目明确要求的扩展属性："+definition.Key)
			}
		}
		for _, value := range object.Version.Attributes {
			found := false
			for _, definition := range definitions {
				if definition.ID == value.DefinitionID && definition.Version == value.DefinitionVersion {
					found = true
					if e := ValidateAttribute(value, definition, object.Kind); e != nil {
						add(e.Error(), definition.Key, "扩展属性类型、单位或审核状态不符合定义："+definition.Key)
					}
				}
			}
			if !found {
				add("ATTRIBUTE_DEFINITION_UNAVAILABLE", value.DefinitionID, "扩展属性定义未找到")
			}
		}
	}
	return issues
}

func validBindingField(field string) bool {
	_, _, extension := AttributeBinding(field)
	return bindingFields[field] || extension
}
func extractAttributes(source, revision string, out *Extraction, rows map[string][]rowData, definitions []AttributeDefinition, used map[string]bool) {
	regionKinds := map[string]string{"equipment": "equipment", "assembly": "assembly", "site": "facility", "room": "facility", "rack": "facility", "placement": "placement", "port": "port", "connection": "connection", "cable": "cable_plan", "power": "power_plan", "material": "material_estimate", "workload": "workload_estimate"}
	for _, object := range ModelObjects(&out.Model) {
		assigned := map[string]TypedValue{}
		for _, row := range rows[object.Version.EntityID] {
			kind := regionKinds[row.kind]
			if strings.HasPrefix(row.kind, "network_") {
				kind = "network_plan"
			}
			if kind != object.Kind {
				continue
			}
			if kind == "facility" && object.Label != row.values["label"] && object.Label != row.values[row.kind] {
				continue
			}
			if kind == "port" && object.Label != row.values["port"] {
				continue
			}
			for field, text := range row.values {
				id, version, ok := AttributeBinding(field)
				if !ok || text == "" {
					continue
				}
				addIssue := func(code, message string) {
					out.Issues = append(out.Issues, ValidationIssue{ID: StableID(source, "attribute-source", object.Version.EntityID+field+code), EntityID: object.Version.EntityID, Code: code, ModuleID: "workbook_intelligence", Field: field, Severity: "BLOCK", Message: message, PublishTarget: object.Target, EvidenceIDs: []string{}})
				}
				var definition *AttributeDefinition
				for i := range definitions {
					d := &definitions[i]
					if d.ID == id && d.Version == version && d.EntityKind == kind && d.ApprovalStatus == "approved" {
						definition = d
						break
					}
				}
				if definition == nil {
					addIssue("ATTRIBUTE_DEFINITION_UNAVAILABLE", "扩展属性必须引用已批准的当前项目定义")
					continue
				}
				value := TypedValue{Kind: definition.ValueType}
				switch value.Kind {
				case "text":
					value.Text = &text
				case "decimal":
					value.Decimal = &text
					unit := row.units[field]
					value.Unit = &unit
				case "boolean":
					value.Boolean = networkBool(text)
				case "reference":
					value.Reference = &text
				}
				a := AttributeValue{DefinitionID: id, DefinitionVersion: version, Value: value}
				if e := ValidateAttribute(a, *definition, kind); e != nil {
					addIssue(e.Error(), "来源扩展值不符合已批准的类型或单位；请核对来源")
					continue
				}
				if prior, exists := assigned[id]; exists {
					if Digest(prior) != Digest(value) {
						addIssue("ATTRIBUTE_SOURCE_CONFLICT", "同一对象的扩展属性来源冲突")
					}
					continue
				}
				assigned[id] = value
				object.Version.Attributes = append(object.Version.Attributes, a)
				subject := object.Version.EntityID
				out.Facts = append(out.Facts, SemanticFact{ID: StableID(source, "attribute-fact", row.regionID+subject+field), ProjectID: out.Model.ProjectID, RevisionID: revision, RegionID: row.regionID, SubjectID: &subject, Predicate: field, Value: value, ClaimStatus: "proposed", Maturity: Estimated, ProvenanceID: source})
				for _, cell := range row.cells[field] {
					if cell.Sensitive || cell.Ref == "" {
						continue
					}
					e := SourceEvidence{ID: StableID(source, "evidence", subject+"/"+field+"/"+cell.Sheet+"/"+cell.Ref), SourceID: source, EntityID: subject, Sheet: cell.Sheet, Cell: cell.Ref, Field: field, Formula: cell.Formula != ""}
					out.Evidence = append(out.Evidence, e)
					object.Version.EvidenceIDs = append(object.Version.EvidenceIDs, e.ID)
					used[cell.Sheet+"/"+cell.Ref] = true
				}
			}
		}
	}
}
