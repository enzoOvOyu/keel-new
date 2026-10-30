package bom

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type ManualCorrection struct {
	SourceFile string `json:"source_file"`
	Actor      string `json:"actor"`
	Reason     string `json:"reason"`
	Before     string `json:"before"`
	After      string `json:"after"`
	At         string `json:"observed_at"`
	Kind       string `json:"source_kind"`
}
type Item struct {
	Corrections map[string]ManualCorrection `json:"manual_corrections,omitempty"`
	Subtype     string                      `json:"sub_type,omitempty"`
	Status      string                      `json:"status"`
	UID         string                      `json:"item_uid"`
	Category    string                      `json:"category"`
	Serial      string                      `json:"serial"`
	Hostname    string                      `json:"hostname"`
	Model       string                      `json:"model"`
	Parent      string                      `json:"parent_uid"`
	Port        string                      `json:"port_index"`
	A           string                      `json:"endpoint_a_uid"`
	B           string                      `json:"endpoint_b_uid"`
	Fields      map[string]string           `json:"fields"`
	Sources     map[string]Cell             `json:"sources"`
	Sheet       string                      `json:"sheet_name"`
}
type Issue struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Rule     string `json:"rule_id"`
	Version  string `json:"version"`
	Item     string `json:"item_uid"`
	Field    string `json:"field"`
	Reason   string `json:"reason"`
}

