package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"strings"
	"testing"
)

func TestKickoffAgentUnderstandingNotApplicable(t *testing.T) {
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "notes", Cells: []bom.Cell{{Sheet: "notes", Ref: "A1", Row: 1, Column: 1, Value: "Notes"}, {Sheet: "notes", Ref: "A2", Row: 2, Column: 1, Value: "Supporting memo"}}}}}
	result := ai.KickoffResult{Regions: []ai.KickoffRegion{{ID: "memo", Sheet: "notes", Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 1}, Steps: []int{}, Reason: "non-engineering memo"}}}
	report := kickoffAgentUnderstandingReport("source", "job", w, result)
	if report.State != "recognized" {
		t.Fatalf("non-applicable report stuck: %+v", report.Steps)
	}
	for _, i := range []int{2, 3, 4} {
		if report.Steps[i].State != "not_applicable" {
			t.Fatal("non-applicable stage counted as missing")
		}
	}
	result.Missing = []string{"Unresolved source information"}
	report = kickoffAgentUnderstandingReport("source", "job", w, result)
	if report.State != "partial" {
		t.Fatal("missing information silently passed")
	}
}

func terminationUnderstandingFixture() (bom.Workbook, ai.KickoffResult) {
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "links", Cells: []bom.Cell{
		{Sheet: "links", Ref: "A1", Row: 1, Column: 1, Value: "switch-a"},
		{Sheet: "links", Ref: "B1", Row: 1, Column: 2, Value: "eth1"},
		{Sheet: "links", Ref: "C1", Row: 1, Column: 3, Value: "switch-b"},
		{Sheet: "links", Ref: "D1", Row: 1, Column: 4, Value: "eth2"},
	}}}}
	ref := func(cell string) ai.KickoffAgentCell { return ai.KickoffAgentCell{Sheet: "links", Cell: cell} }
	result := ai.KickoffResult{FlowVersion: "termination-v1",
		Regions:  []ai.KickoffRegion{{ID: "links", Sheet: "links", Bounds: engineering.Bounds{StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 4}, Steps: []int{1}}},
		Coverage: []ai.KickoffCoverage{{Region: "links", Step: 1, State: "extracted", Rows: 1}},
		Candidates: []ai.KickoffAgentCandidate{
			{TableID: "devices", Rows: []map[string]string{{"device_ref": "switch-a"}, {"device_ref": "switch-b"}}, Evidence: []ai.KickoffAgentEvidence{
				{Row: 1, Fields: map[string]ai.KickoffAgentCell{"device_ref": ref("A1")}, Missing: []string{"serial", "model"}},
				{Row: 2, Fields: map[string]ai.KickoffAgentCell{"device_ref": ref("C1")}},
			}},
			{TableID: "ports", Rows: []map[string]string{{"device_ref": "switch-a", "port": "eth1"}, {"device_ref": "switch-b", "port": "eth2"}}, Evidence: []ai.KickoffAgentEvidence{
				{Row: 1, Fields: map[string]ai.KickoffAgentCell{"device_ref": ref("A1"), "port": ref("B1")}},
				{Row: 2, Fields: map[string]ai.KickoffAgentCell{"device_ref": ref("C1"), "port": ref("D1")}},
			}},
			{TableID: "connections", Rows: []map[string]string{{"a_device_ref": "switch-a", "a_port": "eth1", "b_device_ref": "switch-b", "b_port": "eth2"}}, Evidence: []ai.KickoffAgentEvidence{
				{Row: 1, Fields: map[string]ai.KickoffAgentCell{"a_device_ref": ref("A1"), "a_port": ref("B1"), "b_device_ref": ref("C1"), "b_port": ref("D1")}},
			}},
		},
	}
	return w, result
}

