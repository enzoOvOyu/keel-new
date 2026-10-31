package cmdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type VisualAsset struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Project  string         `json:"project_id"`
	Revision int64          `json:"revision"`
	Source   string         `json:"source"`
	Fields   map[string]any `json:"fields"`
}
type VisualCable struct {
	ID      string         `json:"id"`
	A       string         `json:"a"`
	B       string         `json:"b"`
	DeviceA string         `json:"device_a"`
	DeviceB string         `json:"device_b"`
	Fact    string         `json:"fact"`
	Source  string         `json:"source"`
	Fields  map[string]any `json:"fields"`
}
type VisualFinding struct {
	Asset    string `json:"asset_id"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
	Category string `json:"category,omitempty"`
}
type VisualRelation struct {
	ID        string `json:"id"`
	From      string `json:"source_asset_id"`
	To        string `json:"target_asset_id"`
	Type      string `json:"relationship_type"`
	Fact      string `json:"fact_layer"`
	Source    string `json:"source"`
	SourceRef string `json:"source_ref"`
	Revision  int64  `json:"revision"`
}
type VisualCoverage struct {
	Returned int  `json:"returned_assets"`
	Total    int  `json:"total_assets"`
	Limit    int  `json:"limit"`
	Complete bool `json:"complete"`
}
type Visualization struct {
	SnapshotID    string           `json:"snapshot_id"`
	ReadAt        time.Time        `json:"read_at"`
	Relationships []VisualRelation `json:"relationships"`
	Coverage      VisualCoverage   `json:"coverage"`

	Assets    []VisualAsset   `json:"assets"`
	Cables    []VisualCable   `json:"cables"`
	Findings  []VisualFinding `json:"findings"`
	Total     int             `json:"total_assets"`
	Truncated bool            `json:"truncated"`
}

// Read projection over the existing authoritative entities and typed relations.
// No positions or identities are created by this service.
func (s Service) Visualization(ctx context.Context, scope Scope, rack, focus string) (Visualization, error) {
	return s.VisualizationDetail(ctx, scope, rack, focus, false, 20000)
}
func (s Service) VisualizationDetail(ctx context.Context, scope Scope, rack, focus string, full bool, limit int) (Visualization, error) {
	out := Visualization{Assets: []VisualAsset{}, Cables: []VisualCable{}, Findings: []VisualFinding{}, Relationships: []VisualRelation{}, ReadAt: time.Now().UTC()}
	if limit < 1 || limit > 20000 {
		return out, Reject("VISUALIZATION_LIMIT_INVALID", 400)
	}
	tx, e := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	fingerprint := sha256.New()

	rows, e := tx.Query(ctx, `SELECT e.item_uid,e.asset_type,e.name,e.version,e.source_type,e.source_ref,e.freshness,e.payload,count(*) OVER(),(SELECT min(sc.project_id) FROM cmdb_asset_scope sc WHERE sc.asset_id=e.item_uid AND (sc.project_id=ANY($1) OR sc.project_id||':'||sc.asset_id=ANY($2))) FROM cmdb_entity e WHERE `+Visible("e")+` AND e.asset_type IN('data_center','region','pod','rack','server','switch','nic','gpu','port','cable','optic','pdu','patch_panel','component','psu','fabric','network_segment','vlan','vrf','vni','device') ORDER BY e.item_uid LIMIT `+strconv.Itoa(limit+1), scope.Args()...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	index := map[string]*VisualAsset{}
	all := []*VisualAsset{}
	for rows.Next() {
		var a VisualAsset
		var raw []byte
		var sourceRef, freshness string
		if e = rows.Scan(&a.ID, &a.Type, &a.Name, &a.Revision, &a.Source, &sourceRef, &freshness, &raw, &out.Total, &a.Project); e != nil {
			return out, e
		}
		if len(all) == limit {
			out.Truncated = true
			break
		}
		var p map[string]any
		json.Unmarshal(raw, &p)
		a.Fields = map[string]any{"source_ref": sourceRef, "freshness": freshness}
		fmt.Fprintf(fingerprint, "%s:%d;", a.ID, a.Revision)
		for k, v := range p {
			if f, ok := v.(map[string]any); ok && k == "fields" {
				for key, val := range f {
					p[key] = val
				}
			}
		}
		for _, k := range []string{"model", "serial", "hostname", "site", "pod_id", "rack_id", "rack_uid", "rack_u", "u_start", "u_height", "face", "mounting_face", "depth_mm", "width_mm", "role", "health_status", "maintenance_state", "management_ip", "bmc_ip", "device_id", "parent_uid", "physical_name", "port_index", "speed", "media_type", "cable_type", "length_m", "length", "endpoint_a_uid", "endpoint_b_uid", "gpu_count", "gpu_model", "cpu", "cpu_model", "cpu_count", "memory", "memory_gb", "disk", "disk_spec", "nic_count", "nic_model", "bmc_model", "bios_version", "bmc_version", "sku", "mounting_face", "panel_front", "panel_back", "nos_version", "bios_version", "firmware_version", "observed_at", "freshness", "support", "is_demo", "row", "position", "power_capacity_kw", "power_used_kw", "port_count", "is_full_depth", "u_height", "fabric_id", "rail", "technology", "network_role", "vlan", "vrf", "vni", "asn", "bgp_peers", "subnet", "oper_state", "admin_state", "mtu", "permanent_mac", "name", "mac", "panel_rear", "source_ref", "fact_layer", "evidence_refs"} {
			if v, ok := p[k]; ok {
				a.Fields[k] = v
			}
		}
		a.Fields["source_ref"] = sourceRef
		index[a.ID] = &a
		all = append(all, &a)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	assetIDs := make([]string, 0, len(all))
	for _, asset := range all {
		assetIDs = append(assetIDs, asset.ID)
	}
	relationArgs := append(scope.Args(), assetIDs)
	rels, e := tx.Query(ctx, `SELECT r.id::text,r.from_uid,r.to_uid,r.definition_id,r.fact_layer,r.source_type,r.source_ref,r.version FROM cmdb_relationship r WHERE r.valid_to IS NULL AND `+RelationshipVisible("r")+` AND r.from_uid=ANY($3::text[]) AND r.to_uid=ANY($3::text[]) ORDER BY r.id LIMIT 20001`, relationArgs...)
	if e != nil {
		return out, e
	}
	defer rels.Close()
	facts := map[string]string{}
	references := map[string][]VisualRelation{}
	sources := map[string]string{}
	for rels.Next() {
		var from, to, kind, fact, source, id, sourceType string
		var revision int64
		if e = rels.Scan(&id, &from, &to, &kind, &fact, &sourceType, &source, &revision); e != nil {
			return out, e
		}
		a, b := index[from], index[to]
		if a == nil || b == nil {
			continue
		}
		if len(out.Relationships) == 20000 {
			out.Truncated = true
			break
		}
		out.Relationships = append(out.Relationships, VisualRelation{id, from, to, kind, fact, sourceType, source, revision})
		fmt.Fprintf(fingerprint, "%s:%d;", id, revision)
		references[from+":"+kind] = append(references[from+":"+kind], out.Relationships[len(out.Relationships)-1])
		switch kind {
		case "parent", "contains":
			b.Fields["parent_uid"] = from
		case "located_in":
			a.Fields["rack_id"] = to
		case "cable_endpoint_a":
			a.Fields["endpoint_a_uid"] = to
			facts[from] = fact
			sources[from] = source
		case "cable_endpoint_b":
			a.Fields["endpoint_b_uid"] = to
			facts[from] = fact
			sources[from] = source
		}
	}
	if e = rels.Err(); e != nil {
		return out, e
	}
	rels.Close()

	// A planned physical fact and an observed claim remain distinct. Avoid
	// silently selecting whichever relation happens to sort last by UUID.
	for _, asset := range all {
		for kind, field := range map[string]string{"located_in": "rack_id", "cable_endpoint_a": "endpoint_a_uid", "cable_endpoint_b": "endpoint_b_uid"} {
			candidates := references[asset.ID+":"+kind]
			if len(candidates) == 0 {
				continue
			}
			asset.Fields[kind+"_sources"] = candidates
			preferred := []VisualRelation{}
			targets := map[string]bool{}
			for _, candidate := range candidates {
				targets[candidate.To] = true
				if candidate.Fact == "planned" {
					preferred = append(preferred, candidate)
				}
			}
			if len(preferred) == 0 {
				preferred = candidates
			}
			chosen := preferred[0]
			ambiguous := false
			for _, candidate := range preferred {
				if candidate.To != chosen.To {
					ambiguous = true
				}
			}
			if ambiguous {
				delete(asset.Fields, field)
				if kind == "located_in" {
					delete(asset.Fields, "rack_uid")
				}
			} else {
				asset.Fields[field] = chosen.To
				if kind == "located_in" {
					asset.Fields["position_fact_layer"] = chosen.Fact
					asset.Fields["position_source_ref"] = chosen.SourceRef
					asset.Fields["position_revision"] = chosen.Revision
				}
				if strings.HasPrefix(kind, "cable_endpoint_") {
					asset.Fields[kind+"_fact_layer"] = chosen.Fact
					facts[asset.ID] = chosen.Fact
					sources[asset.ID] = chosen.SourceRef
				}
			}
			if len(targets) > 1 {
				out.Findings = append(out.Findings, VisualFinding{Asset: asset.ID, Code: "RELATION_ENDPOINT_CONFLICT", Message: "规划与观测端点/位置存在不同引用；已保留分层来源，不能视为确认实装", Severity: "warning", Category: "data_quality"})
			}
		}
	}
	// Only explicit typed hierarchy may supplement stable IDs; never infer identity
	// from display names, ordinal positions, or visual grouping.
	for _, asset := range all {
		if parentID, ok := asset.Fields["parent_uid"].(string); ok {
			if parent := index[parentID]; parent != nil {
				if asset.Type == "rack" && parent.Type == "pod" {
					asset.Fields["pod_id"] = parent.ID
				}
			}
		}
		for _, key := range []string{"parent_uid", "device_id", "rack_id", "rack_uid", "pod_id", "fabric_id"} {
			if id, ok := asset.Fields[key].(string); ok && id != "" && index[id] == nil {
				delete(asset.Fields, key)
			}
		}
	}
	str := func(a *VisualAsset, k string) string {
		if a == nil {
			return ""
		}
		v, _ := a.Fields[k].(string)
		return v
	}
	var device func(string) string
	device = func(id string) string {
		seen := map[string]bool{}
		for n := 0; n < 8; n++ {
			a := index[id]
			if a == nil || seen[id] {
				return ""
			}
			seen[id] = true
			if a.Type == "server" || a.Type == "switch" || a.Type == "patch_panel" || a.Type == "pdu" {
				return id
			}
			next := str(a, "device_id")
			if next == "" {
				next = str(a, "parent_uid")
			}
			id = next
		}
		return ""
	}

	if !out.Truncated {
		members := map[string]map[string]map[string]bool{}
		evidence := map[string]map[string][]string{}
		for _, relation := range out.Relationships {
			if relation.Type != "contains" {
				continue
			}
			child := index[relation.To]
			if child == nil || (child.Type != "gpu" && child.Type != "nic") {
				continue
			}
			owner := device(relation.From)
			if owner == "" {
				continue
			}
			if members[owner] == nil {
				members[owner] = map[string]map[string]bool{}
				evidence[owner] = map[string][]string{}
			}
			if members[owner][child.Type] == nil {
				members[owner][child.Type] = map[string]bool{}
			}
			members[owner][child.Type][child.ID] = true
			evidence[owner][child.Type] = append(evidence[owner][child.Type], relation.ID)
		}
		for owner, kinds := range members {
			for kind, ids := range kinds {
				asset := index[owner]
				field := kind + "_count"
				if asset.Fields[field] == nil {
					asset.Fields[field] = len(ids)
					asset.Fields[field+"_source"] = map[string]any{"source": "derived", "method": "unique_contains_members", "relationship_ids": evidence[owner][kind], "scope": "authorized_registered_components"}
				}
			}
		}
	}
	local := map[string]bool{}
	for _, a := range all {
		r := str(a, "rack_id")
		if r == "" {
			r = str(a, "rack_uid")
			a.Fields["rack_id"] = r
		}
		if rack == "" || r == rack || a.ID == rack {
			local[a.ID] = true
		}
		if a.Type == "server" || a.Type == "switch" {
			if r == "" {
				out.Findings = append(out.Findings, VisualFinding{Asset: a.ID, Code: "MISSING_LOCATION", Message: "设备缺少确认的机柜位置"})
			}
		}
	}
	cables := []VisualCable{}
	occupied := map[string]int{}
	for _, a := range all {
		if a.Type != "cable" {
			continue
		}
		pa, pb := str(a, "endpoint_a_uid"), str(a, "endpoint_b_uid")
		// Payload references are not an authorization grant. Never return hidden endpoints.
		if endpoint := index[pa]; endpoint == nil || endpoint.Type != "port" {
			pa = ""
			delete(a.Fields, "endpoint_a_uid")
		}
		if endpoint := index[pb]; endpoint == nil || endpoint.Type != "port" {
			pb = ""
			delete(a.Fields, "endpoint_b_uid")
		}
		da, db := device(pa), device(pb)
		fact := facts[a.ID]
		fa, fb := str(a, "cable_endpoint_a_fact_layer"), str(a, "cable_endpoint_b_fact_layer")
		if fa != "" && fb != "" && fa != fb {
			fact = "mixed"
		}
		if fact == "" {
			fact = "recorded"
		}
		source := str(a, "source_ref")
		if source == "" {
			source = sources[a.ID]
		}
		c := VisualCable{a.ID, pa, pb, da, db, fact, source, a.Fields}
		if da == "" || db == "" {
			out.Findings = append(out.Findings, VisualFinding{Asset: a.ID, Code: "MISSING_ENDPOINT", Message: "线缆缺少有效双端设备或端口"})
		}
		occupied[pa]++
		occupied[pb]++
		if rack == "" || local[da] || local[db] {
			cables = append(cables, c)
		}
	}
	// Local expansion is by stable identifiers; port tracing never follows logical membership.
	if focus != "" {
		seen := map[string]bool{focus: true}
		if a := index[focus]; a != nil && a.Type == "rack" {
			for _, d := range all {
				if str(d, "rack_id") == focus {
					seen[d.ID] = true
				}
			}
		}
		for i := 0; i < 2; i++ {
			previous := map[string]bool{}
			for id, v := range seen {
				previous[id] = v
			}
			for _, c := range cables {
				if previous[c.ID] || previous[c.A] || previous[c.B] || previous[c.DeviceA] || previous[c.DeviceB] {
					seen[c.ID] = true
					seen[c.A] = true
					seen[c.B] = true
					seen[c.DeviceA] = true
					seen[c.DeviceB] = true
				}
			}
		}
		filtered := []VisualCable{}
		for _, c := range cables {
			if seen[c.ID] {
				filtered = append(filtered, c)
			}
		}
		cables = filtered
		local = seen
	}
	for _, c := range cables {
		local[c.A] = true
		local[c.B] = true
		local[c.DeviceA] = true
		local[c.DeviceB] = true
	}
	for port, n := range occupied {
		if port != "" && n > 1 {
			out.Findings = append(out.Findings, VisualFinding{Asset: port, Code: "PORT_OCCUPIED", Message: "物理端口被多条线缆占用，需核查中间跳接"})
		}
	}
	for _, a := range all {
		owner := device(a.ID)
		if owner != "" {
			a.Fields["owner_device_id"] = owner
		}
		if rack != "" && owner != "" && str(index[owner], "rack_id") == rack {
			local[a.ID] = true
		}
		if !local[a.ID] {
			continue
		}
		if !full && rack == "" && focus == "" && a.Type != "rack" && a.Type != "server" && a.Type != "switch" && a.Type != "data_center" && a.Type != "pod" && a.Type != "region" {
			continue
		}
		out.Assets = append(out.Assets, *a)
	}

	connected := map[string]bool{}
	portCounts := map[string]int{}
	ratePattern := regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*(G|M|T)(?:BPS|BIT/S|B/S)?\s*$`)
	rate := func(a *VisualAsset) float64 {
		if a == nil {
			return 0
		}
		m := ratePattern.FindStringSubmatch(fmt.Sprint(a.Fields["speed"]))
		if len(m) == 0 {
			return 0
		}
		n, _ := strconv.ParseFloat(m[1], 64)
		switch strings.ToUpper(m[2]) {
		case "M":
			return n
		case "G":
			return n * 1000
		case "T":
			return n * 1000000
		}
		return 0
	}
	portNames := map[string]string{}
	for _, a := range all {
		if a.Type == "port" {
			owner := device(a.ID)
			portCounts[owner]++
			physicalName := strings.TrimSpace(str(a, "physical_name"))
			if owner != "" && physicalName != "" {
				key := owner + "\x00" + physicalName
				if previous := portNames[key]; previous != "" && previous != a.ID {
					out.Findings = append(out.Findings, VisualFinding{Asset: a.ID, Code: "PORT_NAME_DUPLICATE", Message: "同一设备的物理端口名称存在多个资产 ID：" + previous, Severity: "warning", Category: "data_quality"})
				}
				portNames[key] = a.ID
			}
		}
	}
	for _, c := range cables {
		connected[c.DeviceA] = true
		connected[c.DeviceB] = true
		a, b := rate(index[c.A]), rate(index[c.B])
		if a > 0 && b > 0 && a != b {
			out.Findings = append(out.Findings, VisualFinding{Asset: c.ID, Code: "PORT_SPEED_MISMATCH", Message: "双端已登记速率不同；需核查协商速率及规划，不能自动判硬件故障"})
		}
	}
	for _, a := range all {
		if a.Type != "server" && a.Type != "switch" {
			continue
		}
		if rack != "" && str(a, "rack_id") != rack {
			continue
		}
		if portCounts[a.ID] == 0 {
			out.Findings = append(out.Findings, VisualFinding{Asset: a.ID, Code: "PORT_RELATION_MISSING", Message: "设备没有已登记的端口归属，数据未完整"})
		}
		if !connected[a.ID] {
			out.Findings = append(out.Findings, VisualFinding{Asset: a.ID, Code: "PHYSICAL_RELATION_MISSING", Message: "当前范围没有已登记物理连接，不代表实际断网"})
		}
		if a.Fields["panel_front"] == nil || a.Fields["panel_back"] == nil {
			out.Findings = append(out.Findings, VisualFinding{Asset: a.ID, Code: "PANEL_UNCOLLECTED", Message: "精确正反面板数据未采集，使用通用矢量示意"})
		}
	}
	// Validate the confirmed rack facts independently from the drawing engine.
	occupiedU := map[string]map[int]string{}
	numeric := func(v any) int {
		n, _ := strconv.ParseFloat(fmt.Sprint(v), 64)
		if math.Trunc(n) != n {
			return 0
		}
		return int(n)
	}
	for _, a := range all {
		if a.Type != "server" && a.Type != "switch" {
			continue
		}
		r := str(a, "rack_id")
		if rack != "" && r != rack {
			continue
		}
		u := numeric(a.Fields["u_start"])
		if u == 0 {
			u = numeric(a.Fields["rack_u"])
		}
		h := numeric(a.Fields["u_height"])
		rackAsset := index[r]
		if rackAsset == nil {
			continue
		}
		capacity := numeric(rackAsset.Fields["u_height"])
		if u < 1 || h < 1 {
			out.Findings = append(out.Findings, VisualFinding{Asset: a.ID, Code: "MISSING_U", Message: "设备 U 位或高度未确认"})
			continue
		}
		if u+h-1 > capacity || h > 1000 {
			out.Findings = append(out.Findings, VisualFinding{Asset: a.ID, Code: "U_RANGE", Message: "设备超出机柜范围"})
			continue
		}
		if occupiedU[r] == nil {
			occupiedU[r] = map[int]string{}
		}
		for pos := u; pos < u+h; pos++ {
			if previous := occupiedU[r][pos]; previous != "" {
				out.Findings = append(out.Findings, VisualFinding{Asset: a.ID, Code: "U_OVERLAP", Message: "U 位与 " + previous + " 重叠"})
			}
			occupiedU[r][pos] = a.ID
		}
	}

	sort.Slice(cables, func(i, j int) bool { return cables[i].ID < cables[j].ID })
	out.Cables = cables
	// Keep relationship results restricted to the returned asset set.
	visible := map[string]bool{}
	for _, a := range out.Assets {
		visible[a.ID] = true
	}
	filteredRelations := []VisualRelation{}
	for _, relation := range out.Relationships {
		if visible[relation.From] && visible[relation.To] {
			filteredRelations = append(filteredRelations, relation)
		}
	}
	out.Relationships = filteredRelations
	filteredFindings := []VisualFinding{}
	for _, finding := range out.Findings {
		if finding.Severity == "" {
			finding.Severity = "warning"
		}
		if finding.Category == "" {
			finding.Category = "data_quality"
		}
		if visible[finding.Asset] {
			filteredFindings = append(filteredFindings, finding)
		}
	}
	out.Findings = filteredFindings
	out.Coverage = VisualCoverage{len(out.Assets), out.Total, limit, !out.Truncated}
	fmt.Fprintf(fingerprint, "%s|%s|%v|%v|%t|%d", rack, focus, scope.Projects, scope.Pairs, full, limit)
	out.SnapshotID = hex.EncodeToString(fingerprint.Sum(nil))[:24]
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	return out, nil
}

