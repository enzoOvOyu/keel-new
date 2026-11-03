package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/kickoff"
)

type kickoffAgentStructure struct {
	Sheets  int          `json:"sheets"`
	Cells   int          `json:"cells"`
	Regions []bom.Region `json:"regions"`
}

func kickoffAgentWorkbookStructure(wb bom.Workbook) kickoffAgentStructure {
	v := kickoffAgentStructure{Sheets: len(wb.Sheets), Regions: bom.NewWorkbookIR(wb).InspectWorkbook()}
	for _, sheet := range wb.Sheets {
		for _, cell := range sheet.Cells {
			if !cell.Sensitive && (cell.Value != "" || cell.Formula != "") {
				v.Cells++
			}
		}
	}
	return v
}

// Main kickoff.go calls this before its historical understanding handler. A
// missing domain-agent job returns false, preserving the old historical path.
func (s *Server) kickoffAgentUnderstanding(w http.ResponseWriter, r *http.Request, source string) bool {
	if !kickoffAgentPermission(w, r, s.project, "bom:read") {
		return true
	}
	job, state, code, _, err := s.kickoffAgentJob(r.Context(), source, "")
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		kickoffAgentFailure(w, err)
		return true
	}
	if requested := understandingRequestedRevision(r); requested != "" && requested != job {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	var report understandingReport
	err = s.kickoffAgentArtifact(r.Context(), job, "kickoff_domain_understanding", &report)
	if errors.Is(err, pgx.ErrNoRows) {
		report = emptyUnderstanding(source, job)
		report.Origin = "agent"
		report.State = "partial"
		report.Limitations = []string{"Agent任务" + state + "；尚未产生的阶段保持待处理，不以模型自评作为完成证据。"}
		if code != nil {
			report.Limitations = append(report.Limitations, *code)
		}
		var structure kickoffAgentStructure
		e := s.kickoffAgentArtifact(r.Context(), job, "kickoff_domain_structure", &structure)
		if e == nil {
			report.Steps[0] = understandingStep{ID: "structure", State: "complete", Count: structure.Sheets, Completed: structure.Sheets, Total: structure.Sheets, Note: fmt.Sprintf("Parser已读取%d张工作表、%d个非敏感单元格", structure.Sheets, structure.Cells)}
			report.Steps[1].Total = len(structure.Regions)
		} else if !errors.Is(e, pgx.ErrNoRows) {
			kickoffAgentFailure(w, e)
			return true
		}
		for i := 1; i < len(report.Steps); i++ {
			report.Steps[i].Note = "任务" + state + "，尚无该阶段确定性结果"
		}
	} else if err != nil {
		kickoffAgentFailure(w, err)
		return true
	}
	respond(w, 200, report)
	return true
}

