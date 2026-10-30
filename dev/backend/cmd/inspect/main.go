package main

import (
	"encoding/json"
	"keel.local/dev/internal/bom"
	"os"
)

func main() {
	b, e := os.ReadFile(os.Args[1])
	if e != nil {
		panic("input unavailable")
	}
	w, e := bom.Parse(b)
	if e != nil {
		panic(e)
	}
	type summary struct {
		Name     string `json:"sheet_name"`
		Cells    int    `json:"nonempty_cells"`
		Merges   int    `json:"merge_regions"`
		Formulas int    `json:"formulas"`
		Redacted int    `json:"redacted_cells"`
		Hidden   int    `json:"hidden_cells"`
	}
	out := []summary{}
	for _, s := range w.Sheets {
		x := summary{Name: s.Name, Cells: len(s.Cells), Merges: len(s.Merges)}
		for _, c := range s.Cells {
			if c.Formula != "" {
				x.Formulas++
			}
			if c.Sensitive {
				x.Redacted++
			}
			if c.Hidden {
				x.Hidden++
			}
		}
		out = append(out, x)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"parser": "excelize-structure/v1", "sheets": out, "candidate_count": len(bom.Draft(w)), "mode": "rules", "formal_commit": "NOT_RUN"})
}
