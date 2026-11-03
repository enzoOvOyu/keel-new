package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

func TestKickoffScopeCovered(t *testing.T) {
	for _, tc := range []struct {
		name          string
		prior, wanted []string
		before, after map[string]int
		covered       bool
	}{
		{"same scope", []string{"a", "b"}, []string{"b", "a"}, map[string]int{"rack": 1}, map[string]int{"rack": 1}, true},
		{"covered subset", []string{"a", "b"}, []string{"a"}, nil, nil, true},
		{"new device", []string{"a"}, []string{"a", "b"}, nil, nil, false},
		{"changed input", []string{"a"}, []string{"a"}, map[string]int{"rack": 1}, map[string]int{"rack": 2}, false},
		{"different table", []string{"a"}, []string{"a"}, map[string]int{"rack": 1}, map[string]int{"network": 1}, false},
		{"new input", []string{"a"}, []string{"a"}, nil, map[string]int{"network": 1}, false},
		{"empty selection", []string{"a"}, nil, nil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := kickoffScopeCovered(tc.prior, tc.wanted, tc.before, tc.after); got != tc.covered {
				t.Fatalf("coverage=%v want %v", got, tc.covered)
			}
		})
	}
}
func TestKickoffScopeCoverageExistingReadOnly(t *testing.T) {
	if os.Getenv("KEEL_KICKOFF_SCOPE_READONLY") != "1" {
		t.Skip("explicit read-only existing-data opt-in")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid private DB config")
	}
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	cfg.MaxConns = 1
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("read-only database unavailable")
	}
	defer db.Close()
	project := os.Getenv("KEEL_KICKOFF_PROJECT")
	var head string
	var rawIDs, rawVersions []byte
	err = db.QueryRow(ctx, `SELECT p.model_revision::text,
 COALESCE((SELECT jsonb_agg(o.entity_id::text) FROM kickoff_publication_object o WHERE o.publication_id=p.id AND o.kind='device'),'[]'::jsonb),
 COALESCE((SELECT jsonb_object_agg(i.table_id,i.version) FROM kickoff_publication_input i WHERE i.publication_id=p.id),'{}'::jsonb)
 FROM kickoff_publication p WHERE p.project_id=$1 AND p.state='published' ORDER BY p.revision DESC LIMIT 1`, project).Scan(&head, &rawIDs, &rawVersions)
	if err != nil {
		t.Fatal("published scope unavailable")
	}
	var ids []string
	var versions map[string]int
	if json.Unmarshal(rawIDs, &ids) != nil || json.Unmarshal(rawVersions, &versions) != nil || len(ids) == 0 {
		t.Fatal("invalid prior scope")
	}
	state, err := kickoffCoveredPublication(ctx, db, project, head, ids, versions)
	if err != nil || state == "" {
		t.Fatal("same revision/input scope not recognized")
	}
	state, err = kickoffCoveredPublication(ctx, db, project, head, ids[:1], versions)
	if err != nil || state == "" {
		t.Fatal("published subset not recognized")
	}
	state, err = kickoffCoveredPublication(ctx, db, project, "00000000-0000-0000-0000-000000000000", ids, versions)
	if err != nil || state != "" {
		t.Fatal("different revision blocked")
	}
	versions["synthetic_unpublished_input"] = 1
	state, err = kickoffCoveredPublication(ctx, db, project, head, ids, versions)
	if err != nil || state != "" {
		t.Fatal("new input blocked")
	}
	var readonly string
	if err = db.QueryRow(ctx, `SHOW default_transaction_read_only`).Scan(&readonly); err != nil || readonly != "on" {
		t.Fatal("read-only boundary lost")
	}
	server := &Server{db: db, project: project}
	var workspaceRevision int
	if err = db.QueryRow(ctx, `SELECT revision FROM kickoff_workspace WHERE project_id=$1`, project).Scan(&workspaceRevision); err != nil {
		t.Fatal(err)
	}
	read, err := server.kickoffReadSnapshot(ctx, db, workspaceRevision)
	if err != nil {
		t.Fatal(err)
	}
	options, err := server.kickoffPublicationDeviceOptions(ctx, db, read.model, read.inputs, read.detail(), workspaceRevision)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, v := range options.Devices {
		counts[v.State]++
	}
	if len(options.Devices) != len(read.summary.Devices) {
		t.Fatal("incomplete options")
	}
	raw, _ := json.Marshal(map[string]any{"counts": counts, "total": len(options.Devices), "database_readonly": true})
	if err = os.WriteFile("/tmp/kickoff-publication-options-check.json", raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("options counts=%v", counts)
	t.Log("same-head coverage, subset, changed revision/input SQL and device options verified with read-only database")
}
