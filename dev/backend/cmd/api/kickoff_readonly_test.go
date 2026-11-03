package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/kickoff"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"
)

// Explicitly opt-in, existing data only; every pooled connection is read-only.
func TestKickoffExistingMMReadOnly(t *testing.T) {
	if os.Getenv("KEEL_KICKOFF_READONLY") != "1" {
		t.Skip("requires explicit existing-data read-only validation")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid private database configuration")
	}
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	ctx := context.Background()
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("database connection failed")
	}
	defer db.Close()
	project := os.Getenv("KEEL_KICKOFF_PROJECT")
	if project == "" {
		t.Fatal("KEEL_KICKOFF_PROJECT required")
	}
	svc := &Server{db: db, project: project}
	samples := map[string][]float64{}
	get := func(query string) map[string]json.RawMessage {
		t.Helper()
		w := httptest.NewRecorder()
		start := time.Now()
		svc.kickoffWorkspace(w, httptest.NewRequest("GET", "/workspace"+query, nil))
		if w.Code != 200 {
			t.Fatalf("workspace status=%d", w.Code)
		}
		var envelope struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		elapsed := float64(time.Since(start).Microseconds()) / 1000
		if query == "" {
			t.Logf("homepage_ms=%.2f bytes=%d", elapsed, w.Body.Len())
		} else {
			kind := queryCategory(query)
			samples[kind] = append(samples[kind], elapsed)
		}
		return envelope.Data
	}
	first := get("")
	var rev int
	_ = json.Unmarshal(first["revision"], &rev)
	var totals map[string]int
	_ = json.Unmarshal(first["totals"], &totals)
	var ports []kickoff.Port
	_ = json.Unmarshal(first["ports"], &ports)
	if len(ports) != 0 {
		t.Fatal("homepage returned port details")
	}
	for _, section := range []string{"ports", "connections"} {
		seen := map[string]bool{}
		for offset := 0; offset < totals[section]; offset += 200 {
			data := get(fmt.Sprintf("?section=%s&offset=%d&limit=200&revision=%d", section, offset, rev))
			var gotRevision int
			_ = json.Unmarshal(data["revision"], &gotRevision)
			if gotRevision != rev {
				t.Fatal("mixed revisions")
			}
			var items []struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(data[section], &items)
			for _, v := range items {
				if seen[v.ID] {
					t.Fatal("duplicate page identity")
				}
				seen[v.ID] = true
			}
		}
		if len(seen) != totals[section] {
			t.Fatalf("incomplete %s expected=%d actual=%d", section, totals[section], len(seen))
		}
		t.Logf("section=%s unique_count=%d revision=%d", section, len(seen), rev)
	}
	for kind, values := range samples {
		sort.Float64s(values)
		index := (len(values)*95+99)/100 - 1
		t.Logf("kind=%s requests=%d p95_ms=%.2f max_ms=%.2f", kind, len(values), values[index], values[len(values)-1])
	}
	var devices []kickoff.Device
	_ = json.Unmarshal(first["devices"], &devices)
	if len(devices) > 0 {
		data := get(fmt.Sprintf("?section=context&entity_id=%s&revision=%d", devices[0].ID, rev))
		var local []kickoff.Port
		_ = json.Unmarshal(data["ports"], &local)
		if len(local) >= totals["ports"] {
			t.Fatal("context downloaded entire port collection")
		}
		t.Logf("local_context_ports=%d", len(local))
		ids := []string{}
		for _, d := range devices {
			ids = append(ids, d.ID)
		}
		raw, _ := json.Marshal(map[string]any{"deviceIds": ids, "expectedRevision": rev})
		response := httptest.NewRecorder()
		svc.kickoffPreview(response, httptest.NewRequest("POST", "/preview", bytes.NewReader(raw)))
		if response.Code != 200 {
			t.Fatalf("scope preview status=%d", response.Code)
		}
		var envelope struct {
			Data struct {
				Counts map[string]int `json:"counts"`
			} `json:"data"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &envelope)
		if envelope.Data.Counts["ports"] != totals["ports"] || envelope.Data.Counts["connections"] != totals["connections"] {
			t.Fatal("preview did not assess complete relationship scope")
		}
	}
	kickoffReads.Lock()
	for _, v := range kickoffReads.entries {
		if v.model.ProjectID == project && v.full != nil && v.detailBuilds != 1 {
			t.Error("repeated full projection on subsequent pages")
		}
	}
	kickoffReads.Unlock()
	w := httptest.NewRecorder()
	svc.kickoffWorkspace(w, httptest.NewRequest("GET", fmt.Sprintf("/workspace?section=ports&revision=%d", rev+1), nil))
	if w.Code != 409 {
		t.Fatal("stale revision was accepted")
	}
}
func queryCategory(q string) string {
	if q == "" {
		return "summary"
	}
	if len(q) > 14 && q[9:14] == "ports" {
		return "port_page"
	}
	return "connection_page"
}
