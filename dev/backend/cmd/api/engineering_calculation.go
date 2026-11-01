package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"io"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"os"
	"time"
)

func (s *Server) engineeringCalculatedWorkbook(ctx context.Context, source string, w bom.Workbook) (bom.Workbook, map[string]string, error) {
	out := bom.Workbook{Sheets: append([]bom.Sheet{}, w.Sheets...)}
	references := map[string]string{}
	rows, e := s.db.Query(ctx, `SELECT DISTINCT ON(f.sheet,f.cell) f.sheet,f.cell,r.id::text,f.recalculated_value,f.compatibility FROM eng_calculation_formula f JOIN eng_calculation_run r ON r.id=f.run_id WHERE r.source_id=$1 AND r.project_id=$2 AND r.provider_id='libreoffice-isolated' AND r.status IN('succeeded','partial') ORDER BY f.sheet,f.cell,r.created_at DESC`, source, s.project)
	if e != nil {
		return out, references, e
	}
	defer rows.Close()
	values := map[string]engineering.TypedValue{}
	for rows.Next() {
		var sheet, cell, run, status string
		var raw []byte
		if e = rows.Scan(&sheet, &cell, &run, &raw, &status); e != nil {
			return out, references, e
		}
		if status != "compatible" || raw == nil {
			continue
		}
		var value engineering.TypedValue
		if e = json.Unmarshal(raw, &value); e != nil {
			return out, references, e
		}
		values[sheet+"!"+cell] = value
		references[sheet+"!"+cell] = run
	}
	if e = rows.Err(); e != nil {
		return out, references, e
	}
	for i := range out.Sheets {
		out.Sheets[i].Cells = append([]bom.Cell{}, out.Sheets[i].Cells...)
		for j := range out.Sheets[i].Cells {
			cell := &out.Sheets[i].Cells[j]
			if value, ok := values[out.Sheets[i].Name+"!"+cell.Ref]; ok && cell.Formula != "" {
				switch value.Kind {
				case "decimal":
					cell.Value = *value.Decimal
				case "text":
					cell.Value = *value.Text
				case "boolean":
					if *value.Boolean {
						cell.Value = "TRUE"
					} else {
						cell.Value = "FALSE"
					}
				}
			}
		}
	}
	return out, references, nil
}

func engineeringCalculationEvidence(x *engineering.Extraction, refs map[string]string) {
	targets := map[string]string{}
	for _, e := range x.Model.Equipment {
		targets[e.EntityID] = engineering.EquipmentTarget
	}
	for _, e := range x.Model.Assemblies {
		targets[e.EntityID] = engineering.EquipmentTarget
	}
	for _, e := range x.Model.Facilities {
		targets[e.EntityID] = engineering.FacilityTarget
	}
	for _, e := range x.Model.Placements {
		targets[e.EntityID] = engineering.PlacementTarget
	}
	for i := range x.Evidence {
		e := &x.Evidence[i]
		if !e.Formula {
			continue
		}
		if run, ok := refs[e.Sheet+"!"+e.Cell]; ok {
			e.CalculationRunID = &run
		} else {
			x.Issues = append(x.Issues, engineering.ValidationIssue{ID: uuid.NewString(), EntityID: e.EntityID, Code: "FORMULA_RECALCULATION_REQUIRED", Severity: "BLOCK", ModuleID: "workbook_intelligence", Field: e.Field, Message: "公式字段没有兼容的独立重算结果；缓存仅保留为来源证据", PublishTarget: targets[e.EntityID], EvidenceIDs: []string{e.ID}})
		}
	}
}

