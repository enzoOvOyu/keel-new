package cmdb

import (
	"fmt"
	"math"
	"strconv"
)

// The quality projection uses the same half-U intervals and face semantics as
// the approved SVG layout. Unknown geometry is a gap, never a definite collision.
func visualRackPlacementFindings(all []*VisualAsset, rack string) []VisualFinding {
	index := map[string]*VisualAsset{}
	for _, a := range all {
		index[a.ID] = a
	}
	type placement struct {
		a          *VisualAsset
		rack, face string
		u, h       float64
		depth      *float64
		full       *bool
	}
	positions := []placement{}
	findings := []VisualFinding{}
	add := func(id, code, message string) {
		findings = append(findings, VisualFinding{Asset: id, Code: code, Message: message, Severity: "warning", Category: "data_quality"})
	}
	for _, a := range all {
		switch a.Type {
		case "server", "switch", "patch_panel", "pdu", "device":
		default:
			continue
		}
		r, _ := a.Fields["rack_id"].(string)
		if rack != "" && rack != r {
			continue
		}
		parent := index[r]
		if parent == nil {
			continue
		}
		raw := a.Fields["u_start"]
		if raw == nil {
			raw = a.Fields["rack_u"]
		}
		u, h := visualDimension(raw), visualDimension(a.Fields["u_height"])
		if h != nil && *h == 0 {
			if u != nil {
				add(a.ID, "ZERO_U_POSITION", "0U 设备不分配竖向 U 位，挂装位置需独立核查")
			}
			continue
		}
		if u == nil || h == nil || *u < 1 || *h <= 0 || math.Trunc(*u*2) != *u*2 || math.Trunc(*h*2) != *h*2 {
			add(a.ID, "MISSING_U", "设备 U 位或高度未确认；支持 0.5U 精度")
			continue
		}
		capacity := visualDimension(parent.Fields["u_height"])
		if capacity == nil || *capacity <= 0 {
			add(a.ID, "MISSING_RACK_HEIGHT", "机柜总 U 高未确认，不能判断安装范围")
		} else if *u+*h-1 > *capacity {
			add(a.ID, "U_RANGE", "设备超出已登记机柜范围")
			continue
		}
		face, _ := a.Fields["mounting_face"].(string)
		if face == "" {
			face, _ = a.Fields["face"].(string)
		}
		if face == "back" {
			face = "rear"
		}
		if face != "front" && face != "rear" && face != "both" {
			face = "unknown"
		}
		var full *bool
		if b, ok := a.Fields["is_full_depth"].(bool); ok {
			full = &b
		}
		depth := visualDimension(a.Fields["depth_mm"])
		rackDepth := visualDimension(parent.Fields["depth_mm"])
		if depth != nil && rackDepth != nil && *depth > *rackDepth {
			add(a.ID, "DIMENSION_CONFLICT", "设备深度超出已登记机柜深度")
		}
		positions = append(positions, placement{a, r, face, *u, *h, depth, full})
	}
	for i, a := range positions {
		for _, b := range positions[i+1:] {
			if a.rack != b.rack || a.u >= b.u+b.h || b.u >= a.u+a.h {
				continue
			}
			opposite := a.face == "front" && b.face == "rear" || a.face == "rear" && b.face == "front"
			same := a.face != "unknown" && b.face != "unknown" && (a.face == b.face || a.face == "both" || b.face == "both")
			depth := visualDimension(index[a.rack].Fields["depth_mm"])
			if opposite && a.full != nil && !*a.full && b.full != nil && !*b.full && depth != nil && a.depth != nil && b.depth != nil && *a.depth+*b.depth <= *depth {
				continue
			}
			depthConflict := opposite && depth != nil && a.depth != nil && b.depth != nil && *a.depth+*b.depth > *depth
			definite := same || a.full != nil && *a.full || b.full != nil && *b.full || depthConflict
			if definite {
				add(b.a.ID, "U_OVERLAP", "安装占位与 "+a.a.ID+" 冲突")
			} else {
				add(b.a.ID, "U_COMPATIBILITY_UNKNOWN", "与 "+a.a.ID+" 的 U 区间重合，安装面或深度不足以确认兼容")
			}
		}
	}
	return findings
}
func visualDimension(v any) *float64 {
	if v == nil || v == "" {
		return nil
	}
	n, e := strconv.ParseFloat(fmt.Sprint(v), 64)
	if e != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	return &n
}
