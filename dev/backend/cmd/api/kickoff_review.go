package main

import (
	"fmt"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/kickoff"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type kickoffReviewRow struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Reference   string           `json:"reference"`
	Location    string           `json:"location"`
	State       string           `json:"state"`
	Evidence    kickoff.Evidence `json:"evidence"`
}

func kickoffReviewPage(w http.ResponseWriter, r *http.Request, view kickoff.View, pubs []kickoff.Publication, revision int) {
	q := r.URL.Query()
	category := q.Get("category")
	if category == "" {
		category = "devices"
	}
	offset, limit := 0, 10
	var err error
	if q.Has("offset") {
		offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil || offset < 0 {
			fail(w, 400, "KICKOFF_PAGE_INVALID")
			return
		}
	}
	if q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 200 {
			fail(w, 400, "KICKOFF_PAGE_INVALID")
			return
		}
	}
	blocked, issues, published := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, i := range view.Issues {
		if !i.Resolved {
			issues[i.EntityID] = true
			if i.Scope == "identity" {
				blocked[i.EntityID] = true
			}
		}
	}
	for _, p := range pubs {
		if p.State == "published" {
			for _, id := range p.DeviceIDs {
				published[id] = true
			}
		}
	}
	state := func(id string) string {
		if blocked[id] {
			return "blocked"
		}
		if published[id] {
			return "published"
		}
		return "ready"
	}
	devices := map[string]kickoff.Device{}
	ports := map[string]kickoff.Port{}
	racks := map[string]kickoff.Rack{}
	for _, d := range view.Devices {
		devices[d.ID] = d
	}
	for _, p := range view.Ports {
		ports[p.ID] = p
	}
	for _, v := range view.Racks {
		racks[v.ID] = v
	}
	rows := []kickoffReviewRow{}
	switch category {
	case "devices", "placements":
		for _, d := range view.Devices {
			v := kickoffReviewRow{ID: d.ID, Name: d.Name, Description: d.Model, Reference: d.Serial, Location: racks[d.RackID].Name, State: state(d.ID), Evidence: d.Evidence}
			if v.Location == "" {
				v.Location = "待定位"
			}
			if category == "placements" {
				v.Reference = v.Location
				v.Location = "U位待补充 / 高度待核对"
				v.State = "blocked"
				if d.StartU != nil {
					v.Location = fmt.Sprintf("U%d / 高度待核对", *d.StartU)
				}
				rack := racks[d.RackID]
				if d.StartU != nil && d.HeightU != nil {
					v.Location = fmt.Sprintf("U%d / %dU", *d.StartU, *d.HeightU)
					if rack.CapacityU != nil && *d.StartU+*d.HeightU-1 <= *rack.CapacityU && !issues[d.ID] {
						v.State = "ready"
					}
				}
			}
			rows = append(rows, v)
		}
	case "components":
		for _, c := range view.Components {
			rows = append(rows, kickoffReviewRow{c.ID, c.Name, c.Model, devices[c.DeviceID].Name, c.Slot, state(c.DeviceID), c.Evidence})
		}
	case "ports":
		for _, p := range view.Ports {
			rows = append(rows, kickoffReviewRow{p.ID, p.Name, p.Speed, devices[p.DeviceID].Name, p.MAC, state(p.DeviceID), p.Evidence})
		}
	case "connections":
		for _, c := range view.Connections {
			a, b := ports[c.A], ports[c.B]
			status := "ready"
			if a.ID == "" || b.ID == "" || blocked[a.DeviceID] || blocked[b.DeviceID] {
				status = "blocked"
			}
			rows = append(rows, kickoffReviewRow{c.ID, devices[a.DeviceID].Name, a.Name, devices[b.DeviceID].Name + " / " + b.Name, c.Medium, status, c.Evidence})
		}
	default:
		fail(w, 400, "KICKOFF_CATEGORY_INVALID")
		return
	}
	filtered := rows[:0]
	search := strings.ToLower(q.Get("q"))
	for _, v := range rows {
		if q.Get("issues") == "true" && v.State != "blocked" && !issues[v.ID] {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(v.Name+" "+v.Description+" "+v.Reference+" "+v.Location+" "+v.State), search) {
			continue
		}
		filtered = append(filtered, v)
	}
	field := q.Get("sort")
	value := func(v kickoffReviewRow) string {
		switch field {
		case "reference":
			return v.Reference
		case "location":
			return v.Location
		case "state":
			return v.State
		}
		return v.Name
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := value(filtered[i]), value(filtered[j])
		if a == b {
			return filtered[i].ID < filtered[j].ID
		}
		if q.Get("desc") == "true" {
			return a > b
		}
		return a < b
	})
	start := min(offset, len(filtered))
	end := min(start+limit, len(filtered))
	respond(w, 200, map[string]any{"revision": revision, "rows": filtered[start:end], "total": len(filtered), "offset": offset, "limit": limit, "hasMore": end < len(filtered)})
}

func (s *Server) kickoffPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceIDs        []string `json:"deviceIds"`
		ExpectedRevision int      `json:"expectedRevision"`
	}
	if !engineeringDecode(w, r, &body) {
		return
	}
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	var revision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM kickoff_workspace WHERE project_id=$1`, s.project).Scan(&revision); err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	if revision != body.ExpectedRevision {
		fail(w, 409, "REVISION_CONFLICT")
		return
	}
	read, err := s.kickoffReadSnapshot(ctx, tx, revision)
	if err != nil {
		fail(w, 500, "KICKOFF_MODEL_UNAVAILABLE")
		return
	}
	_, receipt, err := kickoff.Select(read.model, read.detail(), body.DeviceIDs)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	respond(w, 200, map[string]any{"revision": revision, "counts": receipt.Counts})
}