func (s *Server) queueEngineeringCalculation(ctx context.Context, source string, q engineering.CalculationRequest, actor string) (string, error) {
	if len(q.RequestedCells) > 1000 {
		return "", fmt.Errorf("CALC_REQUEST_LIMIT")
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	var version int
	var revision *string
	var hash string
	if e = tx.QueryRow(ctx, `SELECT s.revision,s.head_revision_id::text,b.content_sha256 FROM eng_source s JOIN bom_import_batch b ON b.id=s.import_id WHERE s.id::text=$1 AND s.project_id=$2 FOR UPDATE OF s`, source, s.project).Scan(&version, &revision, &hash); e != nil {
		return "", e
	}
	if version != q.ExpectedRevision {
		return "", errors.New("REVISION_CONFLICT")
	}
	if revision == nil {
		return "", errors.New("CALC_SOURCE_REVISION_REQUIRED")
	}
	provider, module := "libreoffice-isolated", "workbook_intelligence"
	var ruleID *string
	var ruleVersion *int
	if q.Rule != nil {
		view, err := engineeringRule(ctx, tx, s.project, q.Rule.ID, q.Rule.Version)
		if err != nil {
			return "", err
		}
		if view.Rule.ApprovalStatus != "approved" {
			return "", errors.New("RULE_APPROVAL_REQUIRED")
		}
		if _, err = engineering.EvaluateRule(view.Rule, q.Inputs); err != nil {
			return "", err
		}
		provider = "engineering-expression"
		module = string(view.Rule.Domain)
		ruleID = &q.Rule.ID
		ruleVersion = &q.Rule.Version
	} else if len(q.Inputs) > 0 {
		return "", errors.New("WORKBOOK_PARAMETER_BINDINGS_REQUIRED")
	}
	for _, value := range q.Inputs {
		if value.Value.Validate() != nil {
			return "", errors.New("RULE_INPUT_TYPE")
		}
	}
	digest := engineering.Digest([]any{hash, *revision, q.Rule, q.RequestedCells, q.Inputs})
	id := uuid.NewString()
	e = tx.QueryRow(ctx, `INSERT INTO eng_calculation_run(id,project_id,source_id,revision_id,module_instance_id,provider_id,rule_id,rule_version,source_sha256,input_digest,status,actor,requested_cells) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',$11,$12) ON CONFLICT(source_id,input_digest) WHERE status IN('pending','running') DO UPDATE SET input_digest=excluded.input_digest RETURNING id::text`, id, s.project, source, *revision, module, provider, ruleID, ruleVersion, hash, digest, actor, jsonBytes(q.RequestedCells)).Scan(&id)
	if e != nil {
		return "", e
	}
	for _, v := range q.Inputs {
		if _, e = tx.Exec(ctx, `INSERT INTO eng_calculation_value VALUES($1,'input',$2,$3) ON CONFLICT DO NOTHING`, id, v.Name, jsonBytes(v.Value)); e != nil {
			return "", e
		}
	}
	return id, tx.Commit(ctx)
}

func (s *Server) engineeringCalculation(ctx context.Context, id string, offset, limit int) (engineering.CalculationPage, error) {
	page := engineering.CalculationPage{Offset: offset, Limit: limit}
	run := &page.Run
	run.Inputs = []engineering.NamedValue{}
	run.Outputs = []engineering.NamedValue{}
	run.Formulas = []engineering.FormulaEvidence{}
	run.Errors = []engineering.CalculationError{}
	var ruleID *string
	var version *int
	e := s.db.QueryRow(ctx, `SELECT r.id::text,r.project_id,r.module_instance_id,r.revision_id::text,r.provider_id,r.provider_version,r.rule_id::text,r.rule_version,r.source_sha256,r.input_digest,r.status,r.compatibility,r.started_at,r.finished_at,(r.revision_id IS DISTINCT FROM s.head_revision_id) FROM eng_calculation_run r JOIN eng_source s ON s.id=r.source_id WHERE r.id::text=$1 AND r.project_id=$2`, id, s.project).Scan(&run.ID, &run.ProjectID, &run.ModuleInstanceID, &run.RevisionID, &run.ProviderID, &run.ProviderVersion, &ruleID, &version, &run.SourceSHA256, &run.InputDigest, &run.Status, &run.Compatibility, &run.StartedAt, &run.FinishedAt, &page.Stale)
	if e != nil {
		return page, e
	}
	if ruleID != nil && version != nil {
		run.Rule = &engineering.VersionReference{ID: *ruleID, Version: *version}
	}
	rows, e := s.db.Query(ctx, `SELECT direction,name,value FROM eng_calculation_value WHERE run_id=$1 ORDER BY direction,name`, id)
	if e != nil {
		return page, e
	}
	for rows.Next() {
		var direction string
		var value engineering.NamedValue
		var raw []byte
		if e = rows.Scan(&direction, &value.Name, &raw); e != nil {
			break
		}
		if e = json.Unmarshal(raw, &value.Value); e != nil {
			break
		}
		if direction == "input" {
			run.Inputs = append(run.Inputs, value)
		} else {
			run.Outputs = append(run.Outputs, value)
		}
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return page, e
	}
	rows, e = s.db.Query(ctx, `SELECT code,message,sheet,cell FROM eng_calculation_error WHERE run_id=$1 ORDER BY position LIMIT 100`, id)
	if e != nil {
		return page, e
	}
	for rows.Next() {
		var issue engineering.CalculationError
		var sheet, cell *string
		if e = rows.Scan(&issue.Code, &issue.Message, &sheet, &cell); e != nil {
			break
		}
		if sheet != nil && cell != nil {
			issue.Cell = &engineering.FormulaCell{SheetID: *sheet, Address: *cell}
		}
		run.Errors = append(run.Errors, issue)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return page, e
	}
	if e = s.db.QueryRow(ctx, `SELECT count(*) FROM eng_calculation_formula WHERE run_id=$1`, id).Scan(&page.TotalFormulas); e != nil {
		return page, e
	}
	rows, e = s.db.Query(ctx, `SELECT sheet,cell,expression,cached_value,recalculated_value,dependency_status,compatibility,error_code FROM eng_calculation_formula WHERE run_id=$1 ORDER BY sheet,cell OFFSET $2 LIMIT $3`, id, offset, limit)
	if e != nil {
		return page, e
	}
	for rows.Next() {
		f := engineering.FormulaEvidence{Dependencies: []engineering.FormulaDependency{}}
		var cached, result []byte
		if e = rows.Scan(&f.Cell.SheetID, &f.Cell.Address, &f.Expression, &cached, &result, &f.DependencyStatus, &f.Compatibility, &f.ErrorCode); e != nil {
			break
		}
		if cached != nil {
			if e = json.Unmarshal(cached, &f.CachedValue); e != nil {
				break
			}
		}
		if result != nil {
			if e = json.Unmarshal(result, &f.RecalculatedValue); e != nil {
				break
			}
		}
		run.Formulas = append(run.Formulas, f)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return page, e
	}
	// Load just the selected page's dependency edges in one query.
	rows, e = s.db.Query(ctx, `SELECT d.sheet,d.cell,d.to_sheet,d.to_range,d.kind FROM eng_calculation_dependency d JOIN (SELECT sheet,cell FROM eng_calculation_formula WHERE run_id=$1 ORDER BY sheet,cell OFFSET $2 LIMIT $3) f USING(sheet,cell) WHERE d.run_id=$1 ORDER BY d.sheet,d.cell,d.position`, id, offset, limit)
	if e != nil {
		return page, e
	}
	lookup := map[string]int{}
	for i, f := range run.Formulas {
		lookup[f.Cell.SheetID+"!"+f.Cell.Address] = i
	}
	for rows.Next() {
		var sheet, cell string
		var d engineering.FormulaDependency
		if e = rows.Scan(&sheet, &cell, &d.ToSheetID, &d.ToRange, &d.Kind); e != nil {
			break
		}
		d.From = engineering.FormulaCell{SheetID: sheet, Address: cell}
		if i, ok := lookup[sheet+"!"+cell]; ok {
			run.Formulas[i].Dependencies = append(run.Formulas[i].Dependencies, d)
		}
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	return page, e
}

func (s *Server) engineeringCalculationWorker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runEngineeringCalculation(ctx)
		}
	}
}
func (s *Server) runEngineeringCalculation(ctx context.Context) {
	claim := uuid.NewString()
	var id, project, source string
	var requested []byte
	e := s.db.QueryRow(ctx, `UPDATE eng_calculation_run SET status='running',claim_id=$1,updated_at=now(),started_at=now() WHERE id=(SELECT id FROM eng_calculation_run WHERE status='pending' OR (status='running' AND updated_at<now()-interval '5 minutes') ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id::text,project_id,source_id::text,requested_cells`, claim).Scan(&id, &project, &source, &requested)
	if e != nil {
		return
	}
	c, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	scoped := *s
	scoped.project = project
	s = &scoped
	page, e := s.engineeringCalculation(c, id, 0, 0)
	if e != nil {
		return
	}
	run := page.Run
	if run.Rule != nil {
		view, err := engineeringRule(c, s.db, project, run.Rule.ID, run.Rule.Version)
		if err == nil {
			err = s.engineeringRuleDependencies(c, view.Rule)
		}
		if err == nil {
			run, err = engineering.RunRule(view.Rule, project, run.RevisionID, run.Inputs)
		}
		e = err
	} else {
		var key string
		var cells []engineering.FormulaCell
		e = json.Unmarshal(requested, &cells)
		if e == nil {
			e = s.db.QueryRow(c, `SELECT b.object_key FROM eng_source s JOIN bom_import_batch b ON b.id=s.import_id WHERE s.id=$1 AND s.project_id=$2`, source, project).Scan(&key)
		}
		if e == nil {
			provider := engineering.LibreOfficeProvider{Image: os.Getenv("KEEL_CALC_IMAGE")}
			run, e = provider.Calculate(c, engineering.SpreadsheetCalculationRequest{Source: engineering.WorkbookSource{ID: source, ProjectID: project, SHA256: run.SourceSHA256}, Revision: engineering.ModelRevision{ID: run.RevisionID, ModuleInstanceID: run.ModuleInstanceID}, RequestedCells: cells, OpenSource: func(ctx context.Context) (io.ReadCloser, error) {
				return s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
			}})
		}
	}
	run.ID = id
	if run.Status == "running" || run.Status == "pending" || run.Status == "" {
		run.Status = "failed"
	}
	if e != nil && len(run.Errors) == 0 {
		run.Errors = []engineering.CalculationError{{Code: "CALC_EXECUTION_FAILED", Message: "计算失败；没有使用缓存替代"}}
	}
	if run.FinishedAt == nil {
		now := time.Now().UTC()
		run.FinishedAt = &now
	}
	finish, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()
	_ = s.saveEngineeringCalculation(finish, claim, run)
}
func (s *Server) engineeringRuleDependencies(ctx context.Context, rule engineering.CalculationRule) error {
	for _, ref := range rule.Dependencies {
		view, e := engineeringRule(ctx, s.db, s.project, ref.ID, ref.Version)
		if e != nil {
			return e
		}
		if view.Rule.ApprovalStatus != "approved" {
			return fmt.Errorf("RULE_DEPENDENCY_APPROVAL_REQUIRED")
		}
	}
	return nil
}
func (s *Server) saveEngineeringCalculation(ctx context.Context, claim string, run engineering.CalculationRun) error {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var locked string
	if e = tx.QueryRow(ctx, `SELECT id::text FROM eng_calculation_run WHERE id=$1 AND project_id=$2 AND claim_id=$3 AND status='running' FOR UPDATE`, run.ID, s.project, claim).Scan(&locked); e != nil {
		return e
	}
	formulaRows := [][]any{}
	dependencyRows := [][]any{}
	errorRows := [][]any{}
	for _, f := range run.Formulas {
		var cached, result any
		if f.CachedValue != nil {
			cached = jsonBytes(f.CachedValue)
		}
		if f.RecalculatedValue != nil {
			result = jsonBytes(f.RecalculatedValue)
		}
		formulaRows = append(formulaRows, []any{run.ID, f.Cell.SheetID, f.Cell.Address, f.Expression, cached, result, f.DependencyStatus, f.Compatibility, f.ErrorCode})
		for i, d := range f.Dependencies {
			dependencyRows = append(dependencyRows, []any{run.ID, f.Cell.SheetID, f.Cell.Address, i, d.ToSheetID, d.ToRange, d.Kind})
		}
	}
	for i, v := range run.Errors {
		var sheet, cell *string
		if v.Cell != nil {
			sheet = &v.Cell.SheetID
			cell = &v.Cell.Address
		}
		errorRows = append(errorRows, []any{run.ID, i, v.Code, v.Message, sheet, cell})
	}
	for _, entry := range []struct {
		table   string
		columns []string
		rows    [][]any
	}{{"eng_calculation_formula", []string{"run_id", "sheet", "cell", "expression", "cached_value", "recalculated_value", "dependency_status", "compatibility", "error_code"}, formulaRows}, {"eng_calculation_dependency", []string{"run_id", "sheet", "cell", "position", "to_sheet", "to_range", "kind"}, dependencyRows}, {"eng_calculation_error", []string{"run_id", "position", "code", "message", "sheet", "cell"}, errorRows}} {
		if len(entry.rows) > 0 {
			if _, e = tx.CopyFrom(ctx, pgx.Identifier{entry.table}, entry.columns, pgx.CopyFromRows(entry.rows)); e != nil {
				return e
			}
		}
	}
	for _, v := range run.Outputs {
		if _, e = tx.Exec(ctx, `INSERT INTO eng_calculation_value VALUES($1,'output',$2,$3)`, run.ID, v.Name, jsonBytes(v.Value)); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, `UPDATE eng_calculation_run SET status=$3,provider_version=$4,compatibility=$5,finished_at=$6,updated_at=now() WHERE id=$1 AND claim_id=$2`, run.ID, claim, run.Status, run.ProviderVersion, run.Compatibility, run.FinishedAt); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
