package bom

import (
	"fmt"
	"strconv"
	"strings"
)

func Speed(value string) (uint64, bool) {
	s := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	units := []struct {
		suffix string
		factor uint64
	}{{"gbps", 1000000000}, {"mbps", 1000000}, {"kbps", 1000}, {"bps", 1}, {"g", 1000000000}, {"m", 1000000}}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			n, e := strconv.ParseUint(strings.TrimSuffix(s, u.suffix), 10, 32)
			return n * u.factor, e == nil && n > 0
		}
	}
	return 0, false
}
func Engineering(items []Item, caps map[string]map[string]any) []Issue {
	out := []Issue{}
	ids := map[string]Item{}
	for _, i := range items {
		ids[i.UID] = i
	}
	add := func(i Item, f, r, msg string) {
		out = append(out, Issue{fmt.Sprintf("eng-%d", len(out)), "BLOCKER", r, "1.0", i.UID, f, msg})
	}
	for _, i := range items {
		if i.Category == "cable" {
			a, b := ids[i.A], ids[i.B]
			for _, side := range []struct {
				Label string
				Port  Item
			}{{"a", a}, {"b", b}} {
				expected := i.Fields["endpoint_"+side.Label+"_device"]
				parent := ids[side.Port.Parent]
				if side.Port.UID != "" && expected != "" && parent.Hostname != expected && parent.Fields["device_alias"] != expected {
					add(i, "endpoint_"+side.Label+"_uid", "ENDPOINT-SOURCE-CONFLICT", "Selected port owner differs from explicitly mapped device reference")
				}
				expectedPort := i.Fields["endpoint_"+side.Label+"_port"]
				if side.Port.UID != "" && expectedPort != "" && side.Port.Port != expectedPort {
					add(i, "endpoint_"+side.Label+"_uid", "ENDPOINT-SOURCE-CONFLICT", "Selected physical port differs from mapped port reference")
				}
			}
			sa, oka := Speed(a.Fields["speed"])
			sb, okb := Speed(b.Fields["speed"])
			if !oka || !okb {
				add(i, "speed", "CABLE-SPEED-EVIDENCE", "Both endpoint speeds require explicit units")
			} else if sa != sb {
				add(i, "speed", "CABLE-SPEED-MISMATCH", "Both endpoint speeds must match an approved compatibility rule")
			}
			if i.Fields["media_type"] == "" {
				add(i, "media_type", "CABLE-MEDIA", "Cable medium must be explicitly mapped")
			}
		}
		if breakout := i.Fields["breakout"]; breakout != "" {
			parent := ids[i.Parent]
			model := caps[parent.Category+":"+parent.Model]
			allowed, ok := model["breakout"].([]any)
			matched := false
			if ok {
				for _, v := range allowed {
					if v == breakout {
						matched = true
					}
				}
			}
			if !matched {
				add(i, "breakout", "MODEL-BREAKOUT", "Missing approved breakout capability or incompatible mode")
			}
		}
		if i.Category == "optic_module" {
			port, ok := ids[i.Fields["port_uid"]]
			if !ok || port.Category != "port" {
				add(i, "port_uid", "OPTIC-PORT", "Optic requires an explicit physical port reference")
			} else {
				a, oa := Speed(i.Fields["speed"])
				b, ob := Speed(port.Fields["speed"])
				if !oa || !ob || a != b {
					add(i, "speed", "OPTIC-SPEED", "Optic and port speed compatibility not proven")
				}
				if i.Fields["optic_type"] == "" || port.Fields["optic_type"] == "" || i.Fields["optic_type"] != port.Fields["optic_type"] {
					add(i, "optic_type", "OPTIC-TYPE", "Explicit compatible optical interface types required")
				}
			}
		}
	}
	return out
}
