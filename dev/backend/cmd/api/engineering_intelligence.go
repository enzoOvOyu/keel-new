package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"strings"
)

func saveEngineeringIntelligence(ctx context.Context, tx pgx.Tx, project, source, revision string, sequence int, p engineering.EngineeringProposal, x engineering.Extraction, previous *string) error {
	if _, e := tx.Exec(ctx, `INSERT INTO eng_provenance(id,producer_type,producer_id,producer_version,run_id,actor_id) SELECT id,origin,CASE WHEN origin='agent' THEN 'workbook-intelligence' ELSE 'human-review' END,'engineering-v1',id,actor FROM eng_revision WHERE id=$1`, revision); e != nil {
		return e
	}
	if _, e := tx.Exec(ctx, `INSERT INTO eng_provenance_input VALUES($1,'source',$2,$3)`, revision, source, previous); e != nil {
		return e
	}
	plan := engineering.BuildImportPlan(project, source, revision, sequence, p, x.Model)
	var prior *string
	if previous != nil {
		if e := tx.QueryRow(ctx, `SELECT id::text FROM eng_import_plan WHERE revision_id=$1`, *previous).Scan(&prior); e != nil && e != pgx.ErrNoRows {
			return e
		}
	}
	if _, e := tx.Exec(ctx, `INSERT INTO eng_import_plan VALUES($1,$2,$3,$4,$5,$6,$7)`, plan.ID, revision, source, project, sequence, p.Summary, prior); e != nil {
		return e
	}
	for _, m := range plan.Modules {
		if _, e := tx.Exec(ctx, `INSERT INTO eng_module_instance VALUES($1,$2,$3,$4,$5,$6,$7)`, plan.ID, m.ID, m.DefinitionID, m.DefinitionVersion, m.State, m.ApplicabilityReason, m.HeadRevisionID); e != nil {
			return e
		}
		for _, input := range m.Inputs {
			if _, e := tx.Exec(ctx, `INSERT INTO eng_module_input VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, plan.ID, m.ID, input.Name, input.ProducerInstanceID, input.Output, input.RevisionID, input.Digest, input.Required, input.Available); e != nil {
				return e
			}
		}
	}
	for i, r := range p.Regions {
		id := engineering.StableID(source, "region", fmt.Sprint(i))
		if _, e := tx.Exec(ctx, `INSERT INTO eng_workbook_region VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, revision, id, source, r.Sheet, r.Bounds.StartRow, r.Bounds.EndRow, r.Bounds.StartColumn, r.Bounds.EndColumn, r.Kind, r.Semantic, nonNilInts(r.HeaderRows)); e != nil {
			return e
		}
		position := 0
		insert := func(record *int, field, sheet string, cell *string, col *int, unit string, value *string, refs []string) error {
			position++
			_, e := tx.Exec(ctx, `INSERT INTO eng_region_binding VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, revision, id, position, record, field, sheet, cell, col, unit, value, refs)
			return e
		}
		for _, b := range r.Columns {
			column := b.Column
			if e := insert(nil, b.Field, r.Sheet, nil, &column, b.Unit, nil, []string{}); e != nil {
				return e
			}
		}
		for n, record := range r.Records {
			for _, b := range record.Cells {
				cell := b.Cell
				if e := insert(&n, b.Field, b.Sheet, &cell, nil, b.Unit, nil, []string{}); e != nil {
					return e
				}
			}
		}
		for _, b := range r.Constants {
			value := b.Value
			if e := insert(nil, b.Field, r.Sheet, nil, nil, "", &value, b.EvidenceCells); e != nil {
				return e
			}
		}
	}
	if len(x.Facts) == 0 && previous != nil {
		// Human edits retain the original proposed source facts; the reviewed model is a separate revisioned layer.
		if _, e := tx.Exec(ctx, `INSERT INTO eng_semantic_fact SELECT $1,f.id,f.region_id,f.subject_id,f.predicate,f.value_kind,f.text_value,f.decimal_value,f.boolean_value,f.reference_value,f.unit,f.claim_status,f.maturity,f.provenance_id FROM eng_semantic_fact f JOIN eng_workbook_region r ON r.revision_id=$1 AND r.id=f.region_id WHERE f.revision_id=$2`, revision, *previous); e != nil {
			return e
		}
	}
	for _, f := range x.Facts {
		if _, e := tx.Exec(ctx, `INSERT INTO eng_semantic_fact VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT DO NOTHING`, revision, f.ID, f.RegionID, f.SubjectID, f.Predicate, f.Value.Kind, f.Value.Text, f.Value.Decimal, f.Value.Boolean, f.Value.Reference, f.Value.Unit, f.ClaimStatus, f.Maturity, revision); e != nil {
			return e
		}
	}
	return nil
}
func nonNilInts(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}

func (s *Server) engineeringPlan(ctx context.Context, w engineering.Workspace) (engineering.PlanView, error) {
	var p engineering.ImportPlan
	p.Modules = []engineering.ModuleInstance{}
	if w.HeadRevisionID == nil {
		return engineering.PlanView{}, pgx.ErrNoRows
	}
	e := s.db.QueryRow(ctx, `SELECT id::text,project_id,source_id::text,version,summary,previous_plan_id::text FROM eng_import_plan WHERE revision_id=$1`, *w.HeadRevisionID).Scan(&p.ID, &p.ProjectID, &p.SourceID, &p.Version, &p.Summary, &p.PreviousPlanID)
	if e != nil {
		return engineering.PlanView{}, e
	}
	rows, e := s.db.Query(ctx, `SELECT id::text,definition_id,definition_version,state,applicability_reason,head_revision_id::text FROM eng_module_instance WHERE plan_id=$1 ORDER BY definition_id`, p.ID)
	if e != nil {
		return engineering.PlanView{}, e
	}
	for rows.Next() {
		m := engineering.ModuleInstance{Inputs: []engineering.InputBinding{}}
		if e = rows.Scan(&m.ID, &m.DefinitionID, &m.DefinitionVersion, &m.State, &m.ApplicabilityReason, &m.HeadRevisionID); e != nil {
			rows.Close()
			return engineering.PlanView{}, e
		}
		p.Modules = append(p.Modules, m)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return engineering.PlanView{}, e
	}
	for i := range p.Modules {
		var raw []byte
		e = s.db.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_strip_nulls(to_jsonb(t)-'plan_id'-'instance_id') ORDER BY name),'[]'::jsonb) FROM eng_module_input t WHERE plan_id=$1 AND instance_id=$2`, p.ID, p.Modules[i].ID).Scan(&raw)
		if e == nil {
			e = json.Unmarshal(raw, &p.Modules[i].Inputs)
		}
		if e != nil {
			return engineering.PlanView{}, e
		}
	}
	view := engineering.EffectivePlan(p, w.Targets, w.LatestJob)
	active, e := s.db.Query(ctx, `SELECT DISTINCT module_instance_id FROM eng_calculation_run WHERE source_id=$1 AND revision_id=$2 AND status IN('pending','running')`, w.ID, w.HeadRevisionID)
	if e != nil {
		return view, e
	}
	defer active.Close()
	for active.Next() {
		var module string
		if e = active.Scan(&module); e != nil {
			return view, e
		}
		for i := range view.Plan.Modules {
			if view.Plan.Modules[i].ID == module {
				view.Plan.Modules[i].State = engineering.Processing
			}
		}
	}
	return view, active.Err()
}
func (s *Server) engineeringIntelligence(ctx context.Context, w engineering.Workspace, offset int) (engineering.IntelligenceView, error) {
	out := engineering.IntelligenceView{Regions: []engineering.WorkbookRegion{}, Facts: []engineering.SemanticFact{}, Offset: offset}
	var raw []byte
	e := s.db.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'source_id',source_id,'sheet_id',sheet,'bounds',jsonb_build_object('start_row',start_row,'end_row',end_row,'start_column',start_column,'end_column',end_column),'kind',kind,'semantic_label',semantic_label,'headers','[]'::jsonb,'merges','[]'::jsonb) ORDER BY sheet,start_row),'[]'::jsonb) FROM eng_workbook_region WHERE revision_id=$1`, w.HeadRevisionID).Scan(&raw)
	if e == nil {
		e = json.Unmarshal(raw, &out.Regions)
	}
	if e != nil {
		return out, e
	}
	// Header and merged-cell shapes come from the immutable workbook, not generated captions.
	var book bom.Workbook
	var proposal engineering.EngineeringProposal
	var proposalRaw []byte
	e = s.db.QueryRow(ctx, `SELECT b.workbook,r.proposal FROM bom_import_batch b JOIN eng_revision r ON r.id=$3 WHERE b.id=$1 AND b.project_id=$2`, w.ImportID, s.project, w.HeadRevisionID).Scan(&raw, &proposalRaw)
	if e == nil {
		e = json.Unmarshal(raw, &book)
	}
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(proposalRaw, &proposal); e != nil {
		return out, e
	}
	fillRegionStructure(out.Regions, book, proposal)
	if e = s.db.QueryRow(ctx, `SELECT count(*) FROM eng_semantic_fact WHERE revision_id=$1`, w.HeadRevisionID).Scan(&out.TotalFacts); e != nil {
		return out, e
	}
	rows, e := s.db.Query(ctx, `SELECT id::text,region_id::text,subject_id::text,predicate,value_kind,text_value,decimal_value::text,boolean_value,reference_value::text,unit,claim_status,maturity,provenance_id::text FROM eng_semantic_fact WHERE revision_id=$1 ORDER BY region_id,subject_id,predicate,id OFFSET $2 LIMIT 100`, w.HeadRevisionID, offset)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		f := engineering.SemanticFact{ProjectID: s.project, RevisionID: *w.HeadRevisionID}
		if e = rows.Scan(&f.ID, &f.RegionID, &f.SubjectID, &f.Predicate, &f.Value.Kind, &f.Value.Text, &f.Value.Decimal, &f.Value.Boolean, &f.Value.Reference, &f.Value.Unit, &f.ClaimStatus, &f.Maturity, &f.ProvenanceID); e != nil {
			return out, e
		}
		out.Facts = append(out.Facts, f)
	}
	return out, rows.Err()
}

func fillRegionStructure(regions []engineering.WorkbookRegion, book bom.Workbook, proposal engineering.EngineeringProposal) {
	for i := range regions {
		r := &regions[i]
		for n, p := range proposal.Regions {
			if engineering.StableID(r.SourceID, "region", fmt.Sprint(n)) != r.ID {
				continue
			}
			for level, row := range p.HeaderRows {
				for col := p.Bounds.StartColumn; col <= p.Bounds.EndColumn; col++ {
					r.Headers = append(r.Headers, engineering.HeaderCell{Row: row, Column: col, Level: level})
				}
			}
		}
		for _, sheet := range book.Sheets {
			if sheet.Name != r.SheetID {
				continue
			}
			for _, merge := range sheet.Merges {
				parts := strings.Split(merge, ":")
				if len(parts) != 2 {
					continue
				}
				a, b, e := excelize.CellNameToCoordinates(parts[0])
				if e != nil {
					continue
				}
				z, y, e := excelize.CellNameToCoordinates(parts[1])
				if e != nil {
					continue
				}
				if b <= r.Bounds.EndRow && y >= r.Bounds.StartRow && a <= r.Bounds.EndColumn && z >= r.Bounds.StartColumn {
					r.Merges = append(r.Merges, engineering.Bounds{StartRow: b, EndRow: y, StartColumn: a, EndColumn: z})
				}
			}
		}
	}
}

// Queue only intrinsic calculations registered by applicable modules. Results
// remain proposals; applying them and approving release are separate actions.
func (s *Server) queuePlanCalculations(ctx context.Context, source string, expected int) error {
	w, e := s.engineeringWorkspace(ctx, source)
	if e != nil {
		return e
	}
	if w.Revision != expected {
		return fmt.Errorf("REVISION_CONFLICT")
	}
	view, e := s.engineeringPlan(ctx, w)
	if e != nil {
		return e
	}
	for _, m := range view.Plan.Modules {
		if m.State == engineering.NotApplicable || m.State == engineering.Detected || m.State == engineering.Blocked || m.State == engineering.NeedsInput {
			continue
		}
		operation := ""
		switch m.DefinitionID {
		case "power_cooling":
			operation = "power_balance"
		case "material_workload":
			operation = "workload_summary"
		}
		if operation == "" {
			continue
		}
		if _, e = s.queueEngineeringCalculation(ctx, source, engineering.CalculationRequest{Operation: operation, ExpectedRevision: expected, RequestedCells: []engineering.FormulaCell{}, Inputs: []engineering.NamedValue{}}, "engineering-plan"); e != nil {
			return e
		}
	}
	return nil
}
