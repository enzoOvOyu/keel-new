package engineering

import (
	"fmt"
	"github.com/google/uuid"
	"math/big"
	"sort"
	"strings"
)

func (v TypedValue) Validate() error {
	count := 0
	for _, set := range []bool{v.Text != nil, v.Decimal != nil, v.Boolean != nil, v.Reference != nil} {
		if set {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("TYPED_VALUE_REQUIRES_ONE_SLOT")
	}
	switch v.Kind {
	case "text":
		if v.Text == nil || v.Unit != nil {
			return fmt.Errorf("TEXT_VALUE_INVALID")
		}
	case "decimal":
		if v.Decimal == nil || !decimal(*v.Decimal) || v.Unit == nil || strings.TrimSpace(*v.Unit) == "" {
			return fmt.Errorf("DECIMAL_VALUE_OR_UNIT_INVALID")
		}
	case "boolean":
		if v.Boolean == nil || v.Unit != nil {
			return fmt.Errorf("BOOLEAN_VALUE_INVALID")
		}
	case "reference":
		if v.Reference == nil || *v.Reference == "" || v.Unit != nil {
			return fmt.Errorf("REFERENCE_VALUE_INVALID")
		}
	default:
		return fmt.Errorf("UNKNOWN_VALUE_TYPE")
	}
	return nil
}
func decimal(s string) bool {
	// Reject fractions, exponents and non-finite values; transport is an exact
	// decimal string. A unit conversion policy belongs to an approved rule.
	if s == "" || strings.TrimSpace(s) != s {
		return false
	}
	digits, dots := 0, 0
	for i, c := range s {
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' {
			dots++
			if dots > 1 {
				return false
			}
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		digits++
	}
	return digits > 0
}
func positive(s string) bool {
	if !decimal(s) {
		return false
	}
	n, ok := new(big.Rat).SetString(s)
	return ok && n.Sign() > 0
}
func (q Quantity) Validate() error {
	if !decimal(q.Value) || strings.TrimSpace(q.Unit) == "" {
		return fmt.Errorf("INVALID_QUANTITY")
	}
	return nil
}

// PhysicalModel is an API/read aggregate; persistence stores each collection in
// typed tables. It is not a JSON replacement for the engineering domain schema.
type PhysicalModel struct {
	ProjectID  ID               `json:"project_id"`
	Equipment  []EquipmentPlan  `json:"equipment"`
	Assemblies []DeviceAssembly `json:"assemblies"`
	Facilities []FacilityModel  `json:"facilities"`
	Placements []RackPlacement  `json:"placements"`
}

func ValidatePhysical(m PhysicalModel) []ValidationIssue {
	issues := []ValidationIssue{}
	issue := func(code, target, module, field, message string, e EntityVersion) {
		issues = append(issues, ValidationIssue{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(m.ProjectID+"/"+e.RevisionID+"/"+e.EntityID+"/"+target+"/"+code+"/"+field)).String(), ProjectID: m.ProjectID, RevisionID: e.RevisionID, Resolution: "open", Code: code, Severity: "BLOCK", ModuleID: module, EntityID: e.EntityID, Field: field, Message: message, PublishTarget: target, EvidenceIDs: append([]ID{}, e.EvidenceIDs...)})
	}
	eqTarget, facTarget, locTarget := "CMDB_PLANNED_EQUIPMENT", "CMDB_PLANNED_FACILITY", "CMDB_PLANNED_PLACEMENT"
	ids := map[ID]bool{}
	checkVersion := func(e EntityVersion, target, module string) {
		if e.EntityID == "" || e.RevisionID == "" || e.ProvenanceID == "" || len(e.EvidenceIDs) == 0 {
			issue("SOURCE_REQUIRED", target, module, "evidence_ids", "工程对象必须关联版本、来源和证据", e)
		}
		if ids[e.EntityID] {
			issue("DUPLICATE_ENTITY", target, module, "entity_id", "同一快照中实体ID重复", e)
		}
		ids[e.EntityID] = true
		switch e.Maturity {
		case Estimated, Calculated, Confirmed, AsBuilt:
		default:
			issue("MATURITY_REQUIRED", target, module, "maturity", "数据成熟度必须明确", e)
		}
		seen := map[ID]bool{}
		for _, a := range e.Attributes {
			if a.DefinitionID == "" || a.DefinitionVersion < 1 || seen[a.DefinitionID] || a.Value.Validate() != nil {
				issue("ATTRIBUTE_VALUE_INVALID", target, module, "attributes", "扩展属性必须引用版本化类型并提供正确值", e)
			}
			seen[a.DefinitionID] = true
		}
	}
	equipment := map[ID]EquipmentPlan{}
	for _, e := range m.Equipment {
		checkVersion(e.EntityVersion, eqTarget, "equipment")
		if e.EquipmentType == "" || e.Label == "" || e.Quantity <= 0 {
			issue("EQUIPMENT_FIELDS_REQUIRED", eqTarget, "equipment", "quantity", "设备类型、标签及正数量必须明确", e.EntityVersion)
		}
		switch e.IdentityKind {
		case "group":
			issue("PLANNING_GROUP_REQUIRES_INSTANCE_IDENTITIES", eqTarget, "equipment", "identity_kind", "设备数量组保留为工程规划，发布物理资产前需明确各实例身份", e.EntityVersion)
			if e.Serial != nil && *e.Serial != "" {
				issue("GROUP_SERIAL_INVALID", eqTarget, "equipment", "serial", "数量组不能共享单个实物SN", e.EntityVersion)
			}
		case "planned_instance", "observed_reference":
			if e.Quantity != 1 {
				issue("INSTANCE_QUANTITY_INVALID", eqTarget, "equipment", "quantity", "单个实例数量必须为1", e.EntityVersion)
			}
		default:
			issue("IDENTITY_KIND_REQUIRED", eqTarget, "equipment", "identity_kind", "需区分数量组、规划实例与已有实物引用", e.EntityVersion)
		}
		if e.IdentityKind == "planned_instance" && e.Maturity == AsBuilt {
			issue("PLANNED_NOT_AS_BUILT", eqTarget, "equipment", "maturity", "规划身份不能标记为实际安装", e.EntityVersion)
		}
		equipment[e.EntityID] = e
	}
	children := map[ID][]ID{}
	parent := map[ID]ID{}
	for _, a := range m.Assemblies {
		checkVersion(a.EntityVersion, eqTarget, "equipment")
		p, pok := equipment[a.ParentID]
		c, cok := equipment[a.ChildID]
		if !pok || !cok || a.ParentID == a.ChildID || a.Quantity <= 0 {
			issue("ASSEMBLY_REFERENCE_INVALID", eqTarget, "equipment", "parent_id", "组合必须引用两个不同且存在的设备并声明正数量", a.EntityVersion)
			continue
		}
		switch a.RelationType {
		case "contains", "installed_in", "assembly_member":
		default:
			issue("ASSEMBLY_TYPE_INVALID", eqTarget, "equipment", "relation_type", "未知组合关系类型", a.EntityVersion)
		}
		if a.RelationType != "assembly_member" {
			if previous, exists := parent[a.ChildID]; exists && previous != a.ParentID {
				issue("MULTIPLE_PHYSICAL_PARENTS", eqTarget, "equipment", "child_id", "同一物理部件有多个父设备", a.EntityVersion)
			}
			parent[a.ChildID] = a.ParentID
			if c.IdentityKind != "group" && a.Quantity != 1 {
				issue("ASSEMBLY_INSTANCE_QUANTITY", eqTarget, "equipment", "quantity", "单个物理部件不能计为多个实例", a.EntityVersion)
			}
			if p.IdentityKind == "group" && c.IdentityKind != "group" {
				issue("ASSEMBLY_PARENT_AMBIGUOUS", eqTarget, "equipment", "parent_id", "数量组尚不能作为唯一实例的物理父设备", a.EntityVersion)
			}
		}
		children[a.ParentID] = append(children[a.ParentID], a.ChildID)
	}
	for _, a := range m.Assemblies {
		if reachable(children, a.ChildID, a.ParentID) {
			issue("ASSEMBLY_CYCLE", eqTarget, "equipment", "parent_id", "设备组合关系存在循环", a.EntityVersion)
		}
	}
	facilities := map[ID]FacilityModel{}
	for _, f := range m.Facilities {
		facilities[f.EntityID] = f
	}
	for _, f := range m.Facilities {
		checkVersion(f.EntityVersion, facTarget, "facility_placement")
		if f.Label == "" {
			issue("FACILITY_LABEL_REQUIRED", facTarget, "facility_placement", "label", "位置名称缺失", f.EntityVersion)
		}
		want := ""
		switch f.Kind {
		case "site":
		case "room":
			want = "site"
		case "rack":
			want = "room"
		default:
			issue("FACILITY_KIND_INVALID", facTarget, "facility_placement", "kind", "场地类型必须为site、room或rack", f.EntityVersion)
		}
		if want != "" {
			if f.ParentID == nil || facilities[*f.ParentID].Kind != want {
				issue("FACILITY_PARENT_REQUIRED", facTarget, "facility_placement", "parent_id", "位置上级缺失或类型不符", f.EntityVersion)
			}
		} else if f.Kind == "site" && f.ParentID != nil {
			issue("SITE_PARENT_INVALID", facTarget, "facility_placement", "parent_id", "site不能再指向一个room/rack", f.EntityVersion)
		}
		if f.CapacityU != nil && (*f.CapacityU <= 0 || f.Kind != "rack") {
			issue("RACK_CAPACITY_INVALID", facTarget, "facility_placement", "capacity_u", "容量须为机柜的正U数", f.EntityVersion)
		}
		for field, v := range map[string]*string{"width_mm": f.WidthMM, "height_mm": f.HeightMM, "depth_mm": f.DepthMM} {
			if v != nil && !positive(*v) {
				issue("FACILITY_DIMENSION_INVALID", facTarget, "facility_placement", field, "尺寸须为正毫米数", f.EntityVersion)
			}
		}
		for field, v := range map[string]*string{"x_mm": f.XMM, "y_mm": f.YMM, "z_mm": f.ZMM, "orientation_degrees": f.OrientationDegrees} {
			if v != nil && !decimal(*v) {
				issue("COORDINATE_INVALID", facTarget, "facility_placement", field, "坐标或方向格式无效", f.EntityVersion)
			}
		}
		if (f.XMM != nil || f.YMM != nil || f.ZMM != nil || f.OrientationDegrees != nil) && (f.CoordinateFrameID == nil || *f.CoordinateFrameID == "") {
			issue("COORDINATE_FRAME_REQUIRED", facTarget, "facility_placement", "coordinate_frame_id", "坐标必须声明参考系", f.EntityVersion)
		}
	}
	placed := map[ID]bool{}
	for _, p := range m.Placements {
		checkVersion(p.EntityVersion, locTarget, "facility_placement")
		eq, ok := equipment[p.EquipmentID]
		rack, rok := facilities[p.RackID]
		if !ok || eq.IdentityKind == "group" {
			issue("PLACEMENT_EQUIPMENT_UNRESOLVED", locTarget, "facility_placement", "equipment_id", "落位需引用唯一设备实例", p.EntityVersion)
		}
		if !rok || rack.Kind != "rack" {
			issue("PLACEMENT_RACK_UNRESOLVED", locTarget, "facility_placement", "rack_id", "落位需引用明确机柜", p.EntityVersion)
		}
		if placed[p.EquipmentID] {
			issue("MULTIPLE_PLACEMENTS", locTarget, "facility_placement", "equipment_id", "同一设备存在多个规划位置", p.EntityVersion)
		}
		placed[p.EquipmentID] = true
		if p.StartU == nil || p.HeightU == nil || *p.StartU <= 0 || *p.HeightU <= 0 {
			issue("PLACEMENT_U_REQUIRED", locTarget, "facility_placement", "start_u", "起始U和占用高度未知或无效", p.EntityVersion)
		} else if rack.CapacityU == nil {
			issue("PLACEMENT_CAPACITY_UNKNOWN", locTarget, "facility_placement", "rack_id", "机柜容量未知，不能确认落位未越界", p.EntityVersion)
		} else if *p.StartU > *rack.CapacityU || *p.HeightU > *rack.CapacityU-*p.StartU+1 {
			issue("PLACEMENT_OUT_OF_BOUNDS", locTarget, "facility_placement", "height_u", "设备占用超出机柜容量", p.EntityVersion)
		}
		switch p.MountFace {
		case "front", "rear", "both", "unknown":
		default:
			issue("MOUNT_FACE_INVALID", locTarget, "facility_placement", "mount_face", "安装面类型无效", p.EntityVersion)
		}
		if p.DepthMM != nil && !positive(*p.DepthMM) {
			issue("PLACEMENT_DEPTH_INVALID", locTarget, "facility_placement", "depth_mm", "设备深度须为正毫米数", p.EntityVersion)
		}
	}
	for i, a := range m.Placements {
		for _, b := range m.Placements[i+1:] {
			if a.RackID != b.RackID || a.StartU == nil || a.HeightU == nil || b.StartU == nil || b.HeightU == nil || *a.StartU <= 0 || *a.HeightU <= 0 || *b.StartU <= 0 || *b.HeightU <= 0 {
				continue
			}
			// Compare differences instead of adding potentially overflowing integers.
			overlaps := false
			if *a.StartU <= *b.StartU {
				overlaps = *b.StartU-*a.StartU < *a.HeightU
			} else {
				overlaps = *a.StartU-*b.StartU < *b.HeightU
			}
			if !overlaps {
				continue
			}
			code, message := "PLACEMENT_OVERLAP", "设备占用U位重叠"
			opposite := (a.MountFace == "front" && b.MountFace == "rear") || (a.MountFace == "rear" && b.MountFace == "front")
			if opposite {
				rack := facilities[a.RackID]
				if a.DepthMM != nil && b.DepthMM != nil && rack.DepthMM != nil && positive(*a.DepthMM) && positive(*b.DepthMM) && positive(*rack.DepthMM) {
					x, _ := new(big.Rat).SetString(*a.DepthMM)
					y, _ := new(big.Rat).SetString(*b.DepthMM)
					d, _ := new(big.Rat).SetString(*rack.DepthMM)
					if new(big.Rat).Add(x, y).Cmp(d) <= 0 {
						continue
					}
				}
				code, message = "PLACEMENT_DEPTH_UNRESOLVED", "前后同U安装需核对设备及机柜深度，不能默认不冲突"
			}
			issue(code, locTarget, "facility_placement", "start_u", message, a.EntityVersion)
			issue(code, locTarget, "facility_placement", "start_u", message, b.EntityVersion)
		}
	}
	sort.Slice(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		return a.PublishTarget+"/"+a.EntityID+"/"+a.Code+"/"+a.Field < b.PublishTarget+"/"+b.EntityID+"/"+b.Code+"/"+b.Field
	})
	return issues
}
func reachable(edges map[ID][]ID, from, to ID) bool {
	seen := map[ID]bool{}
	stack := []ID{from}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == to {
			return true
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		stack = append(stack, edges[id]...)
	}
	return false
}
