package main

import (
	"context"
	"fmt"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"
)

// Bounded, short-lived typed read projections; never an asset fact source.
// The database revision and source head are checked on every authorized request.
var kickoffReads = struct {
	sync.Mutex
	entries map[string]*kickoffRead
}{entries: map[string]*kickoffRead{}}

type kickoffRead struct {
	mu                         sync.Mutex
	at                         time.Time
	model                      engineering.PhysicalModel
	evidence                   []engineering.SourceEvidence
	source                     string
	inputs                     []kickoff.ModelInput
	summary                    kickoff.View
	full                       *kickoff.View
	portCount, connectionCount int
	detailBuilds               int
}

func (v *kickoffRead) detail() kickoff.View {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.full == nil {
		v.detailBuilds++
		value := kickoff.Project(v.model, v.evidence, v.source, v.inputs)
		v.full = &value
	}
	return *v.full
}
func (s *Server) kickoffReadSnapshot(ctx context.Context, q engineeringQuery, revision int) (*kickoffRead, error) {
	var head string
	if err := q.QueryRow(ctx, `SELECT COALESCE(e.head_revision_id::text,'') FROM kickoff_workspace w LEFT JOIN eng_source e ON e.id=w.source_id WHERE w.project_id=$1`, s.project).Scan(&head); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%p:%s:%d:%s", s.db, s.project, revision, head)
	kickoffReads.Lock()
	for k, v := range kickoffReads.entries {
		if time.Since(v.at) > 10*time.Minute {
			delete(kickoffReads.entries, k)
		}
	}
	if v := kickoffReads.entries[key]; v != nil {
		v.at = time.Now()
		kickoffReads.Unlock()
		return v, nil
	}
	kickoffReads.Unlock()
	model, ev, source, err := s.kickoffModel(ctx, q)
	if err != nil {
		return nil, err
	}
	inputs, err := loadKickoffInputs(ctx, q, s.project)
	if err != nil {
		return nil, err
	}
	inputs = kickoff.ResolveReferences(model, inputs)
	v := &kickoffRead{at: time.Now(), model: model, evidence: ev, source: source, inputs: inputs, summary: kickoff.ProjectSummary(model, ev, source, inputs), portCount: len(model.Ports), connectionCount: len(model.Connections)}
	// Oversize projects remain queryable, but are not retained in the cache.
	if len(model.Equipment)+len(model.Ports)+len(model.Connections)+len(ev) <= 100000 {
		kickoffReads.Lock()
		defer kickoffReads.Unlock()
		if len(kickoffReads.entries) >= 3 {
			oldest := ""
			var at time.Time
			for k, e := range kickoffReads.entries {
				if oldest == "" || e.at.Before(at) {
					oldest = k
					at = e.at
				}
			}
			delete(kickoffReads.entries, oldest)
		}
		kickoffReads.entries[key] = v
	}
	return v, nil
}

func kickoffTypedPage(w http.ResponseWriter, r *http.Request, read *kickoffRead, project, name string, revision int) bool {
	section := r.URL.Query().Get("section")
	if section == "" || section == "review" || section == "context" || section == "calculations" || section == "sources" || section == "publications" || section == "activity" {
		return false
	}
	view := read.summary
	if section == "ports" || section == "connections" {
		view = read.detail()
	}
	values := map[string]any{"devices": view.Devices, "ports": view.Ports, "connections": view.Connections, "components": view.Components, "racks": view.Racks, "issues": view.Issues, "models": read.inputs}
	value, ok := values[section]
	if !ok {
		fail(w, 400, "KICKOFF_SECTION_INVALID")
		return true
	}
	offset, limit := 0, 100
	for key, target := range map[string]*int{"offset": &offset, "limit": &limit} {
		if raw := r.URL.Query().Get(key); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil {
				fail(w, 400, "KICKOFF_PAGE_INVALID")
				return true
			}
			*target = n
		}
	}
	if offset < 0 || limit < 1 || limit > 200 {
		fail(w, 400, "KICKOFF_PAGE_INVALID")
		return true
	}
	payload := map[string]any{"schemaVersion": 1, "projectId": project, "projectName": name, "revision": revision}
	totals := map[string]int{}
	for key, v := range values {
		totals[key] = reflect.ValueOf(v).Len()
		payload[key] = []any{}
	}
	for _, key := range []string{"sources", "publications", "activity", "calculations"} {
		payload[key] = []any{}
	}
	totals["ports"] = read.portCount
	totals["connections"] = read.connectionCount
	items := reflect.ValueOf(value)
	start := min(offset, items.Len())
	end := min(start+limit, items.Len())
	payload[section] = items.Slice(start, end).Interface()
	payload["totals"] = totals
	payload["page"] = map[string]any{"section": section, "offset": offset, "limit": limit, "total": items.Len(), "hasMore": end < items.Len()}
	respond(w, 200, payload)
	return true
}
