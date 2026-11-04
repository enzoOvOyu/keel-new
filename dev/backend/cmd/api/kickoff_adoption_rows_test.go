package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/engineering"
	"testing"
)

func TestKickoffAdoptionEligibilityUsesFullCandidate(t *testing.T) {
	c := ai.KickoffAgentCandidate{TableID: "placements"}
	for i := 1; i <= 25; i++ {
		c.Rows = append(c.Rows, map[string]string{"start_u": "bad"})
		c.Issues = append(c.Issues, engineering.InputProblem{Row: i})
	}
	rows, err := kickoffAgentAdoptionRows(c)
	if err != nil || len(rows) != 0 {
		t.Fatalf("invalid rows selectable: %d %v", len(rows), err)
	}
	c = ai.KickoffAgentCandidate{TableID: "cable_specs", Rows: []map[string]string{{"unrecognized_field": "x"}}}
	if _, err = kickoffAgentAdoptionRows(c); err == nil {
		t.Fatal("preview must apply same schema validation as adoption")
	}
	c = ai.KickoffAgentCandidate{TableID: "devices", Rows: []map[string]string{{"serial": "SYNTHETIC"}}}
	if rows, err = kickoffAgentAdoptionRows(c); err != nil || len(rows) != 1 {
		t.Fatal("physical candidate incorrectly treated as input table")
	}

	c = ai.KickoffAgentCandidate{TableID: "placements", Rows: []map[string]string{{"rack_id": "synthetic-rack", "start_u": "12"}}}
	if rows, err = kickoffAgentAdoptionRows(c); err == nil || len(rows) != 0 {
		t.Fatal("missing device identity must not be selectable")
	}
	c = ai.KickoffAgentCandidate{TableID: "cable_specs", Rows: []map[string]string{{"code": "synthetic-cable"}}}
	if rows, err = kickoffAgentAdoptionRows(c); err == nil || len(rows) != 0 {
		t.Fatal("missing merge key must not be selectable")
	}
}
