package engineering

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"keel.local/dev/internal/bom"
)

func StableID(project, kind, key string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("engineering/"+project+"/"+kind+"/"+strings.TrimSpace(key))).String()
}
func Digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func EmptyPhysical(project string) PhysicalModel {
	return PhysicalModel{ProjectID: project, NetworkPlans: []NetworkPlan{}, Materials: []MaterialEstimate{}, Workloads: []WorkloadEstimate{}, PowerPlans: []PowerPlan{}, Equipment: []EquipmentPlan{}, Assemblies: []DeviceAssembly{}, Facilities: []FacilityModel{}, Placements: []RackPlacement{}, Ports: []PlannedPort{}, Connections: []PlannedConnection{}, Cables: []CablePlan{}, Optics: []OpticDemand{}}
}

var bindingFields = map[string]bool{}

func init() {
	for _, f := range strings.Fields("label equipment_type model quantity identity_kind serial parent child relation_type slot site room rack capacity_u width_mm height_mm depth_mm x_mm y_mm z_mm orientation_degrees coordinate_frame start_u height_u mount_face device port parent_port breakout_index role rate_bps medium a_device a_port z_device z_port a_parent_port z_parent_port a_optic z_optic connector_a connector_z cable_label segment_index cable_model length length_unit length_state standard_length standard_length_unit patch_device patch_port patch_exit_port power power_unit fabric fabric_kind plane interface allocation_kind address prefix_length asn local_device local_interface peer_device peer_interface local_asn peer_asn afi safi bfd_tx_ms bfd_rx_ms bfd_multiplier roce_version pfc_enabled ecn_enabled lossless_priorities adaptive_enabled policy_ref vni vlan vrf_ref vtep_device vtep_interface vtep_address route_distinguisher import_targets export_targets power_basis required_quantity purchased_quantity spare_quantity purchase_gap quantity_unit work_type work_quantity work_unit labor_hours labor_unit cooling_type power_capacity power_capacity_unit cooling_capacity cooling_capacity_unit route_from route_to distance distance_unit") {
		bindingFields[f] = true
	}
}

