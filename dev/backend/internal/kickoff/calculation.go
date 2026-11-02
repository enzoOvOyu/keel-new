package kickoff

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"math"
	"sort"
	"strconv"
	"time"
)

type CalculationRow struct {
	ConnectionID string   `json:"connectionId"`
	From         string   `json:"from"`
	To           string   `json:"to"`
	Route        []string `json:"route"`
	PathM        float64  `json:"pathM"`
	LeadM        float64  `json:"leadM"`
	SlackM       float64  `json:"slackM"`
	CalculatedM  float64  `json:"calculatedM"`
	StockM       *float64 `json:"stockM"`
}
type RackPower struct {
	RackID      string   `json:"rackId"`
	LoadKW      *float64 `json:"loadKW"`
	CapacityKW  *float64 `json:"capacityKW"`
	RemainingKW *float64 `json:"remainingKW"`
}
type Calculation struct {
	ID          string           `json:"id"`
	Domain      string           `json:"domain"`
	InputDigest string           `json:"inputDigest"`
	CreatedAt   time.Time        `json:"createdAt"`
	Status      string           `json:"status"`
	Rows        []CalculationRow `json:"rows"`
	Issues      []string         `json:"issues"`
	Power       []RackPower      `json:"power"`
}

func number(v string) (float64, bool) {
	n, e := strconv.ParseFloat(v, 64)
	return n, e == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}