// A relocation can only be published through the existing approved ChangeSet.
// Locking the rack serializes competing changes to its physical U space.
func (s Service) validateRelocation(ctx context.Context, tx pgx.Tx, project, id string, current, change map[string]any) error {
	number := func(v any) float64 { n, _ := strconv.ParseFloat(fmt.Sprint(v), 64); return n }
	start := number(change["u_start"])
	height := number(current["u_height"])
	if len(change) != 1 || start < 1 || height < 1 || math.Trunc(start) != start || math.Trunc(height) != height {
		return Reject("RACK_POSITION_INVALID", 422)
	}
	var raw []byte
	e := tx.QueryRow(ctx, `SELECT r.payload FROM cmdb_entity r JOIN cmdb_asset_scope sc ON sc.asset_id=r.item_uid WHERE sc.project_id=$1 AND r.asset_type='rack' AND (r.item_uid=COALESCE($2::jsonb->>'rack_id',$2::jsonb->>'rack_uid') OR EXISTS(SELECT 1 FROM cmdb_relationship rel WHERE rel.from_uid=$3 AND rel.to_uid=r.item_uid AND rel.definition_id='located_in' AND rel.project_id=$1 AND rel.valid_to IS NULL)) FOR UPDATE OF r`, project, current, id).Scan(&raw)
	if e != nil {
		return Reject("RACK_LOCATION_UNCONFIRMED", 422)
	}
	var rack map[string]any
	json.Unmarshal(raw, &rack)
	for _, dimension := range []string{"width_mm", "depth_mm"} {
		if number(rack[dimension]) > 0 && number(current[dimension]) > number(rack[dimension]) {
			return Reject("RACK_DIMENSION_CONFLICT", 422)
		}
	}
	if start+height-1 > number(rack["u_height"]) {
		return Reject("RACK_U_OUT_OF_RANGE", 422)
	}
	rows, e := tx.Query(ctx, `SELECT e.payload FROM cmdb_entity e JOIN cmdb_asset_scope sc ON sc.asset_id=e.item_uid WHERE sc.project_id=$1 AND e.item_uid<>$2 AND e.asset_type IN('server','switch','patch_panel','pdu') AND (COALESCE(e.payload->>'rack_id',e.payload->>'rack_uid')=COALESCE($3::jsonb->>'rack_id',$3::jsonb->>'rack_uid') OR EXISTS(SELECT 1 FROM cmdb_relationship a JOIN cmdb_relationship b ON a.to_uid=b.to_uid WHERE a.from_uid=$2 AND b.from_uid=e.item_uid AND a.definition_id='located_in' AND b.definition_id='located_in' AND a.valid_to IS NULL AND b.valid_to IS NULL AND a.project_id=$1 AND b.project_id=$1))`, project, id, current)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		if e = rows.Scan(&raw); e != nil {
			return e
		}
		var p map[string]any
		json.Unmarshal(raw, &p)
		u := number(p["u_start"])
		if u == 0 {
			u = number(p["rack_u"])
		}
		h := number(p["u_height"])
		if u > 0 && h > 0 && start < u+h && u < start+height {
			return Reject("RACK_U_CONFLICT", 422)
		}
	}
	return rows.Err()
}
