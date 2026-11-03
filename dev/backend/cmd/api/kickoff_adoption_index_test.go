package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/engineering"
	"testing"
)

func TestAdoptionEvidenceIndexPreservesDuplicateSources(t *testing.T) {
	m := engineering.PhysicalModel{ProjectID: "test"}
	var ev []engineering.SourceEvidence
	c := ai.KickoffAgentCandidate{TableID: "devices", Rows: []map[string]string{{"serial": "TEST-SN"}, {"serial": "TEST-SN"}}, Evidence: []ai.KickoffAgentEvidence{{Row: 1, Fields: map[string]ai.KickoffAgentCell{"serial": {Sheet: "one", Cell: "A1"}}}, {Row: 2, Fields: map[string]ai.KickoffAgentCell{"serial": {Sheet: "two", Cell: "B2"}}}, {Row: 99, Fields: map[string]ai.KickoffAgentCell{"serial": {Sheet: "invalid", Cell: "X1"}}}}}
	if !kickoffAgentMergePhysical(&m, &ev, c, "source") {
		t.Fatal("no adoption")
	}
	if len(m.Equipment) != 1 || len(ev) != 2 {
		t.Fatalf("equipment=%d evidence=%d", len(m.Equipment), len(ev))
	}
	kickoffAgentMergePhysical(&m, &ev, c, "source")
	if len(m.Equipment) != 1 || len(ev) != 2 {
		t.Fatal("repeat adoption duplicated entities/evidence")
	}
}
