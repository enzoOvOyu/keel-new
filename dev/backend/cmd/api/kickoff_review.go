package main

import (
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/kickoff"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type kickoffReviewSignal struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Severity string `json:"severity"`
	Impact   string `json:"impact"`
	Action   string `json:"action"`
}
type kickoffReviewFacts struct {
	Type            string `json:"type"`
	Model           string `json:"model"`
	Serial          string `json:"serial"`
	OwnerID         string `json:"ownerId"`
	OwnerName       string `json:"ownerName"`
	RackID          string `json:"rackId"`
	RackName        string `json:"rackName"`
	Room            string `json:"room"`
	StartU          *int   `json:"startU"`
	HeightU         *int   `json:"heightU"`
	CapacityU       *int   `json:"capacityU"`
	MAC             string `json:"mac"`
	Speed           string `json:"speed"`
	Role            string `json:"role"`
	LocalPort       string `json:"localPort"`
	PeerName        string `json:"peerName"`
	PeerPort        string `json:"peerPort"`
	Medium          string `json:"medium"`
	Slot            string `json:"slot"`
	PortCount       int    `json:"portCount"`
	ConnectionCount int    `json:"connectionCount"`
	ComponentCount  int    `json:"componentCount"`
}
type kickoffReviewRow struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Reference   string                `json:"reference"`
	Location    string                `json:"location"`
	State       string                `json:"state"`
	Evidence    kickoff.Evidence      `json:"evidence"`
	Facts       kickoffReviewFacts    `json:"facts"`
	Signals     []kickoffReviewSignal `json:"signals"`
}
type kickoffReviewOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// A read-only projection: readiness of geometry is independent of asset publication.
func kickoffReviewRows(view kickoff.View, pubs []kickoff.Publication, category string) []kickoffReviewRow {
	blocked, published := map[string]bool{}, map[string]bool{}
	for _, i := range view.Issues {
		if !i.Resolved && i.Scope == "identity" {
			blocked[i.EntityID] = true
		}
	}
	for _, p := range pubs {
		if p.State == "published" {
			for _, ids := range [][]string{p.DeviceIDs, p.PortIDs, p.ComponentIDs, p.ConnectionIDs} {
				for _, id := range ids {
					published[id] = true
				}
			}
		}
	}
	state := func(id string) string {
		if blocked[id] {
			return "blocked"
		}
		if published[id] {
			return "published"
		}
		return "ready"
	}
	devices, ports, racks := map[string]kickoff.Device{}, map[string]kickoff.Port{}, map[string]kickoff.Rack{}
	portCounts, componentCounts, linkCounts, degree, macCounts := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	for _, d := range view.Devices {
		devices[d.ID] = d
	}
	for _, r := range view.Racks {
		racks[r.ID] = r
	}
	for _, p := range view.Ports {
		ports[p.ID] = p
		portCounts[p.DeviceID]++
		if p.MAC != "" {
			macCounts[strings.ToLower(p.MAC)]++
		}
	}
	for _, c := range view.Components {
		componentCounts[c.DeviceID]++
	}
	for _, c := range view.Connections {
		degree[c.A]++
		degree[c.B]++
		a, b := ports[c.A], ports[c.B]
		if a.ID != "" {
			linkCounts[a.DeviceID]++
		}
		if b.ID != "" && b.DeviceID != a.DeviceID {
			linkCounts[b.DeviceID]++
		}
	}
	facts := func(d kickoff.Device) kickoffReviewFacts {
		r := racks[d.RackID]
		return kickoffReviewFacts{Type: d.Type, Model: d.Model, Serial: d.Serial, RackID: d.RackID, RackName: r.Name, Room: r.Room, StartU: d.StartU, HeightU: d.HeightU, CapacityU: r.CapacityU, Role: d.Role, PortCount: portCounts[d.ID], ConnectionCount: linkCounts[d.ID], ComponentCount: componentCounts[d.ID]}
	}
	add := func(row *kickoffReviewRow, code, label, severity, impact, action string) {
		row.Signals = append(row.Signals, kickoffReviewSignal{code, label, severity, impact, action})
	}
	placement := func(row *kickoffReviewRow, d kickoff.Device) {
		if racks[d.RackID].ID == "" {
			add(row, "unplaced", "机柜未关联", "warning", "可先入库；无法归入机柜图", "models/facility")
		}
		if d.StartU == nil {
			add(row, "unplaced", "缺 U 位", "warning", "可先入库；无法定位柜内位置", "models/facility")
		} else if *d.StartU < 1 {
			add(row, "geometry", "U 位无效", "warning", "无法绘制有效占位", "models/facility")
		}
		if d.HeightU == nil {
			add(row, "geometry", "缺设备高度", "warning", "可先入库；无法确认完整占位", "models/facility")
		} else if *d.HeightU < 1 {
			add(row, "geometry", "设备高度无效", "warning", "无法绘制有效占位", "models/facility")
		}
		r := racks[d.RackID]
		if r.ID != "" && r.CapacityU == nil {
			add(row, "geometry", "缺机柜容量", "warning", "无法检查 U 位是否越界", "models/facility")
		}
		if r.CapacityU != nil && d.StartU != nil && d.HeightU != nil && *d.StartU+*d.HeightU-1 > *r.CapacityU {
			add(row, "geometry", "占位超出机柜", "warning", "落位需修正；不影响设备身份入库", "models/facility")
		}
	}
	overlap := map[string]bool{}
	groups := map[string][]kickoff.Device{}
	for _, d := range view.Devices {
		if d.RackID != "" && d.StartU != nil && d.HeightU != nil && *d.HeightU > 0 {
			groups[d.RackID] = append(groups[d.RackID], d)
		}
	}
	for _, ds := range groups {
		sort.Slice(ds, func(i, j int) bool { return *ds[i].StartU < *ds[j].StartU })
		for i, d := range ds {
			for j := i + 1; j < len(ds) && *ds[j].StartU < *d.StartU+*d.HeightU; j++ {
				overlap[d.ID] = true
				overlap[ds[j].ID] = true
			}
		}
	}
	rows := []kickoffReviewRow{}
	switch category {
	case "devices", "placements":
		for _, d := range view.Devices {
			v := kickoffReviewRow{ID: d.ID, Name: d.Name, Description: d.Model, Reference: d.Serial, Location: racks[d.RackID].Name, State: state(d.ID), Evidence: d.Evidence, Facts: facts(d), Signals: []kickoffReviewSignal{}}
			placement(&v, d)
			if overlap[d.ID] {
				add(&v, "geometry", "U 位重叠", "warning", "同柜设备占位冲突，需核对位置", "models/facility")
			}
			if d.Model == "" {
				add(&v, "model_missing", "型号待补", "warning", "无法关联型号规格", "models/facility")
			}
			if d.Type == "device" || d.Type == "" {
				add(&v, "model_missing", "类型待补", "warning", "无法区分服务器或交换机", "models/facility")
			}
			if blocked[d.ID] {
				add(&v, "blocked", "身份冲突", "block", "阻断此设备入库", "review?category=devices&filter=blocked")
			}
			rows = append(rows, v)
		}
	case "components":
		for _, c := range view.Components {
			d := devices[c.DeviceID]
			f := facts(d)
			f.OwnerID = c.DeviceID
			f.OwnerName = d.Name
			f.Model = c.Model
			f.Type = c.Type
			f.Slot = c.Slot
			f.Serial = ""
			v := kickoffReviewRow{ID: c.ID, Name: c.Name, Description: c.Model, Reference: d.Name, Location: c.Slot, State: state(c.ID), Evidence: c.Evidence, Facts: f, Signals: []kickoffReviewSignal{}}
			if d.ID == "" {
				v.State = "blocked"
				add(&v, "blocked", "父设备未关联", "block", "组件无法随设备入库", "review?category=devices")
			}
			if c.Model == "" {
				add(&v, "model_missing", "型号待补", "warning", "组件规格不完整", "models/facility")
			}
			rows = append(rows, v)
		}
	case "ports":
		for _, p := range view.Ports {
			d := devices[p.DeviceID]
			f := facts(d)
			f.OwnerID = p.DeviceID
			f.OwnerName = d.Name
			f.MAC = p.MAC
			f.Speed = p.Speed
			f.Role = p.Role
			f.ConnectionCount = degree[p.ID]
			v := kickoffReviewRow{ID: p.ID, Name: p.Name, Description: p.Speed, Reference: d.Name, Location: p.MAC, State: state(p.ID), Evidence: p.Evidence, Facts: f, Signals: []kickoffReviewSignal{}}
			if d.ID == "" {
				v.State = "blocked"
				add(&v, "blocked", "所属设备缺失", "block", "无法确认端口归属", "review?category=devices")
			}
			if degree[p.ID] == 0 {
				add(&v, "unconnected", "未关联连接", "info", "尚无规划连接；不代表实际链路断开", "review?category=connections")
			}
			if degree[p.ID] > 1 {
				add(&v, "occupied", "端口多次引用", "warning", "需核对重复连接与端口建模", "review?category=connections")
			}
			if p.MAC != "" && macCounts[strings.ToLower(p.MAC)] > 1 {
				add(&v, "occupied", "MAC 重复", "warning", "需核对端口身份，不自动合并", "review?category=ports&filter=occupied")
			}
			if p.Speed == "" {
				add(&v, "spec_missing", "速率未提供", "info", "无法检查端口规格匹配", "models/network")
			}
			rows = append(rows, v)
		}
	case "connections":
		for _, c := range view.Connections {
			a, b := ports[c.A], ports[c.B]
			da, db := devices[a.DeviceID], devices[b.DeviceID]
			f := facts(da)
			f.OwnerID = da.ID
			f.OwnerName = da.Name
			f.LocalPort = a.Name
			f.PeerName = db.Name
			f.PeerPort = b.Name
			f.Medium = c.Medium
			v := kickoffReviewRow{ID: c.ID, Name: da.Name, Description: a.Name, Reference: db.Name + " / " + b.Name, Location: c.Medium, State: "ready", Evidence: c.Evidence, Facts: f, Signals: []kickoffReviewSignal{}}
			if a.ID == "" || b.ID == "" || da.ID == "" || db.ID == "" {
				v.State = "blocked"
				add(&v, "endpoint_missing", "连接端点缺失", "block", "阻断此连接入库", "review?category=ports")
			}
			if blocked[a.DeviceID] || blocked[b.DeviceID] {
				v.State = "blocked"
				add(&v, "blocked", "端点设备身份冲突", "block", "阻断受影响连接入库", "review?category=devices&filter=blocked")
			}
			if v.State != "blocked" && published[c.ID] {
				v.State = "published"
			}
			if degree[c.A] > 1 || degree[c.B] > 1 {
				add(&v, "occupied", "端口多次引用", "warning", "需核对端口占用与重复连接", "review?category=ports&filter=occupied")
			}
			if da.ID != "" && db.ID != "" && (racks[da.RackID].ID == "" || racks[db.RackID].ID == "") {
				add(&v, "unplaced", "端点机柜未关联", "warning", "可先建立连接；无法计算柜间路径", "models/facility")
			}
			rows = append(rows, v)
		}
	}
	for i := range rows {
		row := &rows[i]
		if blocked[row.Facts.OwnerID] {
			row.State = "blocked"
			add(row, "blocked", "所属设备身份冲突", "block", "影响该对象入库", "review?category=devices&filter=blocked")
		}
		sort.SliceStable(row.Signals, func(i, j int) bool {
			rank := map[string]int{"block": 0, "warning": 1, "info": 2}
			return rank[row.Signals[i].Severity] < rank[row.Signals[j].Severity]
		})
	}
	return rows
}
func reviewMatches(v kickoffReviewRow, filter string) bool {
	switch filter {
	case "", "all":
		return true
	case "ready", "published":
		return v.State == filter
	case "blocked":
		return v.State == "blocked"
	case "issues":
		return len(v.Signals) > 0 || v.State == "blocked"
	}
	for _, s := range v.Signals {
		if s.Code == filter {
			return true
		}
	}
	return false
}
func kickoffReviewPage(w http.ResponseWriter, r *http.Request, view kickoff.View, pubs []kickoff.Publication, revision int) {
	q := r.URL.Query()
	category := q.Get("category")
	if category == "" {
		category = "devices"
	}
	switch category {
	case "devices", "placements", "components", "ports", "connections":
	default:
		fail(w, 400, "KICKOFF_CATEGORY_INVALID")
		return
	}
	offset, limit := 0, 10
	var err error
	if q.Has("offset") {
		offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil || offset < 0 {
			fail(w, 400, "KICKOFF_PAGE_INVALID")
			return
		}
	}
	if q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 200 {
			fail(w, 400, "KICKOFF_PAGE_INVALID")
			return
		}
	}
	filter := q.Get("filter")
	if filter == "" && q.Get("issues") == "true" {
		filter = "issues"
	}
	switch filter {
	case "", "all", "issues", "blocked", "unplaced", "geometry", "model_missing", "ready", "published", "unconnected", "occupied", "endpoint_missing", "spec_missing":
	default:
		fail(w, 400, "KICKOFF_FILTER_INVALID")
		return
	}
	rows := kickoffReviewRows(view, pubs, category)
	filtered := []kickoffReviewRow{}
	search := strings.ToLower(q.Get("q"))
	counts := map[string]int{}
	facetValues := map[string]map[string]string{"type": {}, "model": {}, "rack": {}, "source": {}}
	for _, v := range rows {
		for key, value := range map[string]string{"type": v.Facts.Type, "model": v.Facts.Model, "rack": v.Facts.RackID, "source": v.Evidence.SourceID} {
			if value != "" {
				label := value
				if key == "rack" && v.Facts.RackName != "" {
					label = v.Facts.RackName
				}
				facetValues[key][value] = label
			}
		}
		if q.Get("type") != "" && v.Facts.Type != q.Get("type") || q.Get("model") != "" && v.Facts.Model != q.Get("model") || q.Get("rack") != "" && v.Facts.RackID != q.Get("rack") || q.Get("source") != "" && v.Evidence.SourceID != q.Get("source") {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(strings.Join([]string{v.Name, v.Description, v.Reference, v.Location, v.Facts.Serial, v.Facts.MAC, v.Facts.RackName, v.Facts.Room, v.Facts.Type, v.Facts.Role, v.Facts.PeerName, v.Facts.LocalPort, v.Facts.PeerPort}, " ")), search) {
			continue
		}
		for _, key := range []string{"all", "issues", "blocked", "unplaced", "geometry", "model_missing", "ready", "published", "unconnected", "occupied", "endpoint_missing", "spec_missing"} {
			if reviewMatches(v, key) {
				counts[key]++
			}
		}
		if reviewMatches(v, filter) {
			filtered = append(filtered, v)
		}
	}
	field := q.Get("sort")
	value := func(v kickoffReviewRow) string {
		switch field {
		case "reference":
			return v.Reference
		case "location":
			return v.Location
		case "state":
			return v.State
		case "model":
			return v.Facts.Model
		case "speed":
			return v.Facts.Speed
		}
		return v.Name
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := value(filtered[i]), value(filtered[j])
		if a == b {
			return filtered[i].ID < filtered[j].ID
		}
		if q.Get("desc") == "true" {
			return a > b
		}
		return a < b
	})
	facets := map[string][]kickoffReviewOption{}
	for key, values := range facetValues {
		options := []kickoffReviewOption{}
		for value, label := range values {
			options = append(options, kickoffReviewOption{value, label})
		}
		sort.Slice(options, func(i, j int) bool {
			if options[i].Label == options[j].Label {
				return options[i].Value < options[j].Value
			}
			return options[i].Label < options[j].Label
		})
		facets[key] = options
	}
	start := min(offset, len(filtered))
	end := start + min(limit, len(filtered)-start)
	respond(w, 200, map[string]any{"revision": revision, "rows": filtered[start:end], "total": len(filtered), "offset": offset, "limit": limit, "hasMore": end < len(filtered), "counts": counts, "facets": facets})
}

func (s *Server) kickoffPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceIDs        []string `json:"deviceIds"`
		FacilityIDs      []string `json:"facilityIds"`
		ExpectedRevision int      `json:"expectedRevision"`
	}
	if !engineeringDecode(w, r, &body) {
		return
	}
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	var revision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM kickoff_workspace WHERE project_id=$1`, s.project).Scan(&revision); err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	if revision != body.ExpectedRevision {
		fail(w, 409, "REVISION_CONFLICT")
		return
	}
	read, err := s.kickoffReadSnapshot(ctx, tx, revision)
	if err != nil {
		fail(w, 500, "KICKOFF_MODEL_UNAVAILABLE")
		return
	}
	_, receipt, err := s.kickoffSelectPublication(ctx, tx, read.model, read.detail(), body.DeviceIDs, body.FacilityIDs)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	respond(w, 200, map[string]any{"revision": revision, "counts": receipt.Counts})
}
