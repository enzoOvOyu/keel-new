package bom

import (
	"strings"
)

func ApplyTemplate(w Workbook, defaults, approved []Profile, reference string) []Profile {
	out := append([]Profile{}, defaults...)
	for j, p := range out {
		for _, saved := range approved {
			if p.Sheet != saved.Sheet {
				continue
			}
			var sh *Sheet
			for k := range w.Sheets {
				if w.Sheets[k].Name == p.Sheet {
					sh = &w.Sheets[k]
				}
			}
			if sh == nil {
				continue
			}
			rows := map[int]int{}
			for _, c := range sh.Cells {
				if c.Row > 50 || c.Sensitive {
					continue
				}
				for _, col := range saved.Columns {
					if strings.EqualFold(strings.TrimSpace(c.Value), strings.TrimSpace(col.Header)) {
						rows[c.Row]++
					}
				}
			}
			best, score := 0, 0
			for r, n := range rows {
				if n > score || (n == score && r < best) {
					best, score = r, n
				}
			}
			if score == 0 {
				continue
			}
			mapped := []Column{}
			complete := true
			for _, col := range saved.Columns {
				matches := []Cell{}
				for _, c := range sh.Cells {
					if c.Row == best && strings.EqualFold(strings.TrimSpace(c.Value), strings.TrimSpace(col.Header)) {
						matches = append(matches, c)
					}
				}
				if len(matches) != 1 {
					complete = false
					break
				}
				col.Index = matches[0].Column
				mapped = append(mapped, col)
			}
			if complete {
				p.Purpose = saved.Purpose
				p.Entity = saved.Entity
				p.HeaderRows = []int{best}
				p.Start = best + 1
				p.Columns = mapped
				p.Confirmed = true
				p.Reason = "Approved project template " + reference + "; exact unique headers, final batch review still required"
				out[j] = p
			}
		}
	}
	return out
}
