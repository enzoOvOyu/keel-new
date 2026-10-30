package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"keel.local/dev/internal/bom"
	"net/http"
	"strings"
)

func (s *Server) models(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/v1/bom-models" {
		return false
	}
	ctx := r.Context()
	if r.Method == "GET" {
		rows, e := s.db.Query(ctx, "SELECT id,model,category,capabilities,approved_by FROM device_model_revision ORDER BY created_at DESC LIMIT 200")
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, model, category, actor string
			var caps []byte
			rows.Scan(&id, &model, &category, &caps, &actor)
			out = append(out, map[string]any{"id": id, "model": model, "category": category, "capabilities": json.RawMessage(caps), "approved_by": actor})
		}
		respond(w, 200, out)
		return true
	}
	if r.Method != "POST" {
		return false
	}
	var q struct {
		Model        string         `json:"model"`
		Category     string         `json:"category"`
		Reason       string         `json:"reason"`
		Capabilities map[string]any `json:"capabilities"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || strings.TrimSpace(q.Model) == "" || strings.TrimSpace(q.Reason) == "" {
		fail(w, 422, "MODEL_APPROVAL_REQUIRED")
		return true
	}
	if !bom.RequiresModel(q.Category) {
		fail(w, 422, "INVALID_CATEGORY")
		return true
	}
	caps, _ := json.Marshal(q.Capabilities)
	id := uuid.NewString()
	e := s.db.QueryRow(ctx, "INSERT INTO device_model_revision(id,model,category,capabilities,approved_by,approval_reason) VALUES($1,$2,$3,$4,$6,$5) ON CONFLICT(model,category,capabilities) DO UPDATE SET model=excluded.model RETURNING id", id, q.Model, q.Category, caps, q.Reason, actorName(ctx)).Scan(&id)
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	s.audit(ctx, id, "model.approved", q.Reason)
	respond(w, 201, map[string]string{"id": id})
	return true
}