func TestKickoffTerminationUnderstandingEvidence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		change     func(*bom.Workbook, *ai.KickoffResult)
		recognized bool
	}{
		{"optional absence", func(_ *bom.Workbook, r *ai.KickoffResult) {
			r.Missing = []string{"No serial, model, rack position or cable length provided"}
		}, true},
		{"unresolved endpoint", func(_ *bom.Workbook, r *ai.KickoffResult) { r.Candidates[2].Rows[0]["b_device_ref"] = "unknown" }, false},
		{"duplicate device", func(_ *bom.Workbook, r *ai.KickoffResult) {
			r.Candidates[0].Rows = append(r.Candidates[0].Rows, map[string]string{"device_ref": "switch-a"})
		}, false},
		{"missing source", func(_ *bom.Workbook, r *ai.KickoffResult) { delete(r.Candidates[2].Evidence[0].Fields, "a_port") }, false},
		{"unverified formula", func(w *bom.Workbook, _ *ai.KickoffResult) { w.Sheets[0].Cells[0].Formula = "A2" }, false},
		{"unexpanded coverage", func(_ *bom.Workbook, r *ai.KickoffResult) {
			r.Coverage = append(r.Coverage, ai.KickoffCoverage{Region: "other", Step: 1, State: "needs_input"})
		}, false},
		{"no coverage", func(_ *bom.Workbook, r *ai.KickoffResult) { r.Coverage = nil }, false},
		{"candidate issue", func(_ *bom.Workbook, r *ai.KickoffResult) {
			r.Candidates[2].Issues = []engineering.InputProblem{{Row: 1, Field: "a_port", Code: "INVALID"}}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, r := terminationUnderstandingFixture()
			tc.change(&w, &r)
			report := kickoffAgentUnderstandingReport("source", "job", w, r)
			if (report.State == "recognized") != tc.recognized {
				t.Fatalf("unexpected report: %+v", report)
			}
			if report.FlowVersion != "termination-v1" {
				t.Fatal("version missing")
			}
			if report.Steps[3].State != "not_applicable" {
				t.Fatal("absent optional association blocked")
			}
		})
	}
}

func TestKickoffTerminationUnderstandingLegacy(t *testing.T) {
	w, r := terminationUnderstandingFixture()
	r.FlowVersion = ""
	report := kickoffAgentUnderstandingReport("source", "job", w, r)
	if report.FlowVersion != "" || report.Steps[0].Total != len(w.Sheets) {
		t.Fatal("historical result upgraded")
	}
}

func TestKickoffTerminationPending(t *testing.T) {
	for _, state := range []string{"pending", "running", "failed"} {
		report := emptyUnderstanding("source", "job")
		kickoffTerminationPending(&report, state, []string{"route", "graph", "reason_relationships", "kickoff_step_1", "kickoff_step_2", "kickoff_step_3"})
		if report.FlowVersion != "termination-v1" {
			t.Fatal("missing queued version")
		}
		for _, step := range report.Steps {
			if step.State != "pending" || step.Count != 0 || step.Completed != 0 || step.Total != 0 {
				t.Fatalf("tool completion inflated progress: %+v", step)
			}
		}
		if !strings.Contains(report.Steps[2].Note, "reason_relationships") || !strings.Contains(report.Steps[3].Note, "kickoff_step_3") {
			t.Fatal("missing actual tool notes")
		}
	}
}

func TestKickoffTerminationIssueIndex(t *testing.T) {
	w, r := terminationUnderstandingFixture()
	r.Candidates[0].Issues = []engineering.InputProblem{{Row: 2, Field: "device_ref", Code: "INVALID"}}
	report := kickoffAgentUnderstandingReport("source", "job", w, r)
	if report.Steps[1].Completed != 2 {
		t.Fatalf("row issue leaked: %+v", report.Steps[1])
	}
	w, r = terminationUnderstandingFixture()
	for i := 0; i < 20000; i++ {
		r.Candidates[0].Issues = append(r.Candidates[0].Issues, engineering.InputProblem{Row: 0, Field: "serial", Code: "MISSING"})
	}
	report = kickoffAgentUnderstandingReport("source", "job", w, r)
	if report.State != "recognized" {
		t.Fatal("optional global absence blocked candidates")
	}
	r.Candidates[0].Issues = append(r.Candidates[0].Issues, engineering.InputProblem{Row: 0, Code: "INVALID"})
	report = kickoffAgentUnderstandingReport("source", "job", w, r)
	if report.State == "recognized" {
		t.Fatal("global issue ignored")
	}
}
