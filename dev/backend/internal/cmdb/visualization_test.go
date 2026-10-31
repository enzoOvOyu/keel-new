package cmdb

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

// Read existing synthetic fixture records and roll back the locking transaction.
// No assets or ChangeSets are created or published.
func TestVisualizationAndRelocationBoundaries(t *testing.T) {
	if os.Getenv("KEEL_CMDB_INTEGRATION") != "1" {
		t.Skip("explicit local database run required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := Service{DB: db}
	invisible, err := service.Visualization(ctx, Scope{Projects: []string{"nonexistent-project"}}, "", "")
	if err != nil || len(invisible.Assets) != 0 || len(invisible.Cables) != 0 {
		t.Fatalf("scope isolation: %v", err)
	}
	snapshot, err := service.Visualization(ctx, Scope{Projects: []string{"proj-b300-demo"}}, "rack-0001", "")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]VisualAsset{}
	for _, a := range snapshot.Assets {
		byID[a.ID] = a
	}
	if len(snapshot.Cables) == 0 {
		t.Fatal("fixture cabling absent")
	}
	for _, c := range snapshot.Cables {
		if c.DeviceA != "" {
			if _, ok := byID[c.A]; !ok {
				t.Fatal("missing visible endpoint A")
			}
		}
		if c.DeviceB != "" {
			if _, ok := byID[c.B]; !ok {
				t.Fatal("missing visible endpoint B")
			}
		}
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var raw []byte
	if err = tx.QueryRow(ctx, "SELECT payload FROM cmdb_entity WHERE item_uid='srv-0001'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var attrs map[string]any
	json.Unmarshal(raw, &attrs)
	for _, test := range []struct {
		name  string
		value float64
		pass  bool
	}{{"same confirmed U", 33, true}, {"zero", 0, false}, {"beyond cabinet", 40, false}, {"occupied U", 27, false}} {
		t.Run(test.name, func(t *testing.T) {
			err := service.validateRelocation(ctx, tx, "proj-b300-demo", "srv-0001", attrs, map[string]any{"u_start": test.value})
			if (err == nil) != test.pass {
				t.Fatalf("pass=%v error=%v", test.pass, err)
			}
		})
	}
}

// Exercise the production read adapter only; no writes or fixture loading.
func TestVisualizationFullProjectionBoundaries(t *testing.T) {
	if os.Getenv("KEEL_CMDB_INTEGRATION") != "1" {
		t.Skip("explicit local database run required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := Service{DB: db}
	scope := Scope{Projects: []string{"proj-b300-demo"}}
	full, err := service.VisualizationDetail(ctx, scope, "", "", true, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if full.SnapshotID == "" || full.ReadAt.IsZero() || full.Truncated || !full.Coverage.Complete {
		t.Fatalf("unexpected coverage: %+v", full.Coverage)
	}
	byID := map[string]VisualAsset{}
	ports := 0
	for _, asset := range full.Assets {
		byID[asset.ID] = asset
		if asset.Type == "port" {
			ports++
		}
	}
	if ports == 0 || len(full.Relationships) == 0 {
		t.Fatal("full projection omitted ports or typed relationships")
	}
	for _, relation := range full.Relationships {
		if byID[relation.From].ID == "" || byID[relation.To].ID == "" {
			t.Fatal("relationship exposed outside projection")
		}
	}
	for _, cable := range full.Cables {
		for _, id := range []string{cable.A, cable.B, cable.DeviceA, cable.DeviceB} {
			if id != "" && byID[id].ID == "" {
				t.Fatal("endpoint exposed outside authorized projection")
			}
		}
	}
	capped, err := service.VisualizationDetail(ctx, scope, "", "", true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !capped.Truncated || capped.Coverage.Complete || len(capped.Assets) > 1 || capped.Total <= 1 {
		t.Fatal("source cap not reported")
	}
	hidden, err := service.VisualizationDetail(ctx, Scope{Projects: []string{"not-authorized"}}, "", "", true, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden.Assets)+len(hidden.Cables)+len(hidden.Relationships) > 0 {
		t.Fatal("cross-project data leaked")
	}
}
