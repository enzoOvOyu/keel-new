package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"io"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"sort"
	"strings"
	"time"
)

func (s *Server) workbenchProvider(ctx context.Context, id string) (*ai.Provider, string, error) {
	var provider *string
	if e := s.db.QueryRow(ctx, "SELECT provider_id::text FROM bom_import_batch WHERE id=$1", id).Scan(&provider); e != nil {
		return nil, "", e
	}
	if provider == nil {
		return s.defaultAI(ctx)
	}
	c, e := scanAI(s.db.QueryRow(ctx, aiConfigSelect+" WHERE id=$1 AND enabled", *provider))
	if e != nil {
		return nil, "", errors.New("PROVIDER_NOT_AVAILABLE")
	}
	return s.aiAdapter(c)
}

// One persisted orchestrator operates on immutable Workbook IR. The model proposes
// semantic bindings; local tools extract values and independently audit coverage.
func (s *Server) runRecognition(ctx context.Context, job, id string, rev int, params []byte) {
	b, _, err := s.loadWorkbench(ctx, id)
	if err != nil {
		return
	}
	provider, providerID, err := s.workbenchProvider(ctx, id)
	started := time.Now()
	var selectedIDs []string
	json.Unmarshal(params, &selectedIDs)
	selected := map[string]bool{}
	for _, v := range selectedIDs {
		selected[v] = true
	}
	var raw []byte
	var wb bom.Workbook
	if err == nil {
		err = s.db.QueryRow(ctx, "SELECT workbook FROM bom_import_batch WHERE id=$1", id).Scan(&raw)
	}
	if err == nil {
		err = json.Unmarshal(raw, &wb)
	}
	// Upgrade cached IR from the immutable object once; never change the uploaded file.
	if err == nil && len(wb.Sheets) > 0 && wb.Sheets[0].Parser != "excelize-ir/v3" {
		var key string
		if e := s.db.QueryRow(ctx, "SELECT object_key FROM bom_import_batch WHERE id=$1", id).Scan(&key); e != nil {
			err = e
		} else {
			object, e := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
			if e != nil {
				err = e
			} else {
				source, e := io.ReadAll(io.LimitReader(object, 21<<20))
				object.Close()
				if e != nil {
					err = e
				} else {
					wb, err = bom.Parse(source)
					if err == nil {
						encoded, _ := json.Marshal(wb)
						_, err = s.db.Exec(ctx, "UPDATE bom_import_batch SET workbook=$2 WHERE id=$1", id, encoded)
					}
				}
			}
		}
	}
	ir := bom.NewWorkbookIR(wb)
	regions := ir.InspectWorkbook()
	b.Trace("inspect_workbook", "", "", "succeeded", fmt.Sprintf("%d sheets / %d structural regions; source cells retained", len(wb.Sheets), len(regions)))
	resume := b.Phase != "queued" && b.Phase != "review" && b.Phase != ""
	b.Phase = "understanding"
	b.Provider = providerID
	b.Error = ""
	previous := map[string]bom.RecognitionSheet{}
	for _, info := range b.Sheets {
		if source, ok := ir.AnalyzeSheet(info.Name); ok && len(info.Plan) > 0 {
			if plan, e := ai.RestoreExtraction(source, info.Plan); e == nil {
				info.Plan, _ = json.Marshal(plan)
				info.State = "recognized"
				info.Error = ""
			}
		}
		previous[info.Name] = info
	}
	wanted := map[string]bool{}
	for _, r := range b.Records {
		if !r.Imported && !r.Ignored && (len(selected) == 0 || selected[r.ID]) {
			wanted[r.Sheet] = true
		}
	}
	// Replay already obtained AI plans against immutable cells after a worker restart.
	// No external request or stale generated candidate is needed for this step.
	canReplay := resume
	for _, r := range b.Records {
		if r.Imported || r.Method == "manual" || r.Confirmed || r.Ignored {
			canReplay = false
		}
	}
	if canReplay {
		b.Records = bom.PlanningSourceRows(wb).Records
		selected = map[string]bool{}
		for _, sh := range wb.Sheets {
			info := previous[sh.Name]
			var plan ai.SheetPlan
			if info.State == "recognized" && json.Unmarshal(info.Plan, &plan) == nil && ai.CheckExtraction(sh, plan) == nil {
				b.Records = applySheetPlan(b.Records, sh, plan, nil)
			}
		}
	}
	b.Sheets = nil
	for _, sheet := range wb.Sheets {
		info, exists := previous[sheet.Name]
		if !exists {
			info = bom.RecognitionSheet{Name: sheet.Name, State: "pending"}
		}
		// Resume persisted successful extraction after a worker restart.
		if resume && info.State == "recognized" {
			var plan ai.SheetPlan
			if json.Unmarshal(info.Plan, &plan) == nil && ai.CheckExtraction(sheet, plan) == nil {
				delete(wanted, sheet.Name)
			} else {
				wanted[sheet.Name] = true
			}
		}
		if wanted[sheet.Name] {
			info.State = "pending"
			info.Error = ""
			info.Rows = 0
			info.Recognized = 0
		}
		b.Sheets = append(b.Sheets, info)
	}
	save := func(final bool) bool {
		bom.CompactWorkbench(&b)
		data, marshalErr := json.Marshal(b)
		if marshalErr != nil {
			return false
		}
		query := "UPDATE bom_import_batch SET workbench=$2,updated_at=now() WHERE id=$1 AND revision=$3"
		args := []any{id, data, rev}
		if final {
			draft, _ := json.Marshal(bom.WorkbenchItems(b, s.project))
			query = "UPDATE bom_import_batch SET workbench=$2,draft=$4,revision=revision+1,reviewed_revision=NULL,updated_at=now() WHERE id=$1 AND revision=$3"
			args = append(args, draft)
		}
		tag, e := s.db.Exec(ctx, query, args...)
		return e == nil && tag.RowsAffected() == 1
	}
	save(false)
	var planning ai.PlanningContext
	json.Unmarshal(b.Analysis, &planning)
	if err == nil && planning.Summary == "" {
		project := map[string]string{"id": s.project}
		var name, code string
		s.db.QueryRow(ctx, "SELECT name,code FROM project WHERE id=$1", s.project).Scan(&name, &code)
		project["name"] = name
		project["code"] = code
		safe := bom.Workbook{}
		for _, sh := range wb.Sheets {
			safe.Sheets = append(safe.Sheets, safeSheet(sh))
		}
		planning, err = provider.UnderstandPlanning(ctx, project, safe)
		if err == nil {
			b.Analysis, _ = json.Marshal(planning)
			b.Phase = "extracting"
			save(false)
		}
	}
	if err == nil {
		b.Phase = "extracting"
		save(false)
	}
	completed, failed, inputTokens, outputTokens, calls := 0, 0, 0, 0, 0
	if provider != nil {
		inputTokens = provider.TotalInputTokens
		outputTokens = provider.TotalOutputTokens
		calls = provider.Calls
	}
	// Independent sheets run with two bounded requests. Results are applied and persisted serially.
	type result struct {
		sheet                bom.Sheet
		plan                 ai.SheetPlan
		err                  error
		input, output, calls int
	}
	results := make(chan result)
	jobs := make(chan bom.Sheet)
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err == nil {
		for worker := 0; worker < 2; worker++ {
			go func() {
				for sheet := range jobs {
					adapter := *provider
					adapter.Calls = 0
					adapter.TotalInputTokens = 0
					adapter.TotalOutputTokens = 0
					source, _ := ir.AnalyzeSheet(sheet.Name)
					safe := safeSheet(source)
					plan, e := adapter.PlanSheet(workCtx, safe, planning)
					// A transient timeout on one Sheet must not discard results from all other Sheets.
					if e != nil && (e.Error() == "PROVIDER_TIMEOUT_OR_UNAVAILABLE" || e.Error() == "PROVIDER_RESPONSE_FAILED") {
						plan, e = adapter.PlanSheet(workCtx, safe, planning)
					}
					r := result{sheet: sheet, plan: plan, err: e}
					r.input = adapter.TotalInputTokens
					r.output = adapter.TotalOutputTokens
					r.calls = adapter.Calls
					select {
					case results <- r:
					case <-workCtx.Done():
						return
					}
				}
			}()
		}
		go func() {
			defer close(jobs)
			ordered := append([]bom.Sheet{}, wb.Sheets...)
			priority := func(sh bom.Sheet) int {
				for _, source := range planning.Sources {
					if source.Sheet == sh.Name {
						if source.Role == "switch_inventory" {
							return 0
						}
						if source.Role == "connections" {
							return 1
						}
						if source.Role == "device_inventory" {
							return 2
						}
					}
				}
				return 3
			}
			sort.SliceStable(ordered, func(i, j int) bool { return priority(ordered[i]) < priority(ordered[j]) })
			for _, sheet := range ordered {
				if wanted[sheet.Name] {
					select {
					case jobs <- sheet:
					case <-workCtx.Done():
						return
					}
				}
			}
		}()
		for range wanted {
			var r result
			select {
			case r = <-results:
			case <-ctx.Done():
				return
			}
			calls += r.calls
			inputTokens += r.input
			outputTokens += r.output
			for k := range b.Sheets {
				if b.Sheets[k].Name == r.sheet.Name {
					info := &b.Sheets[k]
					info.State = "recognized"
					info.Purpose = r.plan.Purpose
					info.Note = r.plan.Note
					if r.err != nil {
						info.State = "failed"
						info.Error = r.err.Error()
						info.Plan, _ = json.Marshal(r.plan)
						failed++
					} else {
						b.Trace("analyze_sheet", r.sheet.Name, "", "succeeded", r.plan.Note)
						for _, region := range regions {
							if region.Sheet == r.sheet.Name {
								cells := ir.InspectRegion(region)
								b.Trace("inspect_region", r.sheet.Name, region.Range, "succeeded", fmt.Sprintf("scanned %d source cells", len(cells)))
							}
						}
						b.Records = extractWithTools(&b, r.sheet, r.plan, selected)
						refs := ir.CrossSheetResolve(r.sheet.Name)
						b.Trace("cross_sheet_resolve", r.sheet.Name, "", "succeeded", fmt.Sprintf("resolved %d explicit source cell references", len(refs)))
						planJSON, _ := json.Marshal(r.plan)
						info.Plan = planJSON
					}
					for _, row := range b.Records {
						if row.Sheet == r.sheet.Name {
							info.Rows++
							if row.Method == "provider" && row.Recognition != "unrecognized" {
								info.Recognized++
							}
						}
					}
				}
			}
			completed++
			s.db.Exec(ctx, "UPDATE bom_job SET updated_at=now(),result=$2 WHERE id=$1", job, fmt.Sprintf(`{"completed_sheets":%d,"total_sheets":%d}`, completed, len(b.Sheets)))
			if !save(false) {
				err = errors.New("REVISION_CONFLICT")
				break
			}
		}
	}
	if err == nil {
		b.Phase = "classifying"
		save(false)
		affected := map[string]bool{}
		for _, r := range b.Records {
			if selected[r.ID] {
				affected[fmt.Sprint(r.Sheet, ":", r.Row)] = true
			}
		}
		groups := map[string][]int{}
		descriptions := []ai.Description{}
		for j, r := range b.Records {
			if (len(selected) > 0 && !affected[fmt.Sprint(r.Sheet, ":", r.Row)]) || r.Imported || r.Ignored || r.Confirmed || r.Method == "manual" || r.Item.Model == "" || (r.Item.Category != "other" && r.Item.Category != "material" && r.Item.Category != "server" && r.Item.Category != "switch" && r.Item.Category != "router") {
				continue
			}
			key := strings.TrimSpace(r.Item.Model)
			if _, ok := groups[key]; !ok {
				descriptions = append(descriptions, ai.Description{ID: len(descriptions), Model: r.Item.Model, Name: r.Item.Model, Description: r.Item.Fields["description"] + " " + r.Item.Fields["specification"]})
			}
			groups[key] = append(groups[key], j)
		}

		if b.Classifications == nil {
			b.Classifications = map[string]json.RawMessage{}
		}
		cacheKey := func(d ai.Description) string {
			return fmt.Sprint(providerID, "@", provider.ConfigRevision, "|", d.Model, "|", d.Description)
		}
		apply := func(c ai.Classification, d ai.Description) {
			for _, j := range groups[strings.TrimSpace(d.Model)] {
				r := &b.Records[j]
				r.Item.Category = c.Type
				r.Item.Fields["category"] = c.Type
				if c.Type != "server" && c.Type != "switch" && c.Type != "router" && r.Item.Fields["record_kind"] == "planned_device" {
					r.Item.Fields["record_kind"] = "component"
				}
				r.Confidence = &c.Confidence
				r.Reason = c.Reason
				r.Recognition = "recognized"
				if c.Type == "other" {
					r.Recognition = "unrecognized"
				}
			}
		}
		pending := []ai.Description{}
		for _, d := range descriptions {
			var c ai.Classification
			if raw, ok := b.Classifications[cacheKey(d)]; ok && json.Unmarshal(raw, &c) == nil {
				apply(c, d)
			} else {
				pending = append(pending, d)
			}
		}
		for start := 0; start < len(pending); start += 24 {
			end := start + 24
			if end > len(pending) {
				end = len(pending)
			}
			beforeInput, beforeOutput, beforeCalls := provider.TotalInputTokens, provider.TotalOutputTokens, provider.Calls
			classified, e := provider.Classify(ctx, pending[start:end], planning)
			inputTokens += provider.TotalInputTokens - beforeInput
			outputTokens += provider.TotalOutputTokens - beforeOutput
			calls += provider.Calls - beforeCalls
			if e != nil {
				failed++
				continue
			}
			for _, c := range classified {
				d := descriptions[c.ID]
				apply(c, d)
				b.Classifications[cacheKey(d)], _ = json.Marshal(c)
			}
			s.db.Exec(ctx, "UPDATE bom_job SET updated_at=now() WHERE id=$1", job)
			save(false)
		}

		b.Phase = "linking"
		save(false)
		bom.LinkRecognizedRecords(&b)
		b.Trace("resolve_relationships", "", "", "succeeded", "exact device/port references only; ambiguous endpoints remain blocked")
	}
	for j := range b.Sheets {
		info := &b.Sheets[j]
		info.Rows = 0
		info.Recognized = 0
		for _, r := range b.Records {
			if r.Sheet == info.Name {
				info.Rows++
				if r.Recognition != "unrecognized" {
					info.Recognized++
				}
			}
		}
	}
	b.Phase = "auditing"
	save(false)
	audit := ir.AuditCoverage(&b)
	b.Coverage = &audit
	b.Trace("audit_coverage", "", "", "succeeded", fmt.Sprintf("%d/%d source cells retained, %d extracted; unmapped cells stay reviewable", audit.RetainedCells, audit.TotalCells, audit.ExtractedCells))
	b.Phase = "review"
	b.State = "pending_review"
	if !audit.Complete && err == nil {
		err = errors.New("COVERAGE_REVIEW_REQUIRED")
	}
	if err != nil {
		b.Error = err.Error()
	} else if failed > 0 {
		b.Error = fmt.Sprintf("%d 个工作表识别失败，其他结果已保留；可选择对应 Sheet 后重试。", failed)
	}
	if !save(true) {
		err = errors.New("REVISION_CONFLICT")
	}
	state := "succeeded"
	var code any
	if err != nil {
		state = "failed"
		code = err.Error()
	} else if failed > 0 {
		state = "failed"
		code = "PARTIAL_SHEET_RECOGNITION"
	}
	diag := map[string]any{"schema_version": "workbook-agent/v3", "prompt_version": "workbook-agent/v3", "provider_id": providerID, "completed_sheets": completed, "failed_sheets": failed, "total_sheets": len(b.Sheets), "records": len(b.Records), "model_calls": calls, "input_tokens": inputTokens, "output_tokens": outputTokens, "duration_ms": time.Since(started).Milliseconds(), "error_code": code, "raw_response_stored": false}
	if provider != nil {
		diag["model"] = provider.Model
		diag["provider_revision"] = provider.ConfigRevision
	}
	diagnostic, _ := json.Marshal(diag)
	s.db.Exec(ctx, "UPDATE bom_job SET status=$2,error_code=$3,result=$4,updated_at=now() WHERE id=$1", job, state, code, diagnostic)
	s.db.Exec(ctx, "INSERT INTO ai_task(id,import_id,mode,task,status,diagnostic) VALUES($1,$2,'provider','records',$3,$4)", uuid.NewString(), id, state, diagnostic)
}