func Identity(i Item) string {
	if i.Category == "other" && (i.Subtype == "rack" || i.Subtype == "network_baseline") {
		i.Category = i.Subtype
	}
	switch i.Category {
	case "port":
		return "port:" + i.Parent + ":" + i.Port
	case "cable":
		a, b := i.A, i.B
		if a > b {
			a, b = b, a
		}
		return "cable:" + a + ":" + b
	case "rack":
		return "rack:" + i.Fields["site"] + ":" + i.Fields["rack"]
	case "network_baseline":
		return "network_baseline:" + i.Fields["baseline_ref"]
	default:
		return i.Category + ":" + i.Serial
	}
}
func StableID(identity string) string {
	h := sha256.Sum256([]byte("keel-bom-v1:" + identity))
	x := hex.EncodeToString(h[:16])
	return x[:8] + "-" + x[8:12] + "-" + x[12:16] + "-" + x[16:20] + "-" + x[20:32]
}
func Validate(items []Item) []Issue {
	out := []Issue{}
	add := func(i Item, field, rule, reason string) {
		out = append(out, Issue{fmt.Sprint(len(out) + 1), "BLOCKER", rule, "1.0", i.UID, field, reason})
	}
	ids := map[string]Item{}
	seen := map[string]bool{}
	occupied := map[string]bool{}
	locations := map[string]string{}
	addresses := map[string]string{}
	candidateIDs := map[string]bool{}
	for _, i := range items {
		if i.UID == "" || candidateIDs[i.UID] {
			add(i, "item_uid", "BOM-CANDIDATE-ID", "Candidate IDs must be present and unique")
		}
		candidateIDs[i.UID] = true
		ids[i.UID] = i
	}
	if len(items) == 0 {
		add(Item{}, "items", "BOM-NONEMPTY", "No confirmed candidate items")
	}
	for _, i := range items {
		identity := Identity(i)
		identified := i.Serial != "" || (i.Category == "port" && i.Parent != "" && i.Port != "") || (i.Category == "cable" && i.A != "" && i.B != "") || (i.Category == "rack" && i.Fields["rack"] != "")
		if identified && seen[identity] {
			add(i, "identity", "BOM-IDENTITY", "Duplicate identity requires reconciliation")
		}
		if identified {
			seen[identity] = true
		}
		switch i.Category {
		case "server", "switch", "gpu", "nic", "bmc", "psu", "optic_module":
			if i.Serial == "" {
				add(i, "serial", "BOM-SERIAL", "Serial required for stable identity")
			}
			if i.Model == "" {
				add(i, "model", "BOM-MODEL", "Model requires explicit binding")
			}
		case "rack":
			if i.Fields["rack"] == "" || i.Fields["site"] == "" {
				add(i, "rack", "BOM-RACK", "Rack identifier is required")
			}
		case "network_baseline":
			if i.Fields["baseline_ref"] == "" {
				add(i, "baseline_ref", "BOM-BASELINE", "Explicit approved baseline reference required")
			}
		case "port":
			if _, ok := ids[i.Parent]; !ok || i.Port == "" {
				add(i, "parent_uid", "BOM-PARENT", "Port needs valid parent and index")
			}
		case "cable":
			for _, p := range []string{i.A, i.B} {
				if v, ok := ids[p]; !ok || v.Category != "port" {
					add(i, "endpoints", "BOM-CABLE", "Cable endpoint must reference a physical port")
				}
				if occupied[p] {
					add(i, "endpoints", "BOM-PORT-OCCUPIED", "Port already used")
				}
				occupied[p] = true
			}
			if i.A == i.B {
				add(i, "endpoints", "BOM-CABLE-SELF", "Cable endpoints must differ")
			}
		default:
			add(i, "category", "BOM-CATEGORY", "Unconfirmed material category")
		}
		if i.Category == "gpu" || i.Category == "nic" || i.Category == "bmc" || i.Category == "psu" {
			parent, ok := ids[i.Parent]
			if !ok || (parent.Category != "server" && parent.Category != "switch") || (i.Category == "gpu" && parent.Category != "server") {
				add(i, "parent_uid", "BOM-PARENT", "Component requires an explicit compatible device owner")
			}
		}
		if i.Parent != "" {
			parent, ok := ids[i.Parent]
			if !ok || parent.UID == i.UID {
				add(i, "parent_uid", "BOM-PARENT", "Missing or self parent")
			}
		}
		if i.Fields["u_start"] != "" {
			start, e := strconv.Atoi(i.Fields["u_start"])
			height, h := strconv.Atoi(i.Fields["u_height"])
			capacity, c := strconv.Atoi(i.Fields["rack_height"])
			rack := i.Fields["site"] + ":" + i.Fields["rack"]
			if e != nil || h != nil || c != nil || start < 1 || height < 1 || start+height-1 > capacity || rack == "" {
				add(i, "u_start", "BOM-U-RANGE", "Confirmed rack, U height and capacity required")
			} else {
				for u := start; u < start+height; u++ {
					key := rack + ":" + strconv.Itoa(u)
					if other := locations[key]; other != "" && other != i.UID {
						add(i, "u_start", "BOM-U-CONFLICT", "Overlapping rack placement")
					}
					locations[key] = i.UID
				}
			}
		}
		if i.Category == "cable" {
			a, b := ids[i.A], ids[i.B]
			sa, sb := a.Fields["speed"], b.Fields["speed"]
			va, oka := Speed(sa)
			vb, okb := Speed(sb)
			if oka && okb && va != vb {
				add(i, "endpoints", "BOM-SPEED", "Port speeds differ; explicit compatibility evidence required")
			}
		}
		for k, v := range i.Fields {
			if Sensitive(k) && v != "" {
				add(i, k, "BOM-SECRET", "Sensitive field prohibited")
			}
			if (k == "management_ip" || k == "ip" || k == "mac") && v != "" {
				normalized := v
				if k == "mac" {
					if mac, e := net.ParseMAC(v); e == nil {
						normalized = mac.String()
					}
				} else if ip := net.ParseIP(v); ip != nil {
					normalized = ip.String()
				}
				key := k + ":" + normalized
				if owner := addresses[key]; owner != "" && owner != i.UID && ids[owner].Parent != i.UID && i.Parent != owner {
					add(i, k, "BOM-ADDRESS-DUPLICATE", "Address belongs to multiple assets")
				}
				addresses[key] = i.UID
			}
			if k == "mtu" || k == "vlan" || k == "vni" || k == "asn" {
				n, e := strconv.ParseUint(v, 10, 32)
				if e != nil || n == 0 || (k == "vlan" && n > 4094) || (k == "vni" && n > 16777215) {
					add(i, k, "BOM-NETWORK-RANGE", "Invalid numeric network field")
				}
			}
			if strings.Contains(k, "ip") && v != "" && net.ParseIP(v) == nil {
				add(i, k, "BOM-IP", "Invalid IP address")
			}
			if strings.Contains(k, "mac") && v != "" {
				if _, e := net.ParseMAC(v); e != nil {
					add(i, k, "BOM-MAC", "Invalid MAC")
				}
			}
		}
	}
	visited := map[string]int{}
	var cycle func(string) bool
	cycle = func(id string) bool {
		if id == "" {
			return false
		}
		if visited[id] == 1 {
			return true
		}
		if visited[id] == 2 {
			return false
		}
		visited[id] = 1
		found := cycle(ids[id].Parent)
		visited[id] = 2
		return found
	}
	for _, i := range items {
		if cycle(i.UID) {
			add(i, "parent_uid", "BOM-PARENT-CYCLE", "Parent relationships must be acyclic")
		}
	}
	return out
}
