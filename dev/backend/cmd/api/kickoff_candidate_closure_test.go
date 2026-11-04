package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/engineering"
	"testing"
)

func TestKickoffCandidateClosure(t *testing.T) {
	result := ai.KickoffResult{Candidates: []ai.KickoffAgentCandidate{
		{TableID: "devices", Rows: []map[string]string{{"device_ref": "Node A"}}},
		{TableID: "placements", Rows: []map[string]string{{"device_id": "node a", "rack_id": "rack-1", "start_u": "41u-"}, {"device_id": "inventory only", "rack_id": "rack-1", "start_u": "2"}, {"device_id": "Node A", "rack_id": "rack-2", "start_u": "unknown"}}, Issues: []engineering.InputProblem{{Row: 1, Code: "KICKOFF_INPUT_INVALID", Message: "第1行起始U格式无效"}}},
	}}
	kickoffPrepareCandidates(&result)
	c := result.Candidates[1]
	rows, err := kickoffAgentAdoptionRows(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["start_u"] != "41" || rows[0]["height_u"] != "" || rows[1]["start_u"] != "" || rows[1]["rack_id"] != "rack-2" {
		t.Fatalf("wrong eligible projection: %+v", rows)
	}
	if len(c.Rows) != 3 || len(c.Issues) != 2 {
		t.Fatal("unmatched evidence or missing-position issue lost")
	}
	kickoffPrepareCandidates(&result)
	if len(result.Candidates[1].Issues) != 2 {
		t.Fatal("preparation not idempotent")
	}
}

func TestKickoffCandidateDeviceRole(t *testing.T) {
	model := engineering.EmptyPhysical("test-role")
	var evidence []engineering.SourceEvidence
	candidate := identityCandidate("devices", map[string]string{"device_ref": "Node-A", "label": "Node-A", "equipment_type": "server", "role": "gpu_compute"})
	kickoffAgentMergePhysical(&model, &evidence, candidate, "source")
	if len(model.Equipment) != 1 || model.Equipment[0].Role != "gpu_compute" {
		t.Fatal("device role lost during adoption")
	}
}
