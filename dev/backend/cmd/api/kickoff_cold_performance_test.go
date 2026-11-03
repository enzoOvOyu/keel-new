package main

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/kickoff"
	"os"
	"strings"
	"testing"
	"time"
)

type coldQuery struct {
	engineeringQuery
	t *testing.T
}
type coldRow struct {
	pgx.Row
	t     *testing.T
	label string
	start time.Time
}

func (r coldRow) Scan(values ...any) error {
	err := r.Row.Scan(values...)
	r.t.Logf("query %s: %s", r.label, time.Since(r.start))
	return err
}
func (q coldQuery) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	label := sql
	for _, table := range []string{"eng_equipment", "eng_assembly", "eng_facility", "eng_placement", "eng_port", "eng_connection", "eng_optic_demand"} {
		if strings.Contains(sql, "FROM "+table+" ") {
			label = table
			break
		}
	}
	if len(label) > 100 {
		label = label[:100]
	}
	start := time.Now()
	return coldRow{q.engineeringQuery.QueryRow(ctx, sql, args...), q.t, label, start}
}
func TestKickoffColdReadProfile(t *testing.T) {
	if os.Getenv("KEEL_KICKOFF_READONLY") != "1" {
		t.Skip("explicit real-data read-only opt-in")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid DB configuration")
	}
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	ctx := context.Background()
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("DB unavailable")
	}
	defer db.Close()
	s := &Server{db: db, project: os.Getenv("KEEL_KICKOFF_PROJECT")}
	q := coldQuery{db, t}
	at := time.Now()
	m, ev, source, err := s.kickoffModel(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("model %s", time.Since(at))
	at = time.Now()
	inputs, err := loadKickoffInputs(ctx, q, s.project)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("inputs %s", time.Since(at))
	at = time.Now()
	inputs = kickoff.ResolveReferences(m, inputs)
	v := kickoff.ProjectSummary(m, ev, source, inputs)
	t.Logf("summary %s; devices=%d", time.Since(at), len(v.Devices))
	if len(v.Devices) == 0 {
		t.Fatal("expected populated fixture")
	}
}
