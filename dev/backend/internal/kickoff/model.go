package kickoff

import (
	"fmt"
	"github.com/google/uuid"
	"keel.local/dev/internal/engineering"
	"strconv"
	"strings"
	"time"
)

type Evidence struct {
	SourceID string `json:"sourceId"`
	Sheet    string `json:"sheet"`
	Cell     string `json:"cell"`
	Method   string `json:"method"`
	Note     string `json:"note,omitempty"`
}
type Device struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Serial            string   `json:"serial"`
	Type              string   `json:"type"`
	Model             string   `json:"model"`
	RackID            string   `json:"rackId"`
	StartU            *int     `json:"startU"`
	HeightU           *int     `json:"heightU"`
	Role              string   `json:"role"`
	ManagementAddress string   `json:"managementAddress"`
	Evidence          Evidence `json:"evidence"`
}
type Port struct {
	ID          string   `json:"id"`
	DeviceID    string   `json:"deviceId"`
	Name        string   `json:"name"`
	MAC         string   `json:"mac,omitempty"`
	Speed       string   `json:"speed"`
	Role        string   `json:"role"`
	ComponentID string   `json:"componentId,omitempty"`
	Evidence    Evidence `json:"evidence"`
}
type Connection struct {
	ID       string   `json:"id"`
	A        string   `json:"a"`
	B        string   `json:"b"`
	Medium   string   `json:"medium"`
	Evidence Evidence `json:"evidence"`
}
type Component struct {
	ID       string   `json:"id"`
	DeviceID string   `json:"deviceId"`
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	Model    string   `json:"model"`
	Slot     string   `json:"slot"`
	Evidence Evidence `json:"evidence"`
}
type Rack struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Room      string   `json:"room"`
	CapacityU *int     `json:"capacityU"`
	Evidence  Evidence `json:"evidence"`
}
type Issue struct {
	Action      string   `json:"action,omitempty"`
	ID          string   `json:"id"`
	Scope       string   `json:"scope"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	EntityID    string   `json:"entityId"`
	Field       string   `json:"field"`
	Impact      string   `json:"impact"`
	Resolved    bool     `json:"resolved"`
	Evidence    Evidence `json:"evidence"`
}
type ModelInput struct {
	TableID   string              `json:"tableId"`
	Rows      []map[string]string `json:"rows"`
	Version   int                 `json:"version"`
	Confirmed bool                `json:"confirmed"`
	UpdatedAt time.Time           `json:"updatedAt"`
	Source    string              `json:"source"`
}
type View struct {
	Devices     []Device     `json:"devices"`
	Ports       []Port       `json:"ports"`
	Connections []Connection `json:"connections"`
	Components  []Component  `json:"components"`
	Racks       []Rack       `json:"racks"`
	Issues      []Issue      `json:"issues"`
}

func Rows(inputs []ModelInput, table string) []map[string]string {
	for _, i := range inputs {
		if i.TableID == table {
			return i.Rows
		}
	}
	return nil
}
func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func IsDevice(e engineering.EquipmentPlan) bool {
	return e.EquipmentType == "server" || e.EquipmentType == "switch"
}
func Project(m engineering.PhysicalModel, ev []engineering.SourceEvidence, source string, inputs []ModelInput) View {
	v := View{Devices: []Device{}, Ports: []Port{}, Connections: []Connection{}, Components: []Component{}, Racks: []Rack{}, Issues: []Issue{}}
	evidence := func(id string) Evidence {
		for _, e := range ev {
			if e.EntityID == id {
				method := "source"
				if e.Sheet == "[manual]" {
					method = "manual"
				} else if strings.HasPrefix(e.Sheet, "[") {
					method = "mapping"
				}
				return Evidence{SourceID: source, Sheet: e.Sheet, Cell: e.Cell, Method: method}
			}
		}
		return Evidence{SourceID: source, Method: "mapping"}
	}
	parent := map[string]string{}
	slot := map[string]string{}
	for _, a := range m.Assemblies {
		parent[a.ChildID] = a.ParentID
		slot[a.ChildID] = stringValue(a.SlotLabel)
	}
	root := func(id string) string {
		seen := map[string]bool{}
		for parent[id] != "" && !seen[id] {
			seen[id] = true
			id = parent[id]
		}
		return id
	}
	for _, e := range m.Equipment {
		if e.IdentityKind == "group" {
			continue
		}
		if IsDevice(e) {
			d := Device{ID: e.EntityID, Name: e.Label, Serial: stringValue(e.Serial), Type: e.EquipmentType, Model: e.SourceModelLabel, HeightU: e.HeightU, Evidence: evidence(e.EntityID)}
			for _, p := range m.Placements {
				if p.EquipmentID == e.EntityID {
					d.RackID = p.RackID
					d.StartU = p.StartU
					if p.HeightU != nil {
						d.HeightU = p.HeightU
					}
				}
			}
			for _, r := range Rows(inputs, "management") {
				if r["device_id"] == d.ID {
					d.ManagementAddress = r["management_ip"]
				}
			}
			for _, r := range Rows(inputs, "network_roles") {
				if r["device_id"] == d.ID {
					d.Role = r["role"]
				}
			}
			v.Devices = append(v.Devices, d)
		} else if parent[e.EntityID] != "" {
			v.Components = append(v.Components, Component{ID: e.EntityID, DeviceID: root(e.EntityID), Type: e.EquipmentType, Name: e.Label, Model: e.SourceModelLabel, Slot: slot[e.EntityID], Evidence: evidence(e.EntityID)})
		}
	}
	for _, r := range m.Facilities {
		if r.Kind == "rack" {
			v.Racks = append(v.Racks, Rack{ID: r.EntityID, Name: r.Label, Room: stringValue(r.ParentID), CapacityU: r.CapacityU, Evidence: evidence(r.EntityID)})
		}
	}
	media := map[string]string{}
	for _, p := range m.Ports {
		id := root(p.EquipmentID)
		component := ""
		if id != p.EquipmentID {
			component = p.EquipmentID
		}
		speed := ""
		if p.RateBPS != nil {
			speed = strconv.FormatInt(*p.RateBPS, 10) + " bps"
		}
		v.Ports = append(v.Ports, Port{ID: p.EntityID, DeviceID: id, Name: p.Name, MAC: stringValue(p.MAC), Speed: speed, Role: p.Role, ComponentID: component, Evidence: evidence(p.EntityID)})
		media[p.EntityID] = p.Medium
	}
	for _, c := range m.Connections {
		v.Connections = append(v.Connections, Connection{ID: c.EntityID, A: c.APortID, B: c.ZPortID, Medium: media[c.APortID], Evidence: evidence(c.EntityID)})
	}
	for _, i := range ValidationIssues(m) {
		scope := "configuration"
		if i.PublishTarget == engineering.EquipmentTarget || i.PublishTarget == engineering.ConnectivityTarget {
			scope = "identity"
		}
		if i.PublishTarget == engineering.PlacementTarget {
			scope = "placement"
		}
		entity := i.EntityID
		action := ""
		if scope == "configuration" {
			action = "models/cabling"
			if i.ModuleID == "facility" {
				action = "models/facility?table=racks"
			}
			if i.ModuleID == "power" {
				action = "models/power"
			}
			if i.ModuleID == "network" {
				action = "models/network"
			}
		}
		if scope == "placement" {
			for _, p := range m.Placements {
				if p.EntityID == i.EntityID {
					entity = p.EquipmentID
					if p.StartU == nil {
						action = "models/facility?table=placements"
					}
					if i.Field == "capacity_u" || strings.Contains(i.Code, "CAPACITY") {
						action = "models/facility?table=racks"
					}
				}
			}
		}
		if scope == "identity" && i.Field != "serial" {
			action = "sources"
		}
		v.Issues = append(v.Issues, Issue{ID: i.ID, Scope: scope, Action: action, Title: i.Message, Description: i.Message, EntityID: entity, Field: i.Field, Impact: "仅影响对应对象及其依赖", Evidence: evidence(i.EntityID)})
	}
	return v
}
func ApplyInputs(m *engineering.PhysicalModel, inputs []ModelInput) error {
	stable := func(kind, code string) string {
		if _, e := uuid.Parse(code); e == nil {
			return code
		}
		return engineering.StableID(m.ProjectID, kind, code)
	}
	version := func(kind, code string) engineering.EntityVersion {
		return engineering.EntityVersion{EntityID: stable(kind, code), Maturity: engineering.Confirmed}
	}
	for _, r := range Rows(inputs, "rooms") {
		code := r["room_id"]
		if code == "" {
			code = r["name"]
		}
		if code == "" {
			continue
		}
		id := stable("room", code)
		f := engineering.FacilityModel{EntityVersion: version("room", code), Kind: "room", Code: code, Label: r["name"]}
		if f.Label == "" {
			f.Label = code
		}
		found := false
		for i := range m.Facilities {
			if m.Facilities[i].EntityID == id {
				if r["name"] != "" {
					m.Facilities[i].Label = r["name"]
				}
				found = true
				break
			}
		}
		if !found {
			m.Facilities = append(m.Facilities, f)
		}
	}
	for _, r := range Rows(inputs, "rooms") {
		if r["site"] == "" {
			continue
		}
		roomCode := r["room_id"]
		if roomCode == "" {
			roomCode = r["name"]
		}
		if roomCode == "" {
			continue
		}
		site := stable("site", r["site"])
		found := false
		for _, f := range m.Facilities {
			if f.Kind == "site" && (f.EntityID == r["site"] || f.Code == r["site"] || f.Label == r["site"]) {
				site = f.EntityID
				found = true
				break
			}
		}
		if !found {
			m.Facilities = append(m.Facilities, engineering.FacilityModel{EntityVersion: engineering.EntityVersion{EntityID: site, Maturity: engineering.Confirmed}, Kind: "site", Code: r["site"], Label: r["site"]})
		}
		for i := range m.Facilities {
			if m.Facilities[i].EntityID == stable("room", roomCode) {
				m.Facilities[i].ParentID = &site
			}
		}
	}
	for _, r := range Rows(inputs, "racks") {
		if r["rack_id"] == "" {
			continue
		}
		id := stable("rack", r["rack_id"])
		at := -1
		for i := range m.Facilities {
			if m.Facilities[i].EntityID == id {
				at = i
				break
			}
		}
		if at < 0 {
			m.Facilities = append(m.Facilities, engineering.FacilityModel{EntityVersion: version("rack", r["rack_id"]), Kind: "rack", Code: r["rack_id"], Label: r["rack_id"]})
			at = len(m.Facilities) - 1
		}
		f := &m.Facilities[at]
		if r["room_id"] != "" {
			room := stable("room", r["room_id"])
			exists := false
			for _, p := range m.Facilities {
				if p.EntityID == room && p.Kind == "room" {
					exists = true
				}
			}
			if !exists {
				return fmt.Errorf("机柜引用的机房不存在")
			}
			f.ParentID = &room
		}
		if r["capacity_u"] != "" {
			n, e := strconv.Atoi(r["capacity_u"])
			if e != nil || n <= 0 {
				return fmt.Errorf("机柜U数必须为正整数")
			}
			f.CapacityU = &n
		}
		for key, target := range map[string]**string{"width_mm": &f.WidthMM, "depth_mm": &f.DepthMM, "orientation": &f.OrientationDegrees} {
			if r[key] != "" {
				s := r[key]
				*target = &s
			}
		}
		if r["x"] != "" || r["y"] != "" {
			if r["frame"] == "" || r["unit"] == "" {
				return fmt.Errorf("坐标需要明确坐标系和单位")
			}
			factor := map[string]float64{"mm": 1, "cm": 10, "m": 1000}[r["unit"]]
			if factor == 0 {
				return fmt.Errorf("坐标单位不支持")
			}
			for key, target := range map[string]**string{"x": &f.XMM, "y": &f.YMM} {
				if r[key] != "" {
					n, _ := strconv.ParseFloat(r[key], 64)
					s := strconv.FormatFloat(n*factor, 'f', -1, 64)
					*target = &s
				}
			}
			frame := stable("coordinate-frame", r["frame"])
			f.CoordinateFrameID = &frame
		}
	}
	for i := range m.Equipment {
		e := &m.Equipment[i]
		for _, r := range Rows(inputs, "equipment_specs") {
			if r["model"] == e.SourceModelLabel && e.HeightU == nil && r["height_u"] != "" {
				n, err := strconv.Atoi(r["height_u"])
				if err != nil || n <= 0 {
					return fmt.Errorf("设备高度必须为正整数")
				}
				e.HeightU = &n
			}
		}
	}
	for _, r := range Rows(inputs, "placements") {
		if r["device_id"] == "" || r["rack_id"] == "" {
			continue
		}
		exists := false
		for _, e := range m.Equipment {
			if e.EntityID == r["device_id"] {
				exists = true
			}
		}
		if !exists {
			return fmt.Errorf("落位引用的设备不存在")
		}
		rack := stable("rack", r["rack_id"])
		exists = false
		for _, f := range m.Facilities {
			if f.EntityID == rack && f.Kind == "rack" {
				exists = true
			}
		}
		if !exists {
			return fmt.Errorf("落位引用的机柜不存在")
		}
		at := -1
		for i := range m.Placements {
			if m.Placements[i].EquipmentID == r["device_id"] {
				at = i
				break
			}
		}
		if at < 0 {
			m.Placements = append(m.Placements, engineering.RackPlacement{EntityVersion: version("placement", r["device_id"]), EquipmentID: r["device_id"]})
			at = len(m.Placements) - 1
		}
		p := &m.Placements[at]
		p.RackID = rack
		for key, target := range map[string]**int{"start_u": &p.StartU, "height_u": &p.HeightU} {
			if r[key] != "" {
				n, e := strconv.Atoi(r[key])
				if e != nil || n <= 0 {
					return fmt.Errorf("U位必须为正整数")
				}
				*target = &n
			}
		}
		if r["face"] != "" {
			p.MountFace = r["face"]
		}
		if r["position"] != "" {
			p.PositionCode = r["position"]
			p.InstallationType = "non_u"
		}
	}
	for table, field := range map[string]string{"management": "device_id", "network_roles": "device_id", "network_interfaces": "device_id", "network_addresses": "device_id", "network_bgp": "device_id", "boot_interfaces": "device_id", "power_profiles": "device_id", "power_connections": "device_id"} {
		for _, r := range Rows(inputs, table) {
			if strings.TrimSpace(r[field]) == "" {
				continue
			}
			found := false
			for _, e := range m.Equipment {
				if e.EntityID == r[field] {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("%s引用的设备不存在", table)
			}
		}
	}
	return nil
}

// A room can be planned before a site is supplied. Provided relationships still
// require correct types; no missing site is invented to satisfy a legacy gate.
func ValidationIssues(m engineering.PhysicalModel) []engineering.ValidationIssue {
	out := []engineering.ValidationIssue{}
	for _, issue := range engineering.ValidatePhysical(m) {
		optionalParent := false
		if issue.Code == "FACILITY_PARENT_REQUIRED" {
			for _, f := range m.Facilities {
				if f.EntityID == issue.EntityID && f.Kind == "room" && f.ParentID == nil {
					optionalParent = true
				}
			}
		}
		if !optionalParent {
			out = append(out, issue)
		}
	}
	serials := map[string][]engineering.EquipmentPlan{}
	for _, e := range m.Equipment {
		if IsDevice(e) && e.Serial != nil && *e.Serial != "" {
			serials[*e.Serial] = append(serials[*e.Serial], e)
		}
	}
	for _, entities := range serials {
		if len(entities) > 1 {
			for _, e := range entities {
				out = append(out, engineering.ValidationIssue{ID: engineering.StableID(m.ProjectID, "identity-conflict", e.EntityID), EntityID: e.EntityID, Code: "KICKOFF_DUPLICATE_SERIAL", Severity: "BLOCK", Field: "serial", Message: "相同SN存在多个规划身份，请核对关联", PublishTarget: engineering.EquipmentTarget})
			}
		}
	}
	return out
}

// Only an explicit human choice between matching unpublished serial identities
// can coalesce candidate references. The selected identity and its values survive.
func ResolveIdentity(m *engineering.PhysicalModel, from, to string) error {
	if from == to {
		return fmt.Errorf("请选择同一SN对应的另一个明确设备身份")
	}
	var a, b *engineering.EquipmentPlan
	for i := range m.Equipment {
		if m.Equipment[i].EntityID == from {
			a = &m.Equipment[i]
		}
		if m.Equipment[i].EntityID == to {
			b = &m.Equipment[i]
		}
	}
	if a == nil || b == nil || !IsDevice(*a) || !IsDevice(*b) || a.Serial == nil || b.Serial == nil || *a.Serial == "" || *a.Serial != *b.Serial || a.EquipmentType != b.EquipmentType {
		return fmt.Errorf("只有同类型、同SN且有依据的规划身份可以人工关联")
	}
	for _, p := range m.Placements {
		if p.EquipmentID == from {
			for _, q := range m.Placements {
				if q.EquipmentID == to && (p.RackID != q.RackID || stringInt(p.StartU) != stringInt(q.StartU)) {
					return fmt.Errorf("两个身份的落位冲突，请先核对设备落位")
				}
			}
		}
	}
	b.EvidenceIDs = append(b.EvidenceIDs, a.EvidenceIDs...)
	equipment := m.Equipment[:0]
	for _, e := range m.Equipment {
		if e.EntityID != from {
			equipment = append(equipment, e)
		}
	}
	m.Equipment = equipment
	for i := range m.Ports {
		if m.Ports[i].EquipmentID == from {
			m.Ports[i].EquipmentID = to
		}
	}
	for i := range m.Assemblies {
		if m.Assemblies[i].ParentID == from {
			m.Assemblies[i].ParentID = to
		}
		if m.Assemblies[i].ChildID == from {
			m.Assemblies[i].ChildID = to
		}
	}
	hasPlacement := false
	for _, p := range m.Placements {
		if p.EquipmentID == to {
			hasPlacement = true
		}
	}
	placements := m.Placements[:0]
	for _, p := range m.Placements {
		if p.EquipmentID == from {
			if hasPlacement {
				continue
			}
			p.EquipmentID = to
		}
		placements = append(placements, p)
	}
	m.Placements = placements
	for _, issue := range ValidationIssues(*m) {
		if issue.EntityID == to && issue.PublishTarget == engineering.EquipmentTarget {
			return fmt.Errorf("关联后的设备仍有问题：%s", issue.Message)
		}
	}
	return nil
}
func stringInt(n *int) string {
	if n == nil {
		return ""
	}
	return strconv.Itoa(*n)
}
