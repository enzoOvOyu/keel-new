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
	"net"
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
	if err != nil {
		// A missing provider is an execution failure, not a new empty workbook.
		// Retain all previous plans, candidates and human review decisions.
		b.State, b.Phase, b.Error = "pending_review", "review", err.Error()
		data, _ := json.Marshal(b)
		s.db.Exec(ctx, "UPDATE bom_import_batch SET workbench=$2,revision=revision+1,reviewed_revision=NULL,updated_at=now() WHERE id=$1 AND revision=$3", id, data, rev)
		s.db.Exec(ctx, "UPDATE bom_job SET status='failed',error_code=$2,updated_at=now() WHERE id=$1", job, err.Error())
		return
	}
	started := time.Now()
	var selectedIDs []string
	json.Unmarshal(params, &selectedIDs)
	selected := map[string]bool{}
	for _, v := range selectedIDs {
		selected[v] = true
	}
	affected := map[string]bool{}
	for _, r := range b.Records {
		if selected[r.ID] {
			affected[fmt.Sprint(r.Sheet, ":", r.Row)] = true
		}
	}
	var raw []byte
	var wb bom.Workbook
	if err == nil {
		err = s.db.QueryRow(ctx, "SELECT workbook FROM bom_import_batch WHERE id=$1", id).Scan(&raw)
	}
	if err == nil {
		err = json.Unmarshal(raw, &wb)
		raw = nil
	}
	// Upgrade cached IR from the immutable object once; never change the uploaded file.
	if err == nil && len(wb.Sheets) > 0 && wb.Sheets[0].Parser != "excelize-ir/v4" {
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
	validSheets := map[string]bool{}
	for _, info := range b.Sheets {
		previous[info.Name] = info
	}
	wanted := map[string]bool{}
	for _, r := range b.Records {
		if !r.Imported && !r.Ignored && !r.Confirmed && r.Method != "manual" && (len(selected) == 0 || selected[r.ID]) {
			wanted[r.Sheet] = true
		}
	}
	// Resume from persisted candidates. A failed attempt must never replace a
	// successful plan or reconstruct unrelated reviewed source rows.
	b.Sheets = nil
	for _, sheet := range wb.Sheets {
		info, exists := previous[sheet.Name]
		if !exists {
			info = bom.RecognitionSheet{Name: sheet.Name, State: "pending"}
		}
		// Resume persisted successful extraction after a worker restart.
		if resume && len(selected) == 0 && wanted[sheet.Name] {
			if restoreWholeSheet(&b, sheet, &info) {
				validSheets[sheet.Name] = true
				delete(wanted, sheet.Name)
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
		return s.saveRecognitionSnapshot(ctx, id, rev, &b, final)
	}
	save(false)
	safeWorkbook := bom.Workbook{}
	for _, sh := range wb.Sheets {
		safeWorkbook.Sheets = append(safeWorkbook.Sheets, safeSheet(sh))
	}
	safeIR := bom.NewWorkbookIR(safeWorkbook)
	var planning ai.PlanningContext
	json.Unmarshal(b.Analysis, &planning)
	if err == nil && planning.Summary == "" {
		project := map[string]string{"id": s.project}
		var name, code string
		s.db.QueryRow(ctx, "SELECT name,code FROM project WHERE id=$1", s.project).Scan(&name, &code)
		project["name"] = name
		project["code"] = code
		planning, err = provider.UnderstandPlanning(ctx, project, safeWorkbook)
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
	classificationFailures := 0
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
					var prior *ai.SheetPlan
					if saved, restoreErr := ai.RestoreExtraction(sheet, previous[sheet.Name].Plan); restoreErr == nil && len(saved.Tables) > 0 {
						prior = &saved
					}
					plan, e := adapter.PlanSheetWithWorkbook(workCtx, safeIR, sheet.Name, planning, prior)
					// A transient timeout on one Sheet must not discard results from all other Sheets.
					if e != nil && (e.Error() == "PROVIDER_TIMEOUT_OR_UNAVAILABLE" || e.Error() == "PROVIDER_RESPONSE_FAILED") {
						plan, e = adapter.PlanSheetWithWorkbook(workCtx, safeIR, sheet.Name, planning, prior)
					}
					if e == nil {
						e = ai.CheckPlanRevision(sheet, plan, prior)
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
			b.Steps = append(b.Steps, r.plan.Steps...)
			calls += r.calls
			inputTokens += r.input
			outputTokens += r.output
			for k := range b.Sheets {
				if b.Sheets[k].Name == r.sheet.Name {
					info := &b.Sheets[k]
					info.State = "recognized"
					if r.err != nil {
						info.State = "failed"
						info.Error = r.err.Error()
						failed++
					} else {
						info.Purpose = r.plan.Purpose
						info.Note = r.plan.Note
						if len(selected) > 0 {
							// A partial pass does not prove the persisted whole-sheet
							// plan was applied, even if the old sheet was recognized.
							info.State = "pending"
							if old, e := ai.RestoreExtraction(r.sheet, previous[r.sheet.Name].Plan); e == nil && previous[r.sheet.Name].State == "recognized" && ai.SameExtraction(old, r.plan) {
								info.State = "recognized"
							}
						}
						validSheets[r.sheet.Name] = true
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
		groups := map[int][]int{}
		groupIDs := map[string]int{}
		descriptions := []ai.Description{}
		for j, r := range b.Records {
			if !validSheets[r.Sheet] || (len(selected) > 0 && !affected[fmt.Sprint(r.Sheet, ":", r.Row)]) || r.Imported || r.Ignored || r.Confirmed || r.Method == "manual" || r.Item.Model == "" || (r.Item.Category != "other" && r.Item.Category != "material" && r.Item.Category != "server" && r.Item.Category != "switch" && r.Item.Category != "router") {
				continue
			}
			if sourceCategory(r.Item.Sources["category"]) != "" {
				continue
			}
			description := r.Item.Fields["description"] + " " + r.Item.Fields["specification"] + " source_type=" + r.Item.Category + " record_kind=" + r.Item.Fields["record_kind"]
			key := strings.TrimSpace(r.Item.Model) + "|" + description
			gid, exists := groupIDs[key]
			if !exists {
				gid = len(descriptions)
				groupIDs[key] = gid
				descriptions = append(descriptions, ai.Description{ID: gid, Model: r.Item.Model, Name: r.Item.Model, Description: description})
			}
			groups[gid] = append(groups[gid], j)
		}

		if b.Classifications == nil {
			b.Classifications = map[string]json.RawMessage{}
		}
		cacheKey := func(d ai.Description) string {
			return fmt.Sprint(providerID, "@", provider.ConfigRevision, "|", d.Model, "|", d.Description)
		}
		apply := func(c ai.Classification, d ai.Description) {
			for _, j := range groups[d.ID] {
				r := &b.Records[j]
				if sourceCategory(r.Item.Sources["category"]) != "" && r.Item.Category != "other" && r.Item.Category != "material" && r.Item.Category != c.Type {
					r.Ambiguity = true
					r.Reason = "分类建议与已有来源类型冲突，保留原类型并要求人工核查：" + c.Type
					continue
				}
				r.Item.Category = c.Type
				r.Item.Fields["category"] = c.Type
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
				classificationFailures++
				b.Trace("classify_models", "", "", "failed", fmt.Sprintf("batch=%d items=%d error=%s", start/24+1, end-start, e.Error()))
				for _, description := range pending[start:end] {
					for _, index := range groups[description.ID] {
						b.Records[index].Ambiguity = true
						b.Records[index].Reason = "分类失败，保留来源候选供审核：" + e.Error()
					}
				}
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
	} else if classificationFailures > 0 {
		b.Error = fmt.Sprintf("%d 个工作表提取失败；%d 个型号分类批次失败，具体错误见分类记录。", failed, classificationFailures)
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
	} else if classificationFailures > 0 {
		state = "failed"
		code = "PARTIAL_CLASSIFICATION"
	} else if failed > 0 {
		state = "failed"
		code = "PARTIAL_SHEET_RECOGNITION"
	}
	diag := map[string]any{"schema_version": "workbook-agent/v6", "prompt_version": "workbook-agent/v6", "provider_id": providerID, "completed_sheets": completed, "failed_sheets": failed, "failed_classification_batches": classificationFailures, "total_sheets": len(b.Sheets), "records": len(b.Records), "model_calls": calls, "input_tokens": inputTokens, "output_tokens": outputTokens, "duration_ms": time.Since(started).Milliseconds(), "error_code": code, "raw_response_stored": false}
	if provider != nil {
		diag["model"] = provider.Model
		diag["provider_revision"] = provider.ConfigRevision
	}
	diagnostic, _ := json.Marshal(diag)
	s.db.Exec(ctx, "UPDATE bom_job SET status=$2,error_code=$3,result=$4,updated_at=now() WHERE id=$1", job, state, code, diagnostic)
	s.db.Exec(ctx, "INSERT INTO ai_task(id,import_id,mode,task,status,diagnostic) VALUES($1,$2,'provider','records',$3,$4)", uuid.NewString(), id, state, diagnostic)
}

// A validated saved recipe may have been applied only to selected rows (or
// incorrectly marked pending by an older worker). Full resume applies it to
// every source row locally before marking the sheet complete.
func restoreWholeSheet(b *bom.Workbench, sheet bom.Sheet, info *bom.RecognitionSheet) bool {
	plan, err := ai.RestoreExtraction(sheet, info.Plan)
	if err != nil {
		return false
	}
	b.Records = extractWithTools(b, sheet, plan, nil)
	info.State, info.Error = "recognized", ""
	info.Plan, _ = json.Marshal(plan)
	return true
}

func safeSheet(sheet bom.Sheet) bom.Sheet {
	safe := sheet
	safe.Cells = append([]bom.Cell{}, sheet.Cells...)
	// Only schema/header/model context is required. Do not send known identity or secret columns.
	identityCols := map[int]bool{}
	labelCols := map[int]bool{}
	for _, c := range sheet.Cells {
		if c.Row > 8 {
			continue
		}
		f := bom.Field(c.Value)
		label := strings.ToLower(strings.ReplaceAll(c.Value, " ", ""))
		if f == "serial" || f == "management_ip" || f == "mac" || strings.Contains(label, "ip地址") || strings.Contains(label, "loopback") || strings.Contains(label, "guid") || strings.Contains(label, "username") || strings.Contains(label, "账号") {
			identityCols[c.Column] = true
		}
		if f == "hostname" || f == "endpoint_a_device" || f == "endpoint_b_device" || f == "device_alias" || f == "termination_a" || f == "termination_b" {
			labelCols[c.Column] = true
		}
	}
	for k := range safe.Cells {
		c := &safe.Cells[k]
		if c.Sensitive {
			c.Value = ""
			continue
		}
		if bom.Field(c.Value) != "" {
			continue
		}
		_, macErr := net.ParseMAC(c.Value)
		if net.ParseIP(c.Value) != nil || macErr == nil || identityCols[c.Column] && bom.ExtractCellValue(*c, "serial") != "" {
			c.Value = "[IDENTITY]"
			continue
		}
		if labelCols[c.Column] && bom.ExtractCellValue(*c, "hostname") != "" {
			c.Value = "[DEVICE-" + bom.StableID(c.Value) + "]"
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
	selectedRows := map[int]bool{}
	sourceRows := map[int]bom.ImportRecord{}
	for _, r := range bom.PlanningSourceRows(bom.Workbook{Sheets: []bom.Sheet{sheet}}).Records {
		sourceRows[r.Row] = r
	}
	protected := map[string]bool{}
	priorIDs := map[string]string{}
	for _, old := range records {
		if old.Sheet != sheet.Name {
			continue
		}
		if selected[old.ID] {
			selectedRows[old.Row] = true
		}
		if old.Method == "source_relation" {
			continue
		}
		cols := map[string]int{}
		for field, cell := range old.Item.Sources {
			cols[field] = cell.Column
		}
		if anchor := extractionAnchor(cols); anchor != "" {
			key := fmt.Sprint(old.Row) + ":" + anchor
			priorIDs[key] = old.ID
			if old.Imported || old.Ignored || old.Confirmed || old.Method == "manual" {
				protected[key] = true
			}
		}
	}
	if len(selected) == 0 {
		present := map[int]bool{}
		for _, old := range records {
			if old.Sheet == sheet.Name {
				present[old.Row] = true
			}
		}
		for _, source := range bom.PlanningSourceRows(bom.Workbook{Sheets: []bom.Sheet{sheet}}).Records {
			if !present[source.Row] {
				records = append(records, source)
			}
		}
	}
	out := []bom.ImportRecord{}
	seenRows := map[int]bool{}
	for _, record := range records {
		if record.Sheet != sheet.Name || record.Imported || record.Ignored || record.Confirmed || record.Method == "manual" || (len(selected) > 0 && !selectedRows[record.Row]) {
			out = append(out, record)
			continue
		}
		if seenRows[record.Row] {
			continue
		}
		seenRows[record.Row] = true
		// A surviving derived optic/port or an old extracted entity cannot be
		// reused as the raw-row placeholder. Its ID may be regenerated later.
		rawRow := sourceRows[record.Row]
		record.Raw, record.Original = rawRow.Raw, rawRow.Original
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
				if !ai.TableRowPresent(t, cells[record.Row]) || len(fields) == 0 {
					continue
				}
				kind := t.Type
				if t.TypeColumn > 0 {
					v := strings.TrimSpace(cells[record.Row][t.TypeColumn].Value)
					mapped, ok := t.Types[v]
					if ok {
						kind = mapped
						sources["category"] = cells[record.Row][t.TypeColumn]
					} else {
						continue // another typed extractor may handle it; otherwise raw row is retained
					}
				}
				if explicit := sourceCategory(sources["category"]); explicit != "" {
					kind = explicit
				}
				r := record
				anchor := extractionAnchor(t.Fields)
				r.ID = bom.StableID("candidate:" + record.Sheet + ":" + fmt.Sprint(record.Row) + ":source:" + anchor)
				if protected[fmt.Sprint(record.Row)+":"+anchor] {
					continue
				}
				if old := priorIDs[fmt.Sprint(record.Row)+":"+anchor]; old != "" {
					r.ID = old
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
				if sourceCategory(record.Item.Sources["category"]) != "" && record.Item.Category != "" && record.Item.Category != "other" && record.Item.Category != "material" && record.Item.Category != kind && extractionAnchorFromItem(record.Item) == anchor {
					r.Ambiguity = true
					r.Reason = "重分析分类与已有候选冲突，保留原类型并要求人工核查：" + kind
					r.Item.Category = record.Item.Category
					r.Item.Fields["category"] = record.Item.Category
				}
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
			record.ID = rawRow.ID
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

// Candidate identity is derived from source identity columns, never an AI-generated table name or classification.
func extractionAnchor(fields map[string]int) string { return ai.SourceAnchor(fields) }

func extractionAnchorFromItem(item bom.Item) string {
	cols := map[string]int{}
	for field, cell := range item.Sources {
		cols[field] = cell.Column
	}
	return extractionAnchor(cols)
}

// Only a literal supported source type is authoritative; model suggestions
// and ambiguous type labels still require classification and review.
func sourceCategory(c bom.Cell) string {
	kind := strings.ToLower(strings.TrimSpace(c.Value))
	if !c.Sensitive && kind != "other" && bom.KnownImportType(kind) {
		return kind
	}
	return ""
}

// Keep checkpoint and final draft atomic while releasing each PostgreSQL JSON
// input context before parsing the next large document.
func (s *Server) saveRecognitionSnapshot(ctx context.Context, id string, rev int, b *bom.Workbench, final bool) bool {
	bom.CompactWorkbench(b)
	data, err := json.Marshal(b)
	if err != nil {
		return false
	}
	if !final {
		tag, err := s.db.Exec(ctx, "UPDATE bom_import_batch SET workbench=$2,updated_at=now() WHERE id=$1 AND revision=$3", id, data, rev)
		return err == nil && tag.RowsAffected() == 1
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "UPDATE bom_import_batch SET workbench=$2,revision=revision+1,reviewed_revision=NULL,updated_at=now() WHERE id=$1 AND revision=$3", id, data, rev)
	if err != nil || tag.RowsAffected() != 1 {
		return false
	}
	data = nil
	draft, err := json.Marshal(bom.WorkbenchItems(*b, s.project))
	if err != nil {
		return false
	}
	if _, err = tx.Exec(ctx, "UPDATE bom_import_batch SET draft=$2 WHERE id=$1", id, draft); err != nil {
		return false
	}
	return tx.Commit(ctx) == nil
}
