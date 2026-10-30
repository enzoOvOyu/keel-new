package bom

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

type Item struct {
	UID      string            `json:"item_uid"`
	Category string            `json:"category"`
	Serial   string            `json:"serial"`
	Hostname string            `json:"hostname"`
	Model    string            `json:"model"`
	Parent   string            `json:"parent_uid"`
	Port     string            `json:"port_index"`
	A        string            `json:"endpoint_a_uid"`
	B        string            `json:"endpoint_b_uid"`
	Fields   map[string]string `json:"fields"`
	Sources  map[string]Cell   `json:"sources"`
	Sheet    string            `json:"sheet_name"`
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
	switch i.Category {
	case "port":
		return "port:" + i.Parent + ":" + i.Port
	case "cable":
		a, b := i.A, i.B
		if a > b {
			a, b = b, a
		}
		return "cable:" + a + ":" + b
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
	for _, i := range items {
		ids[i.UID] = i
	}
	if len(items) == 0 {
		add(Item{}, "items", "BOM-NONEMPTY", "No confirmed candidate items")
	}
	for _, i := range items {
		identity := Identity(i)
		if seen[identity] {
			add(i, "identity", "BOM-IDENTITY", "Duplicate identity requires reconciliation")
		}
		seen[identity] = true
		switch i.Category {
		case "server", "switch", "gpu", "nic", "bmc", "psu", "optic_module":
			if i.Serial == "" {
				add(i, "serial", "BOM-SERIAL", "Serial required for stable identity")
			}
			if i.Model == "" {
				add(i, "model", "BOM-MODEL", "Model requires explicit binding")
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
		for k, v := range i.Fields {
			if Sensitive(k) && v != "" {
				add(i, k, "BOM-SECRET", "Sensitive field prohibited")
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
	return out
}
