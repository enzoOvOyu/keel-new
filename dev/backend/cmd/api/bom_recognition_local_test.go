package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
)

// Explicitly opt-in, real-provider evidence collection. Source files, credentials
// and reports remain outside Git. Reads the configured test batches, writes no DB
// state, starts no workers, and never approves or submits a candidate.
func TestLocalRecognitionEvidence(t *testing.T) {
	manifest := os.Getenv("KEEL_BOM_LOCAL_MANIFEST")
	if manifest == "" {
		t.Skip("explicit local workbook/provider manifest required")
	}
	var config struct {
		Output string
		Rounds int
		Cases  []struct {
			Name, Path, ImportID string
			Sheets               []string
		}
	}
	raw, e := os.ReadFile(manifest)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &config); e != nil {
		t.Fatal(e)
	}
	if config.Rounds < 1 || config.Rounds > 4 {
		t.Fatal("expected 1..4 explicit rounds")
	}
	if e = os.MkdirAll(config.Output, 0700); e != nil {
		t.Fatal(e)
	}
	for _, c := range config.Cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			s := Server{db: db}
			provider, _, e := s.workbenchProvider(ctx, c.ImportID)
			if e != nil {
				t.Fatal(e)
			}
			var analysis, sheets []byte
			if e = db.QueryRow(ctx, "SELECT workbench->'analysis', workbench->'sheets' FROM bom_import_batch WHERE id=$1", c.ImportID).Scan(&analysis, &sheets); e != nil {
				t.Fatal(e)
			}
			var planning ai.PlanningContext
			if e = json.Unmarshal(analysis, &planning); e != nil {
				t.Fatal(e)
			}
			var saved []bom.RecognitionSheet
			if e = json.Unmarshal(sheets, &saved); e != nil {
				t.Fatal(e)
			}
			data, e := os.ReadFile(c.Path)
			if e != nil {
				t.Fatal(e)
			}
			wb, e := bom.Parse(data)
			if e != nil {
				t.Fatal(e)
			}
			safe := bom.Workbook{}
			for _, sh := range wb.Sheets {
				safe.Sheets = append(safe.Sheets, safeSheet(sh))
			}
			ir := bom.NewWorkbookIR(safe)
			b := bom.PlanningSourceRows(wb)
			plans := map[string]ai.SheetPlan{}
			for _, info := range saved {
				for _, source := range wb.Sheets {
					if source.Name == info.Name {
						if p, err := ai.RestoreExtractionBindings(source, info.Plan); err == nil {
							plans[info.Name] = p
						}
					}
				}
			}
			for round := 1; round <= config.Rounds; round++ {
				for _, name := range c.Sheets {
					var source bom.Sheet
					for _, sh := range wb.Sheets {
						if sh.Name == name {
							source = sh
						}
					}
					if source.Name == "" {
						t.Fatal("unknown sheet in manifest")
					}
					var prior *ai.SheetPlan
					if p, ok := plans[name]; ok {
						prior = &p
					}
					output := filepath.Join(config.Output, fmt.Sprintf("%s-r%d-%s.json", c.Name, round, name))
					var result struct {
						Plan               ai.SheetPlan
						Steps              []bom.AgentStep
						Error              string
						Calls              int
						Records, UniqueIDs int
					}
					// Resume the evidence run without repeating successful paid calls.
					cached, readErr := os.ReadFile(output)
					cacheOK := readErr == nil && json.Unmarshal(cached, &result) == nil && result.Error == "" && ai.CheckPlanRevision(source, result.Plan, prior) == nil
					if !cacheOK {
						result.Records = 0
						result.UniqueIDs = 0
						adapter := *provider
						adapter.Calls = 0
						p, err := adapter.PlanSheetWithWorkbook(ctx, ir, name, planning, prior)
						if err == nil {
							err = ai.CheckPlanRevision(source, p, prior)
						}
						result.Plan = p
						result.Steps = p.Steps
						result.Calls = adapter.Calls
						if err != nil {
							result.Error = err.Error()
						} else {
							result.Error = ""
						}
					}
					if result.Error == "" {
						plans[name] = result.Plan
						b.Records = applySheetPlan(b.Records, source, result.Plan, nil)
						bom.LinkRecognizedRecords(&b)
						result.Records = 0
						seen := map[string]bool{}
						for _, r := range b.Records {
							if r.Sheet == name {
								result.Records++
								seen[r.ID] = true
							}
						}
						result.UniqueIDs = len(seen)
					}
					encoded, _ := json.Marshal(result)
					if result.Error != "" {
						output += ".failed.json"
					}
					if e = os.WriteFile(output, encoded, 0600); e != nil {
						t.Fatal(e)
					}
					t.Logf("round=%d sheet=%s calls=%d records=%d unique=%d error=%s", round, name, result.Calls, result.Records, result.UniqueIDs, result.Error)
					if result.Error != "" {
						t.Errorf("source plan failed: %s", result.Error)
					}
					if result.Records != result.UniqueIDs {
						t.Errorf("duplicate candidate IDs")
					}
				}
				bom.CompactWorkbench(&b)
				encoded, _ := json.Marshal(b)
				if e = os.WriteFile(filepath.Join(config.Output, fmt.Sprintf("%s-r%d-workbench.json", c.Name, round)), encoded, 0600); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
