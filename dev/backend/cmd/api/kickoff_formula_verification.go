package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
)

// Local-only result path: text identities stay in the private checkpoint, never
// in the model tool response. One isolated recalculation covers all formula rows.
func (s *Server) kickoffVerifyWorkbook(ctx context.Context, source string, revision int, wb bom.Workbook) (map[string]ai.KickoffVerifiedCell, error) {
	id, err := s.queueEngineeringCalculation(ctx, source, engineering.CalculationRequest{ExpectedRevision: revision}, "engineering-agent")
	if err != nil {
		return nil, err
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			var status string
			if err = s.db.QueryRow(ctx, `SELECT status FROM eng_calculation_run WHERE id=$1 AND project_id=$2`, id, s.project).Scan(&status); err != nil {
				return nil, err
			}
			if status == "pending" || status == "running" {
				continue
			}
			if status != "succeeded" && status != "partial" {
				return nil, fmt.Errorf("WORKBOOK_RECALCULATION_FAILED")
			}
			allowed := map[string]bool{}
			for _, sheet := range wb.Sheets {
				for _, cell := range sheet.Cells {
					if cell.Formula != "" && !cell.Sensitive && !bom.Sensitive(cell.Value) {
						allowed[sheet.Name+"!"+cell.Ref] = true
					}
				}
			}
			rows, err := s.db.Query(ctx, `SELECT sheet,cell,recalculated_value FROM eng_calculation_formula WHERE run_id=$1 AND compatibility='compatible' AND error_code IS NULL AND recalculated_value IS NOT NULL`, id)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			out := map[string]ai.KickoffVerifiedCell{}
			for rows.Next() {
				var sheet, cell string
				var raw []byte
				if err = rows.Scan(&sheet, &cell, &raw); err != nil {
					return nil, err
				}
				if !allowed[sheet+"!"+cell] {
					continue
				}
				var v engineering.TypedValue
				if err = json.Unmarshal(raw, &v); err != nil {
					return nil, err
				}
				value := ""
				if v.Decimal != nil {
					value = *v.Decimal
				} else if v.Text != nil {
					value = *v.Text
				}
				if value != "" && !bom.Sensitive(value) {
					out[sheet+"!"+cell] = ai.KickoffVerifiedCell{Value: value, RunID: id}
				}
			}
			return out, rows.Err()
		}
	}
}
