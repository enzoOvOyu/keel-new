package cmdb

import "testing"

func TestVisualPlacementKnownAndUnknownGeometry(t *testing.T) {
	rack := &VisualAsset{ID: "rack", Type: "rack", Fields: map[string]any{"u_height": 42, "depth_mm": 1000}}
	device := func(id string, u, h any, face string, depth float64, full bool) *VisualAsset {
		return &VisualAsset{ID: id, Type: "server", Fields: map[string]any{"rack_id": "rack", "u_start": u, "u_height": h, "mounting_face": face, "depth_mm": depth, "is_full_depth": full}}
	}
	tests := []struct {
		name  string
		items []*VisualAsset
		code  string
	}{
		{"half U", []*VisualAsset{rack, device("a", 1.5, .5, "front", 200, false)}, ""},
		{"zero U", []*VisualAsset{rack, device("a", nil, 0, "rear", 200, false)}, ""},
		{"opposed compatible", []*VisualAsset{rack, device("a", 1, 2, "front", 400, false), device("b", 1, 2, "rear", 400, false)}, ""},
		{"same face collision", []*VisualAsset{rack, device("a", 1, 2, "front", 400, false), device("b", 2, 2, "front", 400, false)}, "U_OVERLAP"},
		{"unknown face", []*VisualAsset{rack, device("a", 1, 2, "", 400, false), device("b", 1, 2, "", 400, false)}, "U_COMPATIBILITY_UNKNOWN"},
		{"unknown cabinet", []*VisualAsset{{ID: "rack", Type: "rack", Fields: map[string]any{}}, device("a", 1, 2, "front", 200, false)}, "MISSING_RACK_HEIGHT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			findings := visualRackPlacementFindings(test.items, "")
			if test.code == "" && len(findings) > 0 {
				t.Fatalf("valid geometry reported %+v", findings)
			}
			if test.code != "" {
				found := false
				for _, f := range findings {
					if f.Code == test.code {
						found = true
					}
					if test.code == "U_COMPATIBILITY_UNKNOWN" && f.Code == "U_OVERLAP" {
						t.Fatal("unknown is not a proven collision")
					}
				}
				if !found {
					t.Fatalf("missing %s: %+v", test.code, findings)
				}
			}
		})
	}
}
