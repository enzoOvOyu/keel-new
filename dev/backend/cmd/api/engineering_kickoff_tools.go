package main

import (
	"bytes"
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/xuri/excelize/v2"
	"io"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"strings"
	"time"
)

func (s *Server) kickoffToolHooks(ctx context.Context, source string, revision int, wb bom.Workbook) ai.EngineeringToolHooks {
	hooks := ai.EngineeringToolHooks{}
	hooks.VerifyWorkbook = func(ctx context.Context) (map[string]ai.KickoffVerifiedCell, error) {
		return s.kickoffVerifyWorkbook(ctx, source, revision, wb)
	}
	hidden := map[string]bool{}
	for _, sheet := range engineeringSafeWorkbook(wb).Sheets {
		for _, cell := range sheet.Cells {
			if cell.Sensitive || strings.HasPrefix(cell.Value, "[IDENTITY]") || strings.HasPrefix(cell.Value, "[LABEL-") {
				hidden[sheet.Name+"!"+cell.Ref] = true
			}
		}
	}
	var key string
	if s.objects != nil && s.db.QueryRow(ctx, `SELECT b.object_key FROM eng_source s JOIN bom_import_batch b ON b.id=s.import_id WHERE s.id=$1 AND s.project_id=$2`, source, s.project).Scan(&key) == nil {
		if object, e := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{}); e == nil {
			data, err := io.ReadAll(io.LimitReader(object, 20*1024*1024+1))
			object.Close()
			if err == nil && len(data) <= 20*1024*1024 {
				if book, err := excelize.OpenReader(bytes.NewReader(data)); err == nil {
					hooks.Names = book.GetDefinedName()
					book.Close()
				}
			}
		}
	}
	hooks.ProposeRule = func(ctx context.Context, rule engineering.CalculationRule) (engineering.RuleView, error) {
		// Ignore proposed publication authority and identity. The agent can only
		// create a new project rule candidate with deterministically checked cases.
		rule.ID = ""
		rule.Version = 1
		rule.ScopeType = "project"
		rule.ScopeID = s.project
		rule.ApprovalStatus = "draft"
		rule.ReviewDecisionID = nil
		return s.saveEngineeringRule(ctx, engineering.RuleDraft{Rule: rule, Reason: "Agent提出规则；案例不代表独立Ground Truth"}, "engineering-agent")
	}
	hooks.Calculate = func(ctx context.Context, rule *engineering.VersionReference, cells []engineering.FormulaCell, inputs []engineering.NamedValue) (engineering.CalculationPage, error) {
		id, e := s.queueEngineeringCalculation(ctx, source, engineering.CalculationRequest{ExpectedRevision: revision, Rule: rule, RequestedCells: cells, Inputs: inputs}, "engineering-agent")
		if e != nil {
			return engineering.CalculationPage{}, e
		}
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return engineering.CalculationPage{}, ctx.Err()
			case <-ticker.C:
				page, err := s.engineeringCalculation(ctx, id, 0, 32)
				if err != nil {
					return page, err
				}
				if page.Run.Status == "pending" || page.Run.Status == "running" {
					continue
				}
				for i := range page.Run.Formulas {
					f := &page.Run.Formulas[i]
					if hidden[f.Cell.SheetID+"!"+f.Cell.Address] {
						f.CachedValue = nil
						f.RecalculatedValue = nil
					}
					f.Expression = engineeringLiteral.ReplaceAllString(f.Expression, `"[TEXT]"`)
					if f.CachedValue != nil && f.CachedValue.Kind == "text" {
						f.CachedValue = nil
					}
					if f.RecalculatedValue != nil && f.RecalculatedValue.Kind == "text" {
						f.RecalculatedValue = nil
					}
				}
				return page, nil
			}
		}
	}
	return hooks
}