// BindingFields is the canonical field vocabulary shared with Agent tool schemas.
func BindingFields() []string {
	fields := make([]string, 0, len(bindingFields))
	for field := range bindingFields {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func sourceCells(w bom.Workbook) map[string]map[string]bom.Cell {
	out := map[string]map[string]bom.Cell{}
	for _, sheet := range w.Sheets {
		out[sheet.Name] = map[string]bom.Cell{}
		for row, cols := range bom.MergedSourceCells(sheet) {
			for col, c := range cols {
				ref, _ := excelize.CoordinatesToCellName(col, row)
				c.Sheet = sheet.Name
				out[sheet.Name][ref] = c
			}
		}
	}
	return out
}
func CheckProposal(w bom.Workbook, p EngineeringProposal) error {
	recordSources := sourceCells(w)
	if p.Summary == "" || len(p.Regions) == 0 {
		return fmt.Errorf("ENGINEERING_REGIONS_REQUIRED")
	}
	sheets := map[string]bom.Sheet{}
	for _, s := range w.Sheets {
		sheets[s.Name] = s
	}
	for _, r := range p.Regions {
		s, ok := sheets[r.Sheet]
		if !ok {
			return fmt.Errorf("UNKNOWN_SHEET: %s", r.Sheet)
		}
		maxRow, maxCol := 0, 0
		cells := map[string]bom.Cell{}
		for _, c := range s.Cells {
			cells[c.Ref] = c
			if c.Row > maxRow {
				maxRow = c.Row
			}
			if c.Column > maxCol {
				maxCol = c.Column
			}
		}
		b := r.Bounds
		if b.StartRow < 1 || b.EndRow < b.StartRow || b.EndRow > maxRow || b.StartColumn < 1 || b.EndColumn < b.StartColumn || b.EndColumn > maxCol {
			return fmt.Errorf("INVALID_REGION: %s", r.Sheet)
		}
		switch r.Kind {
		case "equipment", "assembly", "site", "room", "rack", "placement", "port", "connection", "cable", "power", "material", "workload", "network_fabric", "network_plane", "network_role", "network_interface", "network_allocation", "network_bgp", "network_bfd", "network_qos", "network_ar", "network_evpn", "other":
		default:
			return fmt.Errorf("UNKNOWN_REGION_KIND")
		}
		if len(r.Records) > 0 && len(r.Columns) > 0 {
			return fmt.Errorf("MIXED_REGION_BINDINGS")
		}
		for _, record := range r.Records {
			bound := map[string]bool{}
			if len(record.Cells) == 0 {
				return fmt.Errorf("EMPTY_RECORD_BINDING")
			}
			for _, b := range record.Cells {
				if !validRegionField(r.Kind, b.Field) || usedConstant(r.Constants, b.Field) || bound[b.Field] {
					return fmt.Errorf("INVALID_RECORD_FIELD")
				}
				bound[b.Field] = true
				_, ok := sheets[b.Sheet]
				if !ok {
					return fmt.Errorf("UNKNOWN_RECORD_SHEET")
				}
				_, _, e := excelize.CellNameToCoordinates(b.Cell)
				if e != nil {
					return fmt.Errorf("INVALID_RECORD_CELL")
				}
				cell, ok := recordSources[b.Sheet][b.Cell]
				if !ok || cell.Sensitive {
					return fmt.Errorf("UNAVAILABLE_RECORD_CELL")
				}
			}
		}
		used := map[string]bool{}
		for _, c := range r.Columns {
			if !validRegionField(r.Kind, c.Field) {
				return fmt.Errorf("FIELD_NOT_SUPPORTED_FOR_REGION: kind=%s field=%s; supported=%s", r.Kind, c.Field, strings.Join(RegionFields(r.Kind), ","))
			}
			if used[c.Field] {
				return fmt.Errorf("DUPLICATE_FIELD_BINDING: sheet=%s kind=%s field=%s; split side-by-side tables into separate regions or use records; each field occurs once per object", r.Sheet, r.Kind, c.Field)
			}
			if c.Column < b.StartColumn || c.Column > b.EndColumn {
				return fmt.Errorf("FIELD_COLUMN_OUTSIDE_REGION: sheet=%s field=%s column=%d bounds=%d..%d", r.Sheet, c.Field, c.Column, b.StartColumn, b.EndColumn)
			}
			used[c.Field] = true
		}
		for _, c := range r.Constants {
			if !validRegionField(r.Kind, c.Field) || used[c.Field] || len(c.EvidenceCells) == 0 {
				return fmt.Errorf("INVALID_CONSTANT_EVIDENCE: %s", c.Field)
			}
			used[c.Field] = true
			matches := false
			for _, ref := range c.EvidenceCells {
				cell, ok := cells[ref]
				if !ok || cell.Sensitive {
					return fmt.Errorf("INVALID_CONSTANT_CELL")
				}
				if strings.TrimSpace(cell.Value) == strings.TrimSpace(c.Value) {
					matches = true
				}
			}
			switch c.Field {
			case "equipment_type", "identity_kind", "relation_type", "mount_face", "role", "medium", "length_state", "power_basis", "cooling_type", "work_type", "fabric_kind", "allocation_kind":
				matches = true
			}
			if !matches {
				return fmt.Errorf("UNSUPPORTED_CONSTANT_VALUE: %s", c.Field)
			}
		}
		for _, h := range r.HeaderRows {
			if h < 1 || h > b.EndRow {
				return fmt.Errorf("INVALID_HEADER_ROW")
			}
		}
	}
	seen := map[string]bool{}
	registry := DefaultRegistry()
	for _, m := range p.Modules {
		if _, ok := registry.Definition(m.ModuleID); !ok || seen[m.ModuleID] || m.Reason == "" {
			return fmt.Errorf("INVALID_MODULE_DETECTION")
		}
		seen[m.ModuleID] = true
		for _, s := range m.Sheets {
			if _, ok := sheets[s]; !ok {
				return fmt.Errorf("UNKNOWN_MODULE_SOURCE")
			}
		}
	}
	return nil
}

func usedConstant(constants []ConstantBinding, field string) bool {
	for _, c := range constants {
		if c.Field == field {
			return true
		}
	}
	return false
}

type rowData struct {
	regionID string
	kind     string
	values   map[string]string
	cells    map[string][]bom.Cell
	units    map[string]string
}

type Extraction struct {
	Resolutions             []DifferenceResolution
	Model                   PhysicalModel
	Facts                   []SemanticFact
	Evidence                []SourceEvidence
	Issues                  []ValidationIssue
	TotalCells, MappedCells int
}

func ExtractPhysical(w bom.Workbook, p EngineeringProposal, project, source, revision string, definitions ...AttributeDefinition) (Extraction, error) {
	out := Extraction{Model: EmptyPhysical(project), Evidence: []SourceEvidence{}, Issues: []ValidationIssue{}}
	if e := CheckProposal(w, p); e != nil {
		return out, e
	}
	sheets := map[string]bom.Sheet{}
	for _, s := range w.Sheets {
		sheets[s.Name] = s
		out.TotalCells += len(s.Cells)
	}
	used := map[string]bool{}
	equipment := map[ID]int{}
	facilities := map[ID]int{}
	placements := map[ID]int{}
	addIssue := func(id, code, field, message, target string) {
		out.Issues = append(out.Issues, ValidationIssue{ID: StableID(project, "issue", revision+id+code+field), ProjectID: project, RevisionID: revision, EntityID: id, Code: code, Severity: "BLOCK", ModuleID: "workbook_intelligence", Field: field, Message: message, PublishTarget: target, Resolution: "open", EvidenceIDs: []ID{}})
	}
	rows := []rowData{}
	recordSources := sourceCells(w)
	for regionIndex, region := range p.Regions {
		if region.Kind == "other" {
			continue
		}
		s := sheets[region.Sheet]
		index := map[string]bom.Cell{}
		for row, columns := range bom.MergedSourceCells(s) {
			for column, c := range columns {
				index[fmt.Sprintf("%d:%d", row, column)] = c
			}
		}
		raw := map[string]bom.Cell{}
		for _, c := range s.Cells {
			raw[c.Ref] = c
		}
		headers := map[int]bool{}
		for _, h := range region.HeaderRows {
			headers[h] = true
		}
		regionID := StableID(source, "region", fmt.Sprint(regionIndex))
		for _, record := range region.Records {
			r := rowData{regionID: regionID, kind: region.Kind, values: map[string]string{}, cells: map[string][]bom.Cell{}, units: map[string]string{}}
			for _, b := range record.Cells {
				c := recordSources[b.Sheet][b.Cell]
				c.Sheet = b.Sheet
				r.values[b.Field] = strings.TrimSpace(c.Value)
				if b.Unit != "" {
					r.values[b.Field] = normalizeUnit(c.Value, b.Unit, b.Field)
				}
				r.units[b.Field] = b.Unit
				r.cells[b.Field] = []bom.Cell{c}
			}
			for _, b := range region.Constants {
				r.values[b.Field] = b.Value
				for _, ref := range b.EvidenceCells {
					c := raw[ref]
					c.Sheet = region.Sheet
					r.cells[b.Field] = append(r.cells[b.Field], c)
				}
			}
			rows = append(rows, r)
		}
		if len(region.Records) > 0 {
			continue
		}
		for row := region.Bounds.StartRow; row <= region.Bounds.EndRow; row++ {
			if headers[row] {
				continue
			}
			r := rowData{regionID: regionID, kind: region.Kind, values: map[string]string{}, cells: map[string][]bom.Cell{}, units: map[string]string{}}
			nonempty := false
			for _, b := range region.Columns {
				c := index[fmt.Sprintf("%d:%d", row, b.Column)]
				if c.Sensitive || c.Ref == "" {
					continue
				}
				r.values[b.Field] = strings.TrimSpace(c.Value)
				r.units[b.Field] = b.Unit
				if b.Unit != "" && strings.TrimSpace(c.Value) != "" {
					r.values[b.Field] = strings.TrimSpace(c.Value)
					if b.Unit != "" {
						r.values[b.Field] = normalizeUnit(c.Value, b.Unit, b.Field)
					}
				}
				r.cells[b.Field] = []bom.Cell{c}
				if strings.TrimSpace(c.Value) != "" {
					nonempty = true
				}
			}
			if !nonempty {
				continue
			}
			for _, b := range region.Constants {
				r.values[b.Field] = b.Value
				for _, ref := range b.EvidenceCells {
					r.cells[b.Field] = append(r.cells[b.Field], raw[ref])
				}
			}
			rows = append(rows, r)
		}
	}
	attributeRows := map[string][]rowData{}
	makeVersion := func(id string, r rowData, fields ...string) EntityVersion {
		attributeRows[id] = append(attributeRows[id], r)
		v := EntityVersion{EntityID: id, RevisionID: revision, ProvenanceID: source, Maturity: Estimated, EvidenceIDs: []ID{}, Attributes: []AttributeValue{}}
		for _, f := range fields {
			if value := r.values[f]; value != "" {
				typed := TypedValue{Kind: "text", Text: &value}
				if unit := r.units[f]; decimal(value) && unit != "" {
					if strings.HasSuffix(f, "_mm") {
						unit = "mm"
					}
					typed = TypedValue{Kind: "decimal", Decimal: &value, Unit: &unit}
				}
				subject := id
				out.Facts = append(out.Facts, SemanticFact{ID: StableID(source, "fact", r.regionID+"/"+id+"/"+f+"/"+value), ProjectID: project, RevisionID: revision, RegionID: r.regionID, SubjectID: &subject, Predicate: f, Value: typed, ClaimStatus: "proposed", Maturity: Estimated, ProvenanceID: source})
			}
			for _, c := range r.cells[f] {
				if c.Ref == "" || c.Sensitive {
					continue
				}
				e := SourceEvidence{SourceID: source, ID: StableID(source, "evidence", id+"/"+f+"/"+c.Sheet+"/"+c.Ref), EntityID: id, Sheet: c.Sheet, Cell: c.Ref, Field: f, Formula: c.Formula != ""}
				out.Evidence = append(out.Evidence, e)
				v.EvidenceIDs = append(v.EvidenceIDs, e.ID)
				used[c.Sheet+"/"+c.Ref] = true
			}
		}
		return v
	}
	textPtr := func(s string) *string {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return &s
	}
	integer := func(s string) *int {
		v, e := strconv.Atoi(bom.RackUnitValue("u_start", s))
		if e != nil {
			return nil
		}
		return &v
	}
	dimension := func(r rowData, field, target, id string) *string {
		value := r.values[field]
		if value == "" {
			return nil
		}
		if !decimal(value) {
			addIssue(id, "NUMERIC_VALUE_UNRESOLVED", field, "来源数值/单位无法确定，已保留原始证据", target)
			return nil
		}
		return &value
	}
	facility := func(kind, label, parent string, r rowData) ID {
		id := StableID(project, kind, parent+"/"+label)
		v := makeVersion(id, r, "label", "site", "room", "rack", "capacity_u", "width_mm", "height_mm", "depth_mm", "x_mm", "y_mm", "z_mm", "orientation_degrees", "coordinate_frame")
		f := FacilityModel{EntityVersion: v, Kind: kind, Label: label, ParentID: textPtr(parent)}
		if r.kind == kind {
			f.CapacityU = integer(r.values["capacity_u"])
			f.WidthMM = dimension(r, "width_mm", FacilityTarget, id)
			f.HeightMM = dimension(r, "height_mm", FacilityTarget, id)
			f.DepthMM = dimension(r, "depth_mm", FacilityTarget, id)
			f.XMM = dimension(r, "x_mm", FacilityTarget, id)
			f.YMM = dimension(r, "y_mm", FacilityTarget, id)
			f.ZMM = dimension(r, "z_mm", FacilityTarget, id)
			f.OrientationDegrees = dimension(r, "orientation_degrees", FacilityTarget, id)
			if r.values["coordinate_frame"] != "" {
				f.CoordinateFrameID = textPtr(StableID(project, "frame", r.values["coordinate_frame"]))
			}
		}
		if idx, ok := facilities[id]; ok {
			old := &out.Model.Facilities[idx]
			old.EvidenceIDs = append(old.EvidenceIDs, v.EvidenceIDs...)
			if f.CapacityU != nil {
				if old.CapacityU != nil && *old.CapacityU != *f.CapacityU {
					addIssue(id, "SOURCE_CONFLICT", "capacity_u", "来源机柜容量冲突", FacilityTarget)
				} else {
					old.CapacityU = f.CapacityU
				}
			}
			for _, pair := range [][2]**string{{&old.WidthMM, &f.WidthMM}, {&old.HeightMM, &f.HeightMM}, {&old.DepthMM, &f.DepthMM}, {&old.XMM, &f.XMM}, {&old.YMM, &f.YMM}, {&old.ZMM, &f.ZMM}, {&old.OrientationDegrees, &f.OrientationDegrees}, {&old.CoordinateFrameID, &f.CoordinateFrameID}} {
				if *pair[1] != nil {
					if *pair[0] != nil && **pair[0] != **pair[1] {
						addIssue(id, "SOURCE_CONFLICT", "geometry", "来源空间参数冲突", FacilityTarget)
					} else {
						*pair[0] = *pair[1]
					}
				}
			}
		} else {
			facilities[id] = len(out.Model.Facilities)
			out.Model.Facilities = append(out.Model.Facilities, f)
		}
		return id
	}
	for _, r := range rows {
		v := r.values
		site, room, rack := "", "", ""
		if r.kind == "site" && v["label"] != "" {
			v["site"] = v["label"]
		}
		if r.kind == "room" && v["label"] != "" {
			v["room"] = v["label"]
		}
		if r.kind == "rack" && v["label"] != "" {
			v["rack"] = v["label"]
		}
		if v["site"] != "" {
			site = facility("site", v["site"], "", r)
		}
		if v["room"] != "" {
			room = facility("room", v["room"], site, r)
		}
		if v["rack"] != "" {
			rack = facility("rack", v["rack"], room, r)
		}
		if r.kind == "equipment" {
			label := v["label"]
			identity := v["identity_kind"]
			q := int64(0)
			if label != "" {
				q = 1
				if identity == "" {
					identity = "planned_instance"
				}
			} else {
				label = v["model"]
				identity = "group"
			}
			if v["quantity"] != "" {
				q, _ = strconv.ParseInt(v["quantity"], 10, 64)
				if q != 1 {
					identity = "group"
				}
			}
			id := StableID(project, "equipment", label)
			e := EquipmentPlan{EntityVersion: makeVersion(id, r, "label", "equipment_type", "model", "quantity", "identity_kind", "serial"), EquipmentType: v["equipment_type"], SourceModelLabel: v["model"], Quantity: q, IdentityKind: identity, Label: label, Serial: textPtr(v["serial"])}
			if idx, exists := equipment[id]; exists {
				old := out.Model.Equipment[idx]
				old.EntityVersion = e.EntityVersion
				if Digest(old) != Digest(e) {
					addIssue(id, "EQUIPMENT_IDENTITY_AMBIGUOUS", "label", "多个来源对同一标签提供不同设备事实", EquipmentTarget)
				}
				out.Model.Equipment[idx].EvidenceIDs = append(out.Model.Equipment[idx].EvidenceIDs, e.EvidenceIDs...)
			} else {
				equipment[id] = len(out.Model.Equipment)
				out.Model.Equipment = append(out.Model.Equipment, e)
			}
		}
		if (r.kind == "equipment" || r.kind == "placement") && (rack != "" || v["start_u"] != "") {
			id := StableID(project, "placement", v["label"])
			eq := StableID(project, "equipment", v["label"])
			if rack == "" {
				rack = StableID(project, "unresolved", "rack")
			}
			face := v["mount_face"]
			if face == "" {
				face = "unknown"
			}
			p := RackPlacement{EntityVersion: makeVersion(id, r, "label", "site", "room", "rack", "start_u", "height_u", "mount_face", "depth_mm"), EquipmentID: eq, RackID: rack, StartU: integer(v["start_u"]), HeightU: integer(v["height_u"]), MountFace: face, DepthMM: dimension(r, "depth_mm", PlacementTarget, id)}
			if idx, exists := placements[id]; exists {
				old := out.Model.Placements[idx]
				old.EntityVersion = p.EntityVersion
				if Digest(old) != Digest(p) {
					addIssue(id, "PLACEMENT_SOURCE_CONFLICT", "rack_id", "同一设备的来源落位不一致", PlacementTarget)
				}
				out.Model.Placements[idx].EvidenceIDs = append(out.Model.Placements[idx].EvidenceIDs, p.EvidenceIDs...)
			} else {
				placements[id] = len(out.Model.Placements)
				out.Model.Placements = append(out.Model.Placements, p)
			}
		}
	}
	for _, r := range rows {
		if r.kind != "assembly" {
			continue
		}
		v := r.values
		parent, child := StableID(project, "equipment", v["parent"]), StableID(project, "equipment", v["child"])
		id := StableID(project, "assembly", parent+"/"+child)
		quantity := int64(1)
		if v["quantity"] != "" {
			quantity, _ = strconv.ParseInt(v["quantity"], 10, 64)
		}
		out.Model.Assemblies = append(out.Model.Assemblies, DeviceAssembly{EntityVersion: makeVersion(id, r, "parent", "child", "relation_type", "quantity", "slot"), ParentID: parent, ChildID: child, RelationType: v["relation_type"], Quantity: quantity, SlotLabel: textPtr(v["slot"])})
	}
	extractConnectivity(project, source, revision, rows, &out, makeVersion, addIssue)
	extractPower(project, rows, &out, makeVersion, addIssue)
	extractEstimates(project, rows, &out, makeVersion, addIssue)
	extractNetwork(project, source, revision, rows, &out, makeVersion, addIssue)
	extractAttributes(source, revision, &out, attributeRows, definitions, used)
	out.MappedCells = len(used)
	out.Issues = append(out.Issues, ValidatePhysical(out.Model)...)
	// Omitted regions remain represented by source retention and an explicit issue.
	if out.MappedCells < out.TotalCells {
		out.Issues = append(out.Issues, ValidationIssue{ID: StableID(project, "issue", revision+"unmapped"), ProjectID: project, RevisionID: revision, EntityID: source, Code: "SOURCE_INFORMATION_RETAINED", Severity: "WARNING", ModuleID: "workbook_intelligence", Field: "coverage", Message: fmt.Sprintf("%d个单元格保留在来源中，未映射为工程模型；不代表资料已全部理解", out.TotalCells-out.MappedCells), PublishTarget: "", Resolution: "open", EvidenceIDs: []ID{}})
	}
	sort.Slice(out.Evidence, func(i, j int) bool { return out.Evidence[i].ID < out.Evidence[j].ID })
	return out, nil
}
func normalizeUnit(value, unit, field string) string {
	s := strings.TrimSpace(value)
	if field == "start_u" || field == "height_u" || field == "capacity_u" {
		return bom.RackUnitValue("u_start", s)
	}
	if strings.HasSuffix(field, "_mm") {
		scales := map[string]string{"mm": "1", "cm": "10", "m": "1000", "in": "25.4", "inch": "25.4", "ft": "304.8"}
		scale, ok := scales[strings.ToLower(unit)]
		if !ok {
			return "[UNRESOLVED_UNIT]"
		}
		if strings.HasSuffix(strings.ToLower(s), strings.ToLower(unit)) {
			s = strings.TrimSpace(s[:len(s)-len(unit)])
		}
		if !decimal(s) {
			return s
		}
		n, ok := new(big.Rat).SetString(s)
		if !ok {
			return s
		}
		factor, _ := new(big.Rat).SetString(scale)
		n.Mul(n, factor)
		precision := 0
		for _, v := range []string{s, scale} {
			if dot := strings.IndexByte(v, '.'); dot >= 0 {
				precision += len(v) - dot - 1
			}
		}
		return n.FloatString(precision)
	}
	return s
}

// SourceRange serves local human review. It never evaluates formulas.
func SourceRange(w bom.Workbook, sheet string, b Bounds) ([]bom.Cell, error) {
	if b.StartRow < 1 || b.EndRow < b.StartRow || b.EndRow-b.StartRow >= 32 || b.StartColumn < 1 || b.EndColumn < b.StartColumn || b.EndColumn-b.StartColumn >= 64 {
		return nil, fmt.Errorf("RANGE_LIMIT")
	}
	for _, s := range w.Sheets {
		if s.Name != sheet {
			continue
		}
		out := []bom.Cell{}
		for _, c := range s.Cells {
			if c.Row >= b.StartRow && c.Row <= b.EndRow && c.Column >= b.StartColumn && c.Column <= b.EndColumn && !c.Sensitive {
				out = append(out, c)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("UNKNOWN_SHEET")
}