func rounded(n float64) float64 { return math.Round(n*1000) / 1000 }
func Digest(v View, inputs []ModelInput, domain string) string {
	ids := map[string]bool{"placements": true}
	if domain == "cabling" {
		for _, id := range []string{"route_nodes", "route_edges", "rack_access", "cable_specs", "cabling_rules"} {
			ids[id] = true
		}
	} else {
		ids["power_profiles"] = true
		ids["rack_capacity"] = true
	}
	selected := [][]any{}
	for _, m := range inputs {
		if ids[m.TableID] {
			selected = append(selected, []any{m.TableID, m.Version})
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i][0].(string) < selected[j][0].(string) })
	devices := [][]string{}
	for _, d := range v.Devices {
		devices = append(devices, []string{d.ID, d.RackID})
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i][0] < devices[j][0] })
	links := [][]string{}
	if domain == "cabling" {
		for _, c := range v.Connections {
			links = append(links, []string{c.ID, c.A, c.B, c.Medium})
		}
		sort.Slice(links, func(i, j int) bool { return links[i][0] < links[j][0] })
	}
	raw, _ := json.Marshal([]any{selected, devices, links})
	return string(raw)
}
func ShortestPath(edges []map[string]string, from, to string) (float64, []string, bool) {
	distance := map[string]float64{from: 0}
	paths := map[string][]string{from: {from}}
	visited := map[string]bool{}
	for {
		next := ""
		cost := math.Inf(1)
		for node, d := range distance {
			if !visited[node] && (d < cost || d == cost && (next == "" || node < next)) {
				next = node
				cost = d
			}
		}
		if next == "" {
			return 0, nil, false
		}
		if next == to {
			return cost, paths[next], true
		}
		visited[next] = true
		for _, edge := range edges {
			other := ""
			if edge["from"] == next {
				other = edge["to"]
			} else if edge["to"] == next {
				other = edge["from"]
			}
			length, ok := number(edge["distance_m"])
			if other == "" || !ok || length <= 0 {
				continue
			}
			old, found := distance[other]
			if !found || cost+length < old {
				distance[other] = cost + length
				paths[other] = append(append([]string{}, paths[next]...), other)
			}
		}
	}
}
func Calculate(v View, inputs []ModelInput, domain string) Calculation {
	result := Calculation{ID: uuid.NewString(), Domain: domain, InputDigest: Digest(v, inputs, domain), CreatedAt: time.Now().UTC(), Status: "needs_input", Rows: []CalculationRow{}, Issues: []string{}, Power: []RackPower{}}
	if domain == "power" {
		for _, rack := range v.Racks {
			members := []Device{}
			for _, d := range v.Devices {
				if d.RackID == rack.ID {
					members = append(members, d)
				}
			}
			if len(members) == 0 {
				continue
			}
			row := RackPower{RackID: rack.ID}
			missing := 0
			load := 0.0
			for _, d := range members {
				found := false
				for _, p := range Rows(inputs, "power_profiles") {
					if p["device_id"] == d.ID && p["basis"] == "planned" {
						if n, ok := number(p["power_kw"]); ok && n >= 0 {
							load += n
							found = true
						}
						break
					}
				}
				if !found {
					missing++
				}
			}
			if missing == 0 {
				n := rounded(load)
				row.LoadKW = &n
			} else {
				result.Issues = append(result.Issues, fmt.Sprintf("%s：%d 台设备缺少规划功率", rack.Name, missing))
			}
			for _, p := range Rows(inputs, "rack_capacity") {
				if p["rack_id"] == rack.ID {
					if n, ok := number(p["capacity_kw"]); ok && n > 0 {
						row.CapacityKW = &n
					}
					break
				}
			}
			if row.CapacityKW == nil {
				result.Issues = append(result.Issues, rack.Name+"：容量待补充")
			}
			if row.LoadKW != nil && row.CapacityKW != nil {
				n := rounded(*row.CapacityKW - *row.LoadKW)
				row.RemainingKW = &n
				if n < 0 {
					result.Issues = append(result.Issues, rack.Name+"：规划负荷超过容量")
				}
			}
			result.Power = append(result.Power, row)
		}
		if len(result.Power) > 0 {
			result.Status = "ready"
			if len(result.Issues) > 0 {
				result.Status = "partial"
			}
		}
		return result
	}
	rules := Rows(inputs, "cabling_rules")
	if len(rules) == 0 {
		result.Issues = append(result.Issues, "先明确路径规则、规则依据、线缆规格及余量参数。")
		return result
	}
	rule := rules[0]
	slack, ok := number(rule["slack_m"])
	percent, okPercent := number(rule["slack_percent"])
	if rule["code"] == "" || rule["version"] == "" || rule["strategy"] != "shortest_path" || rule["source"] == "" || rule["cable_spec"] == "" || !ok || !okPercent || slack < 0 || percent < 0 {
		result.Issues = append(result.Issues, "先明确路径规则、规则依据、线缆规格及余量参数。")
		return result
	}
	nodes := map[string]bool{}
	for _, r := range Rows(inputs, "route_nodes") {
		nodes[r["code"]] = true
	}
	edges := []map[string]string{}
	for _, e := range Rows(inputs, "route_edges") {
		if nodes[e["from"]] && nodes[e["to"]] {
			edges = append(edges, e)
		}
	}
	access := map[string]map[string]string{}
	for _, r := range Rows(inputs, "rack_access") {
		access[r["rack_id"]] = r
	}
	ports := map[string]Port{}
	for _, p := range v.Ports {
		ports[p.ID] = p
	}
	devices := map[string]Device{}
	for _, d := range v.Devices {
		devices[d.ID] = d
	}
	missing := map[string]bool{}
	for _, link := range v.Connections {
		a, foundA := devices[ports[link.A].DeviceID]
		b, foundB := devices[ports[link.B].DeviceID]
		if !foundA || !foundB {
			result.Issues = append(result.Issues, link.ID+"：端点设备缺失")
			continue
		}
		start, end := access[a.RackID], access[b.RackID]
		la, okA := number(start["lead_m"])
		lb, okB := number(end["lead_m"])
		if !nodes[start["node"]] || !okA || la < 0 {
			missing[a.RackID] = true
		}
		if !nodes[end["node"]] || !okB || lb < 0 {
			missing[b.RackID] = true
		}
		if missing[a.RackID] || missing[b.RackID] {
			continue
		}
		distance, path, found := ShortestPath(edges, start["node"], end["node"])
		if !found {
			result.Issues = append(result.Issues, a.Name+" → "+b.Name+"：没有允许的连通路径")
			continue
		}
		lead := la + lb
		extra := rounded(slack + (distance+lead)*percent/100)
		length := rounded(distance + lead + extra)
		var stock *float64
		for _, s := range Rows(inputs, "cable_specs") {
			n, ok := number(s["length_m"])
			if s["code"] == rule["cable_spec"] && s["medium"] != "" && s["medium"] == link.Medium && ok && n >= length && (stock == nil || n < *stock) {
				copy := n
				stock = &copy
			}
		}
		result.Rows = append(result.Rows, CalculationRow{ConnectionID: link.ID, From: a.Name, To: b.Name, Route: path, PathM: rounded(distance), LeadM: lead, SlackM: extra, CalculatedM: length, StockM: stock})
		if stock == nil {
			result.Issues = append(result.Issues, fmt.Sprintf("%s：没有满足 %gm 的同规格标准长度", link.ID, length))
		}
	}
	keys := []string{}
	for k := range missing {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result.Issues = append(result.Issues, k+"：接入节点或引线长度待补充")
	}
	if len(result.Rows) > 0 {
		result.Status = "ready"
		if len(result.Issues) > 0 {
			result.Status = "partial"
		}
	}
	return result
}