func safeSheet(sheet bom.Sheet) bom.Sheet {
	safe := sheet
	safe.Cells = append([]bom.Cell{}, sheet.Cells...)
	// Only schema/header/model context is required. Do not send known identity or secret columns.
	identityCols := map[int]bool{}
	for _, c := range sheet.Cells {
		f := bom.Field(c.Value)
		if f == "serial" || f == "management_ip" || f == "mac" {
			identityCols[c.Column] = true
		}
	}
	for k := range safe.Cells {
		c := &safe.Cells[k]
		if c.Sensitive {
			c.Value = ""
		} else if identityCols[c.Column] && bom.Field(c.Value) == "" {
			c.Value = "[IDENTITY]"
		}
	}
	return safe
}

func applySheetPlan(records []bom.ImportRecord, sheet bom.Sheet, plan ai.SheetPlan, selected map[string]bool) []bom.ImportRecord {
	cells := bom.MergedSourceCells(sheet)
	headers := []bom.Cell{}
	headerRows := map[int]bool{}
	for _, r := range plan.HeaderRows {
		headerRows[r] = true
	}
	for _, c := range sheet.Cells {
		if c.Sensitive {
			continue
		}
		if cells[c.Row] == nil {
			cells[c.Row] = map[int]bom.Cell{}
		}
		cells[c.Row][c.Column] = c
		if headerRows[c.Row] {
			headers = append(headers, c)
		}
	}
	out := []bom.ImportRecord{}
	seenRows := map[int]bool{}
	for _, record := range records {
		if record.Sheet != sheet.Name || record.Imported || record.Ignored || (len(selected) > 0 && !selected[record.ID]) {
			out = append(out, record)
			continue
		}
		if seenRows[record.Row] {
			continue
		}
		seenRows[record.Row] = true
		record.HeaderContext = headers
		record.Method = "provider"
		record.Recognition = "unrecognized"
		record.Confirmed = false
		record.Reason = plan.Note
		matches := []bom.ImportRecord{}
		if !headerRows[record.Row] {
			for _, t := range plan.Tables {
				if t.Kind == "reference" {
					continue
				}
				if record.Row < t.Start || record.Row > t.End {
					continue
				}
				fields := map[string]string{}
				sources := map[string]bom.Cell{}
				for f, col := range t.Fields {
					c := cells[record.Row][col]
					v := bom.ExtractCellValue(c, f)
					for _, h := range headers {
						if h.Column == col && c.FormulaBacked {
							label := strings.ToLower(strings.ReplaceAll(h.Value, " ", ""))
							if strings.Contains(label, "firstflag") || strings.Contains(label, "akey") || strings.Contains(label, "zkey") || strings.Contains(label, "rowid") {
								v = ""
							}
						}
					}
					if v != "" {
						for _, h := range headers {
							if h.Column == col && strings.TrimSpace(h.Value) == v {
								v = ""
								break
							}
						}
						if v == "" {
							continue
						}
						fields[f] = v
						sources[f] = c
					}
				}
				valid := true
				for _, key := range t.Required {
					if fields[key] == "" {
						valid = false
					}
				}
				if !valid || len(fields) == 0 {
					continue
				}
				kind := t.Type
				if t.TypeColumn > 0 {
					v := strings.TrimSpace(cells[record.Row][t.TypeColumn].Value)
					mapped, ok := t.Types[v]
					if ok {
						kind = mapped
					} else {
						continue // another typed extractor may handle it; otherwise raw row is retained
					}
				}
				r := record
				if len(plan.Tables) > 1 {
					r.ID = bom.StableID("candidate:" + record.Sheet + ":" + fmt.Sprint(record.Row) + ":object:" + t.Key)
				}
				r.Item = bom.Item{UID: r.ID, Sheet: r.Sheet, Status: "planned", Category: kind, Fields: fields, Sources: sources, Serial: fields["serial"], Hostname: fields["hostname"], Model: fields["model"], Parent: fields["parent_uid"], Port: fields["port_index"]}
				r.Item.Fields["category"] = kind
				r.Item.Fields["record_kind"] = t.Kind
				if fields["name"] == "" && fields["hostname"] != "" {
					fields["name"] = fields["hostname"]
					sources["name"] = sources["hostname"]
				}
				r.Confidence = &t.Confidence
				r.Ambiguity = false
				r.Recognition = "recognized"
				if kind == "other" {
					r.Recognition = "unrecognized"
				}
				r.Before = map[string]string{}
				for field, old := range record.Item.Fields {
					if fields[field] != old {
						r.Before[field] = old
					}
				}
				r.Unmapped = nil
				r.Dispositions = nil
				r.Issues = []bom.Issue{}
				matches = append(matches, r)
			}
		}
		if len(matches) == 0 {
			record.Item = bom.Item{UID: record.ID, Sheet: record.Sheet, Status: "planned", Category: "other", Fields: map[string]string{"category": "other"}, Sources: map[string]bom.Cell{}}
			record.Confidence = nil
			record.Ambiguity = false
			out = append(out, record)
		} else {
			out = append(out, matches...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Sheet < out[j].Sheet || out[i].Sheet == out[j].Sheet && out[i].Row < out[j].Row
	})
	return out
}

// Specialized extractors share one source-grounded implementation, while exposing
// the domain actually processed in the persisted tool trace.
func extractWithTools(b *bom.Workbench, sheet bom.Sheet, plan ai.SheetPlan, selected map[string]bool) []bom.ImportRecord {
	kinds := map[string]bool{}
	for _, t := range plan.Tables {
		tool := "extract_entities"
		if t.Type == "switch" || t.Type == "port" {
			tool = "extract_switches"
		}
		if t.Kind == "physical_connection" {
			tool = "extract_terminations"
		}
		kinds[tool] = true
	}
	result := applySheetPlan(b.Records, sheet, plan, selected)
	for tool := range kinds {
		b.Trace(tool, sheet.Name, "", "succeeded", "copied bound original cell values; no generated equipment identities")
	}
	return result
}