// The report is a derived read product of immutable Workbook IR and locally
// expanded field evidence. Agent labels alone never complete any of five steps.
func kickoffAgentUnderstandingReport(source, job string, wb bom.Workbook, result ai.KickoffResult) understandingReport {
	structure := kickoffAgentWorkbookStructure(wb)
	report := emptyUnderstanding(source, job)
	report.Origin = "agent"
	report.State = "partial"
	report.Regions = []understandingRegion{}
	report.Limitations = []string{"分域Agent候选，尚未确认或发布；五栏完成数来自Parser、覆盖与本地字段/唯一键证据，不代表资产入库或设备健康。"}
	report.Limitations = append(report.Limitations, result.Missing...)
	if len(report.Limitations) > 101 {
		report.Limitations = append(report.Limitations[:100], "更多缺口保留在完整分析artifact中")
	}
	// Source cells are indexed once; candidate values never substitute for raw
	// source evidence, and sensitive cells never count toward completed fields.
	cells := map[string]bom.Cell{}
	for _, sheet := range wb.Sheets {
		for _, cell := range sheet.Cells {
			cells[sheet.Name+"!"+cell.Ref] = cell
		}
	}
	validFields, totalFields := 0, 0
	missingFields := 0
	relations, validRelations := 0, 0
	sourceIdentities := map[string]int{}
	for _, candidate := range result.Candidates {
		if candidate.TableID == "devices" || candidate.TableID == "components" {
			for _, row := range candidate.Rows {
				if row["serial"] != "" {
					sourceIdentities[row["serial"]]++
				}
			}
		}
	}
	for _, candidate := range result.Candidates {
		for _, e := range candidate.Evidence {
			missingFields += len(e.Missing)
			if e.Row < 1 || e.Row > len(candidate.Rows) {
				continue
			}
			row := candidate.Rows[e.Row-1]
			for field, ref := range e.Fields {
				totalFields++
				cell, exists := cells[ref.Sheet+"!"+ref.Cell]
				valid := exists && !cell.Sensitive && row[field] != "" && (!ref.Formula || ref.CalculationRunID != "")
				if valid && candidate.TableID != "devices" && candidate.TableID != "ports" && candidate.TableID != "connections" && candidate.TableID != "components" {
					valid = kickoff.ValidateInput(candidate.TableID, []map[string]string{{field: row[field]}}) == nil
				}
				if valid {
					validFields++
				}
				if ref.Lookup != nil {
					relations++
					if valid {
						validRelations++
					}
				}
				if cell.FormulaBacked {
					relations++
					if valid && ref.CalculationRunID != "" {
						validRelations++
					}
				}
				if field == "parent_serial" || field == "a_serial" || field == "b_serial" || field == "device_id" {
					relations++
					if valid && sourceIdentities[row[field]] == 1 {
						validRelations++
					}
				}
			}
		}
	}
	coveredRegions, semanticRegions, notApplicableRegions := 0, 0, 0
	for _, region := range structure.Regions {
		from, _ := excelize.CoordinatesToCellName(region.FirstColumn, region.Start)
		to, _ := excelize.CoordinatesToCellName(region.LastColumn, region.End)
		value := understandingRegion{kickoffRegion: kickoffRegion{ID: region.ID, Sheet: region.Sheet, Range: from + ":" + to, Category: "unknown", RowCount: region.End - region.Start + 1, State: "needs_input", Note: "保留原始结构区域；未识别单元格与域外数据计入缺口"}, Categories: []string{}, Mappings: []understandingMapping{}, Issues: []string{}, TotalCells: region.Cells}
		mappings := map[string]int{}
		objects := map[string]bool{}
		mapped := map[string]bool{}
		routed := map[string]bool{}
		applicableRoute := false
		categories := map[string]bool{}
		for _, route := range result.Regions {
			if route.Sheet == region.Sheet && route.Bounds.EndRow >= region.Start && route.Bounds.StartRow <= region.End && route.Bounds.EndColumn >= region.FirstColumn && route.Bounds.StartColumn <= region.LastColumn {
				routed[route.ID] = true
				applicableRoute = applicableRoute || len(route.Steps) > 0
			}
		}
		for _, candidate := range result.Candidates {
			for _, e := range candidate.Evidence {
				if e.Row < 1 || e.Row > len(candidate.Rows) {
					continue
				}
				row := candidate.Rows[e.Row-1]
				touched := false
				for field, ref := range e.Fields {
					cell, ok := cells[ref.Sheet+"!"+ref.Cell]
					if !ok || cell.Sensitive || ref.Sheet != region.Sheet || cell.Row < region.Start || cell.Row > region.End || cell.Column < region.FirstColumn || cell.Column > region.LastColumn {
						continue
					}
					if row[field] != "" && (!ref.Formula || ref.CalculationRunID != "") {
						value.FieldEvidenceCount++
						mappings[field+"_candidate"]++
						mapped[ref.Cell] = true
						touched = true
					}
				}
				if touched {
					objects[fmt.Sprintf("%s:%d", candidate.TableID, e.Row)] = true
					categories[candidate.Domain] = true
				}
				if touched && len(e.Missing) > 0 && len(value.Issues) < 20 {
					value.Issues = append(value.Issues, e.Missing[:min(len(e.Missing), 20-len(value.Issues))]...)
				}
			}
		}
		value.CoveredCells = len(mapped)
		value.EvidenceCount = value.FieldEvidenceCount
		value.ObjectCount = len(objects)
		if len(routed) > 0 {
			coveredRegions++
		}
		if len(routed) > 0 && !applicableRoute {
			notApplicableRegions++
			value.Category = "not_applicable"
			value.State = "extracted"
			value.Note = "已识别为非工程输入区域"
		}
		if len(categories) > 0 {
			semanticRegions++
			for domain := range categories {
				value.Categories = append(value.Categories, domain)
			}
			sort.Strings(value.Categories)
			value.Category = strings.Join(value.Categories, "、")
		}
		if value.CoveredCells > 0 {
			value.State = "partial"
		}
		for field, count := range mappings {
			value.Mappings = append(value.Mappings, understandingMapping{Field: field, Kind: "field", Count: count})
		}
		sort.Slice(value.Mappings, func(i, j int) bool { return value.Mappings[i].Field < value.Mappings[j].Field })
		if value.CoveredCells < value.TotalCells && (len(routed) == 0 || applicableRoute) {
			value.Issues = append(value.Issues, fmt.Sprintf("%d个来源单元格尚无已展开字段证据（包含表头/未支持字段，不自动忽略）", value.TotalCells-value.CoveredCells))
		}
		if len(categories) == 0 && (len(routed) == 0 || applicableRoute) {
			value.Issues = append(value.Issues, "未形成来源支持的语义字段，不能按Agent分类自评为完成")
		}
		if value.Category != "not_applicable" {
			value.Note = fmt.Sprintf("来源字段证据%d条；覆盖%d/%d个单元格；候选对象%d个", value.FieldEvidenceCount, value.CoveredCells, value.TotalCells, value.ObjectCount)
		}
		report.Regions = append(report.Regions, value)
	}
	step := func(id string, completed, total int, note string) understandingStep {
		return understandingStep{ID: id, State: understandingCompletion(completed, total, completed > 0), Count: completed, Completed: completed, Total: total, Note: note}
	}
	report.Steps = []understandingStep{
		step("structure", structure.Sheets, structure.Sheets, fmt.Sprintf("Parser解析%d张工作表、%d个非敏感单元格；隐藏Sheet与区域保留", structure.Sheets, structure.Cells)),
		step("regions", coveredRegions, len(structure.Regions), "按WorkbookIR结构分块核对Agent路由；没有路由的区域保留为缺口"),
		step("semantics", semanticRegions, len(structure.Regions)-notApplicableRegions, "仅有本地展开字段来源的区域计已分类；标签和模型自评不计完成"),
		step("relationships", validRelations, relations, "只计唯一精确来源键/本地lookup和独立重算公式；缺身份、非唯一键、公式缓存不算完成"),
		step("fields", validFields, totalFields+missingFields, "核对候选字段的来源、类型与明确缺失；标题及备注不冒充必填字段，未映射来源仍在区域覆盖与缺口中保留"),
	}
	if relations == 0 && len(result.Missing) == 0 && len(result.Candidates) > 0 {
		report.Steps[3].State = "not_applicable"
		report.Steps[3].Note = "当前来源支持的候选不涉及跨表/公式关联"
	}
	if notApplicableRegions > 0 && notApplicableRegions == len(structure.Regions) {
		for _, i := range []int{2, 3, 4} {
			report.Steps[i].State = "not_applicable"
			report.Steps[i].Note = "所有已识别区域均不属于本次工程输入"
		}
	}
	complete := len(report.Regions) > 0 && len(result.Missing) == 0
	for _, candidate := range result.Candidates {
		complete = complete && len(candidate.Issues) == 0
	}
	for _, step := range report.Steps {
		complete = complete && (step.State == "complete" || step.State == "not_applicable")
	}
	if complete {
		report.State = "recognized"
	}
	return report
}

