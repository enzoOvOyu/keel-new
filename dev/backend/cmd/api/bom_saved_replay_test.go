package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
)

// Explicit offline evidence replay: local files only, no provider or database
// calls. Private XLSX/answers are never embedded in fixtures or test output.
func TestLocalSavedWorkbookRepair(t *testing.T) {
	sourceDir, evidenceDir := os.Getenv("KEEL_BOM_SOURCE_DIR"), os.Getenv("KEEL_BOM_SAVED_EVIDENCE")
	if sourceDir == "" || evidenceDir == "" {
		t.Skip("explicit private offline source/evidence paths required")
	}
	for _, name := range []string{"bomv1.xlsx", "netbom.xlsx"} {
		t.Run(name, func(t *testing.T) {
			data, e := os.ReadFile(filepath.Join(sourceDir, name))
			if e != nil {
				t.Fatal(e)
			}
			wb, e := bom.Parse(data)
			if e != nil {
				t.Fatal(e)
			}
			data, e = os.ReadFile(filepath.Join(evidenceDir, name+".fixed_targeted_5.json"))
			if e != nil {
				t.Fatal(e)
			}
			var saved bom.Workbench
			if e = json.Unmarshal(data, &saved); e != nil {
				t.Fatal(e)
			}
			sheets := map[string]bom.Sheet{}
			for _, s := range wb.Sheets {
				sheets[s.Name] = s
			}
			b := bom.PlanningSourceRows(wb)
			accepted, rejected := 0, 0
			for _, info := range saved.Sheets {
				s := sheets[info.Name]
				p, err := ai.RestoreExtraction(s, info.Plan)
				if err != nil {
					rejected++
					t.Logf("retained for correction: sheet=%s error=%v", info.Name, err)
					continue
				}
				b.Records = applySheetPlan(b.Records, s, p, nil)
				accepted++
			}
			network, cables, nsRow := 0, 0, 0
			for _, r := range b.Records {
				if r.Item.Category == "cable" {
					cables++
					if r.Sheet == "Cabling_NS" && r.Row == 3 {
						nsRow++
					}
				}
				if r.Sheet == "Network Information" && r.Item.Category == "switch" {
					network++
				}
			}
			if network != 189 {
				t.Fatalf("source switch inventory still incomplete: %d/189", network)
			}
			if cables != 7154 || nsRow != 1 {
				t.Fatalf("physical source count wrong: cables=%d/7154 ns_row3=%d/1", cables, nsRow)
			}
			bom.LinkRecognizedRecords(&b)
			seen := map[string]bool{}
			before := map[string]bom.ImportRecord{}
			for _, r := range b.Records {
				if seen[r.ID] {
					t.Fatal("duplicate candidate identity after linking")
				}
				seen[r.ID] = true
				before[r.ID] = r
			}
			// One deterministic replay, not another model or publication run.
			bom.LinkRecognizedRecords(&b)
			for _, r := range b.Records {
				if !seen[r.ID] {
					t.Fatal("link replay changed candidate identity")
				}
				delete(seen, r.ID)
			}
			if len(seen) != 0 {
				counts := map[string]int{}
				for id := range seen {
					r := before[id]
					counts[r.Item.Category+":"+r.Method+":"+r.Sheet]++
				}
				t.Fatalf("link replay removed candidates: %v", counts)
			}
			audit := bom.NewWorkbookIR(wb).AuditCoverage(&b)
			if audit.TotalCells != audit.RetainedCells {
				t.Fatalf("lost immutable cells: %d/%d", audit.RetainedCells, audit.TotalCells)
			}
			linked, missingParents := 0, 0
			known := map[string]bool{}
			for _, r := range b.Records {
				known[r.ID] = true
			}
			for _, r := range b.Records {
				if r.Item.Category == "cable" {
					if r.Item.A != "" && r.Item.B != "" && known[r.Item.A] && known[r.Item.B] {
						linked++
					}
					if r.Item.Parent != "" && !known[r.Item.Parent] {
						missingParents++
					}
				}
			}
			if linked != 7150 {
				t.Fatalf("resolvable source cables missing physical port endpoints: %d/7150", linked)
			}
			if missingParents != 0 {
				t.Fatalf("phantom cable parents still block dependencies: %d", missingParents)
			}
			t.Logf("switches=%d; cables=7154; ns_row3=1; linked_cables=%d; missing_cable_parents=%d; accepted_saved_sheets=%d; rejected_saved_sheets=%d; retained_cells=%d/%d; records=%d; graph replay stable", network, linked, missingParents, accepted, rejected, audit.RetainedCells, audit.TotalCells, len(b.Records))
			data, e = os.ReadFile(filepath.Join(evidenceDir, name+".fixed_full.json"))
			if e != nil {
				t.Fatal(e)
			}
			var legacy bom.Workbench
			if e = json.Unmarshal(data, &legacy); e != nil {
				t.Fatal(e)
			}
			bom.LinkRecognizedRecords(&legacy)
			seen = map[string]bool{}
			for _, r := range legacy.Records {
				if seen[r.ID] {
					t.Fatal("first-pass duplicate identity not repaired")
				}
				seen[r.ID] = true
			}
		})
	}
}
