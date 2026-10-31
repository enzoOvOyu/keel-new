package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"keel.local/dev/internal/flows"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) flowCompletion(w http.ResponseWriter, r *http.Request, scope flows.Scope, parts []string) bool {
	if len(parts) == 1 && parts[0] == "approval-request" && r.Method == "POST" {
		var in struct {
			Run    flows.RunRequest `json:"run"`
			Reason string           `json:"reason"`
		}
		if !readFlowInput(w, r, &in) {
			return true
		}
		if !subjectOf(r.Context()).Can("flows:execute", in.Run.Project, "", "") {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		decisions, e := s.flows.Preflight(r.Context(), scope, in.Run)
		if e != nil {
			flowsFail(w, e)
			return true
		}
		for _, d := range decisions {
			if in.Run.TargetRevisions[d.Asset] != d.Revision {
				fail(w, 409, "APPROVAL_PREFLIGHT_VERSION_REQUIRED")
				return true
			}
		}

		resource := in.Run.Flow
		if resource == "" {
			resource = "quick-run"
		}
		b, _ := json.Marshal(map[string]any{"action": in.Run.ApprovalAction, "resource_type": "flow_job", "resource_id": resource, "snapshot_version": flows.ApprovalDigest(in.Run), "risk": "normal", "target_ids": in.Run.Targets, "reason": in.Reason + "\n\nFrozen job snapshot (SHA256=" + flows.ApprovalDigest(in.Run) + "):\n" + string(jsonBytes(in.Run))})
		clone := r.Clone(r.Context())
		clone.URL.Path = "/api/v1/projects/" + in.Run.Project + "/approvals"
		clone.Body = http.NoBody
		clone.Body = readCloser{bytes.NewReader(b)}
		return s.approvalRoutes(w, clone)
	}
	if len(parts) == 2 && parts[1] == "schedule" {
		var in *flows.Schedule
		if r.Method == "PUT" {
			in = &flows.Schedule{}
			if !readFlowInput(w, r, in) {
				return true
			}
			u := subjectOf(r.Context())
			f, e := s.flows.Flow(r.Context(), scope, parts[0])
			if e != nil {
				flowsFail(w, e)
				return true
			}
			if !u.Can("flows:execute", f.Project, "", "") || !u.Can("flows:edit", f.Project, "", "") {
				fail(w, 403, "FORBIDDEN")
				return true
			}
		} else if r.Method != "GET" {
			fail(w, 405, "METHOD_NOT_ALLOWED")
			return true
		}
		out, e := s.flows.Schedule(r.Context(), scope, parts[0], in)
		if e != nil {
			flowsFail(w, e)
		} else {
			respond(w, 200, out)
		}
		return true
	}
	if len(parts) >= 2 && parts[1] == "namespace-artifacts" {
		id := parts[0]
		var out any
		var e error
		if len(parts) == 2 && r.Method == "GET" {
			out, e = s.flows.FileVersions(r.Context(), scope, id)
		} else if len(parts) == 2 && r.Method == "POST" {
			var in flows.FileUpload
			if !readFlowInput(w, r, &in) {
				return true
			}
			out, e = s.flows.UploadFile(r.Context(), scope, id, in)
		} else if len(parts) == 3 && r.Method == "GET" {
			_, b, err := s.flows.FileContent(r.Context(), scope, id, parts[2])
			if err != nil {
				flowsFail(w, err)
			} else {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Disposition", "attachment")
				w.Write(b)
			}
			return true
		} else if len(parts) == 4 && parts[3] == "distribute" && r.Method == "POST" {
			var in flows.RunRequest
			if !readFlowInput(w, r, &in) {
				return true
			}
			f, err := s.flows.Flow(r.Context(), scope, id)
			if err != nil {
				flowsFail(w, err)
				return true
			}
			if !subjectOf(r.Context()).Can("flows:execute", f.Project, "", "") {
				fail(w, 403, "FORBIDDEN")
				return true
			}
			out, e = s.flows.DistributeFile(r.Context(), scope, id, parts[2], in, r.Header.Get("Idempotency-Key"))
		} else {
			fail(w, 405, "METHOD_NOT_ALLOWED")
			return true
		}
		if e != nil {
			flowsFail(w, e)
		} else {
			respond(w, 200, out)
		}
		return true
	}
	if len(parts) == 3 && parts[0] == "executions" && parts[2] == "steps" && r.Method == "GET" {
		out, e := s.flows.StepResults(r.Context(), scope, parts[1])
		if e != nil {
			flowsFail(w, e)
		} else {
			respond(w, 200, out)
		}
		return true
	}
	if len(parts) == 3 && parts[0] == "executions" && parts[2] == "events" && r.Method == "GET" {
		s.flowJournal(w, r, scope, parts[1])
		return true
	}
	return false
}

type readCloser struct{ *bytes.Reader }

func (readCloser) Close() error { return nil }
func (s *Server) flowJournal(w http.ResponseWriter, r *http.Request, scope flows.Scope, id string) {
	after := int64(0)
	cursor := r.Header.Get("Last-Event-ID")
	if cursor == "" {
		cursor = r.URL.Query().Get("after")
	}
	if cursor != "" {
		n, e := strconv.ParseInt(cursor, 10, 64)
		if e != nil || n < 0 {
			fail(w, 400, "EVENT_CURSOR_INVALID")
			return
		}
		after = n
	}
	if e := s.flows.CaptureEvents(r.Context(), scope, id); e != nil {
		flowsFail(w, e)
		return
	}
	events, e := s.flows.Events(r.Context(), scope, id, after)
	if e != nil {
		flowsFail(w, e)
		return
	}
	if r.URL.Query().Get("history") == "true" {
		respond(w, 200, events)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	f, ok := w.(http.Flusher)
	if !ok {
		return
	}
	f.Flush()
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		for _, event := range events {
			if _, e = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, event.Data); e != nil {
				return
			}
			after = event.ID
		}
		f.Flush()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		subject := subjectOf(r.Context())
		var valid bool
		if e = s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app_user u JOIN api_session a ON a.user_id=u.id WHERE u.id::text=$1 AND u.status='active' AND u.permission_version=$2 AND a.token_hash=$3 AND a.revoked_at IS NULL AND a.expires_at>now())`, subject.ID, subject.Version, subject.SessionID).Scan(&valid); e != nil || !valid {
			return
		}
		if e = s.flows.CaptureEvents(ctx, scope, id); e != nil {
			return
		}
		events, e = s.flows.Events(ctx, scope, id, after)
		if e != nil {
			return
		}
		if len(events) == 0 {
			if _, e = w.Write([]byte(": heartbeat\n\n")); e != nil {
				return
			}
			f.Flush()
		}
	}
}