func (s *Server) kickoffAgentRegionPreview(w http.ResponseWriter, r *http.Request, source, regionID string) bool {
	if !kickoffAgentPermission(w, r, s.project, "bom:read") {
		return true
	}
	job, state, _, _, err := s.kickoffAgentJob(r.Context(), source, "")
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		kickoffAgentFailure(w, err)
		return true
	}
	if requested := understandingRequestedRevision(r); requested != "" && requested != job {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	if state != "succeeded" {
		fail(w, 409, "KICKOFF_ANALYSIS_NOT_READY")
		return true
	}
	var result ai.KickoffResult
	if err = s.kickoffAgentArtifact(r.Context(), job, "kickoff_domain_result", &result); err != nil {
		kickoffAgentFailure(w, err)
		return true
	}
	wb, err := s.kickoffAgentStoredWorkbook(r.Context(), job)
	if err != nil {
		kickoffAgentFailure(w, err)
		return true
	}
	var selected *bom.Region
	regions := bom.NewWorkbookIR(wb).InspectWorkbook()
	for i := range regions {
		if regions[i].ID == regionID {
			selected = &regions[i]
			break
		}
	}
	if selected == nil {
		fail(w, 404, "KICKOFF_REGION_NOT_FOUND")
		return true
	}
	offset, limit := 0, 20
	if v := r.URL.Query().Get("offset"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 {
			fail(w, 400, "KICKOFF_PAGE_INVALID")
			return true
		}
		offset = n
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 100 {
			fail(w, 400, "KICKOFF_PAGE_INVALID")
			return true
		}
		limit = n
	}
	out := understandingPreview{SourceID: source, RevisionID: job, RegionID: regionID, Offset: offset, Limit: limit, Rows: []understandingRow{}}
	if r.URL.Query().Get("view") == "normalized" {
		objects := []understandingRow{}
		for _, candidate := range result.Candidates {
			for _, ref := range candidate.Evidence {
				if ref.Sheet != selected.Sheet || ref.SourceRow < selected.Start || ref.SourceRow > selected.End || ref.Row < 1 || ref.Row > len(candidate.Rows) {
					continue
				}
				row := candidate.Rows[ref.Row-1]
				fields := []understandingField{}
				for key, value := range row {
					if value != "" {
						fields = append(fields, understandingField{Field: key, Value: value})
					}
				}
				sort.Slice(fields, func(i, j int) bool { return fields[i].Field < fields[j].Field })
				// Candidate references are not canonical asset IDs. The UI opens evidence.
				object := understandingObject{ID: fmt.Sprintf("%s:%s:%d", job, candidate.TableID, ref.Row), Kind: "candidate", Label: candidate.TableID, Fields: fields, Evidence: len(ref.Fields)}
				objects = append(objects, understandingRow{Row: len(objects) + 1, Cells: []understandingCell{}, Objects: []understandingObject{object}})
			}
		}
		out.Total = len(objects)
		start := min(offset, len(objects))
		out.Rows = objects[start:min(start+limit, len(objects))]
	} else {
		byRow := map[int][]understandingCell{}
		order := []int{}
		annotated := map[string]map[string]bool{}
		for _, candidate := range result.Candidates {
			for _, ref := range candidate.Evidence {
				for field, cell := range ref.Fields {
					if cell.Sheet == selected.Sheet {
						if annotated[cell.Cell] == nil {
							annotated[cell.Cell] = map[string]bool{}
						}
						annotated[cell.Cell][field+"_candidate"] = true
					}
				}
			}
		}
		for _, sheet := range wb.Sheets {
			if sheet.Name != selected.Sheet {
				continue
			}
			for _, cell := range sheet.Cells {
				if cell.Row < selected.Start || cell.Row > selected.End || cell.Column < selected.FirstColumn || cell.Column > selected.LastColumn {
					continue
				}
				if _, ok := byRow[cell.Row]; !ok {
					order = append(order, cell.Row)
				}
				value, formula := cell.Value, cell.Formula
				if cell.Sensitive {
					value = "[REDACTED]"
					formula = ""
				}
				fields := []string{}
				for field := range annotated[cell.Ref] {
					fields = append(fields, field)
				}
				sort.Strings(fields)
				byRow[cell.Row] = append(byRow[cell.Row], understandingCell{Cell: cell.Ref, Column: cell.Column, Value: value, Formula: formula, Sensitive: cell.Sensitive, Fields: fields, EntityIDs: []string{}})
			}
		}
		sort.Ints(order)
		out.Total = len(order)
		start := min(offset, len(order))
		for _, row := range order[start:min(start+limit, len(order))] {
			out.Rows = append(out.Rows, understandingRow{Row: row, Cells: byRow[row], Objects: []understandingObject{}})
		}
	}
	respond(w, 200, out)
	return true
}
func (s *Server) kickoffAgentStoredWorkbook(ctx context.Context, job string) (bom.Workbook, error) {
	var raw []byte
	var wb bom.Workbook
	err := s.db.QueryRow(ctx, `SELECT b.workbook FROM eng_job j JOIN eng_source e ON e.id=j.source_id JOIN bom_import_batch b ON b.id=e.import_id WHERE j.id::text=$1 AND j.project_id=$2 AND b.project_id=$2`, job, s.project).Scan(&raw)
	if err != nil {
		return wb, err
	}
	err = json.Unmarshal(raw, &wb)
	return wb, err
}
