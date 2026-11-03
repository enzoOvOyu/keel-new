package main

import (
	"encoding/json"
	"keel.local/dev/internal/kickoff"
	"net/http/httptest"
	"testing"
)

func TestKickoffReviewEngineeringSignals(t *testing.T) {
	u, height, capacity := 2, 2, 42
	view := kickoff.View{Devices: []kickoff.Device{{ID: "d1", Name: "synthetic-sn", Serial: "synthetic-sn", Type: "server", Model: "model"}, {ID: "d2", Name: "named", Type: "switch", Model: "model", RackID: "rack", StartU: &u, HeightU: &height}}, Racks: []kickoff.Rack{{ID: "rack", Name: "rack", CapacityU: &capacity}}, Ports: []kickoff.Port{{ID: "p1", DeviceID: "d1", Name: "eth1", Speed: "400G", MAC: "02:00:00:00:00:01"}, {ID: "p2", DeviceID: "d2", Name: "eth2", MAC: "02:00:00:00:00:01"}}, Connections: []kickoff.Connection{{ID: "c1", A: "p1", B: "p2"}, {ID: "c2", A: "p1", B: "missing"}}}
	pubs := []kickoff.Publication{{State: "published", DeviceIDs: []string{"d1"}, PortIDs: []string{"p1"}, ConnectionIDs: []string{"c1"}}}
	placements := kickoffReviewRows(view, pubs, "placements")
	if placements[0].State != "published" || !reviewMatches(placements[0], "unplaced") || !reviewMatches(placements[0], "geometry") {
		t.Fatal("geometry must stay visible without blocking asset publication")
	}
	if reviewMatches(placements[1], "issues") {
		t.Fatal("complete placement incorrectly flagged")
	}
	ports := kickoffReviewRows(view, pubs, "ports")
	if ports[0].Facts.Speed != "400G" || ports[0].Name != "eth1" || ports[0].Facts.ConnectionCount != 2 || !reviewMatches(ports[0], "occupied") {
		t.Fatal("port facts or occupancy lost")
	}
	if ports[1].State == "published" {
		t.Fatal("parent publication must not imply child published")
	}
	links := kickoffReviewRows(view, pubs, "connections")
	if links[0].State != "published" || links[1].State != "blocked" || links[1].Signals[0].Code != "endpoint_missing" {
		t.Fatal("connection receipt or blocking priority incorrect")
	}
}
func TestKickoffReviewFiltersUseWholeResult(t *testing.T) {
	view := kickoff.View{Devices: []kickoff.Device{}}
	for _, id := range []string{"a", "b", "c"} {
		view.Devices = append(view.Devices, kickoff.Device{ID: id, Name: id, Type: "server", Model: "synthetic", Evidence: kickoff.Evidence{SourceID: "source"}})
	}
	w := httptest.NewRecorder()
	kickoffReviewPage(w, httptest.NewRequest("GET", "/review?category=devices&filter=unplaced&type=server&limit=1&offset=1", nil), view, nil, 7)
	var got struct {
		Data struct {
			Revision, Total int
			Rows            []kickoffReviewRow
			Counts          map[string]int
			Facets          map[string][]kickoffReviewOption
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || got.Data.Total != 3 || len(got.Data.Rows) != 1 || got.Data.Counts["unplaced"] != 3 || got.Data.Rows[0].ID != "b" || len(got.Data.Facets["model"]) != 1 {
		t.Fatal("full-scope counts, facets or pagination wrong")
	}
	w = httptest.NewRecorder()
	kickoffReviewPage(w, httptest.NewRequest("GET", "/review?filter=not-supported", nil), view, nil, 7)
	if w.Code != 400 {
		t.Fatal("unsupported filter accepted")
	}
}
func TestKickoffReviewPlacementOverlapAndParentConflict(t *testing.T) {
	a, b, h := 1, 2, 2
	view := kickoff.View{Devices: []kickoff.Device{{ID: "a", RackID: "rack", StartU: &a, HeightU: &h}, {ID: "b", RackID: "rack", StartU: &b, HeightU: &h}}, Racks: []kickoff.Rack{{ID: "rack"}}, Ports: []kickoff.Port{{ID: "port", DeviceID: "a"}}, Issues: []kickoff.Issue{{EntityID: "a", Scope: "identity"}}}
	rows := kickoffReviewRows(view, nil, "placements")
	for _, row := range rows {
		found := false
		for _, s := range row.Signals {
			if s.Label == "U 位重叠" {
				found = true
			}
		}
		if !found {
			t.Fatal("overlap not surfaced")
		}
	}
	rows = kickoffReviewRows(view, nil, "ports")
	if rows[0].State != "blocked" || rows[0].Signals[0].Severity != "block" {
		t.Fatal("parent identity conflict not emphasized")
	}
}
