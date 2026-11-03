package cmdb

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInventoryRejectsInvalidLimit(t *testing.T) {
	for _, limit := range []int{-1, 0, 2001} {
		if _, err := (Service{}).InventoryPage(context.Background(), Scope{}, "", limit); err == nil {
			t.Fatalf("accepted limit %d", limit)
		}
	}
}

// Read-only verification against existing data; no assets, publications or AI calls.
func TestInventoryPagesReadOnly(t *testing.T) {
	project := os.Getenv("KEEL_INVENTORY_PROJECT")
	if project == "" {
		t.Skip("requires explicit existing project")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := Service{DB: db}
	scope := Scope{Projects: []string{project}}
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		r, err := service.InventoryPage(ctx, scope, cursor, 2000)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for kind, rows := range r["data"].(map[string][]map[string]any) {
			if kind == "rack_placements" {
				continue
			}
			for _, row := range rows {
				id := stringValue(row["id"])
				if seen[id] {
					t.Fatalf("duplicate %s", id)
				}
				seen[id] = true
				if row["project_id"] != project {
					t.Fatal("cross-project row")
				}
			}
		}
		page := r["page"].(map[string]any)
		more := page["has_more"].(bool)
		if r["truncated"] != more {
			t.Fatal("inaccurate truncation")
		}
		if !more {
			break
		}
		next := page["next_cursor"].(string)
		if next <= cursor {
			t.Fatal("cursor not progressing")
		}
		cursor = next
	}
	schema, err := service.Active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	kinds := []string{}
	for _, typ := range schema.Types {
		for _, base := range []string{"server", "switch", "gpu", "nic", "port", "optic", "cable", "rack"} {
			if schema.IsA(typ.Name, base) {
				kinds = append(kinds, typ.Name)
				break
			}
		}
	}
	var expected int
	err = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_entity e WHERE lifecycle_state<>'retired' AND `+Visible("e")+` AND asset_type=ANY($3)`, append(scope.Args(), kinds)...).Scan(&expected)
	if err != nil {
		t.Fatal(err)
	}
	if expected == 0 {
		t.Fatal("explicit project contains no inventory; cannot validate pagination")
	}
	if len(seen) != expected {
		t.Fatalf("got %d want %d", len(seen), expected)
	}
	t.Logf("%d pages, %d unique authorized assets", pages, len(seen))
	var asset string
	err = db.QueryRow(ctx, `SELECT e.entity_id::text FROM eng_evidence e JOIN kickoff_publication p ON p.model_revision=e.revision_id JOIN cmdb_entity c ON c.item_uid::text=e.entity_id::text WHERE p.project_id=$1 LIMIT 1`, project).Scan(&asset)
	if err != nil {
		t.Fatal(err)
	}
	sources, err := service.Provenance(ctx, scope, asset)
	if err != nil {
		t.Fatal(err)
	}
	resolved := 0
	for _, source := range sources {
		if strings.HasPrefix(stringValue(source["source_ref"]), "engineering:") {
			if source["href"] != "/p/"+project+"/kickoff/publish?publication="+strings.TrimPrefix(stringValue(source["source_ref"]), "engineering:") {
				t.Fatal("wrong engineering link")
			}
			if source["sheet"] != nil && source["cell"] != nil {
				resolved++
			}
		}
	}
	if resolved == 0 {
		t.Fatal("engineering evidence was not resolved")
	}
	t.Logf("%d sources resolved with sheet/cell", resolved)
}
