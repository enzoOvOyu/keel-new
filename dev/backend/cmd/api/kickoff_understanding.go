package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/xuri/excelize/v2"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
)

type understandingStep struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Count     int    `json:"count"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Note      string `json:"note"`
}
type understandingMapping struct {
	Field string `json:"field"`
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}
type understandingRegion struct {
	kickoffRegion
	Categories         []string               `json:"categories"`
	EvidenceCount      int                    `json:"evidenceCount"`
	FieldEvidenceCount int                    `json:"fieldEvidenceCount"`
	ObjectCount        int                    `json:"objectCount"`
	CoveredCells       int                    `json:"coveredCells"`
	TotalCells         int                    `json:"totalCells"`
	Mappings           []understandingMapping `json:"mappings"`
	Issues             []string               `json:"issues"`
}
type understandingReport struct {
	FlowVersion string                `json:"flowVersion,omitempty"`
	SourceID    string                `json:"sourceId"`
	RevisionID  string                `json:"revisionId"`
	State       string                `json:"state"`
	Origin      string                `json:"origin"`
	Steps       []understandingStep   `json:"steps"`
	Regions     []understandingRegion `json:"regions"`
	Limitations []string              `json:"limitations"`
}
type understandingCell struct {
	Cell      string   `json:"cell"`
	Column    int      `json:"column"`
	Value     string   `json:"value"`
	Formula   string   `json:"formula"`
	Sensitive bool     `json:"sensitive"`
	Fields    []string `json:"fields"`
	EntityIDs []string `json:"entityIds"`
}
type understandingField struct {
	Field string `json:"field"`
	Value string `json:"value"`
}
type understandingObject struct {
	ID       string               `json:"id"`
	Kind     string               `json:"kind"`
	Label    string               `json:"label"`
	Fields   []understandingField `json:"fields"`
	Evidence int                  `json:"evidence"`
}
type understandingRow struct {
	Row     int                   `json:"row"`
	Cells   []understandingCell   `json:"cells"`
	Objects []understandingObject `json:"objects"`
}
type understandingPreview struct {
	SourceID   string             `json:"sourceId"`
	RevisionID string             `json:"revisionId"`
	RegionID   string             `json:"regionId"`
	Offset     int                `json:"offset"`
	Limit      int                `json:"limit"`
	Total      int                `json:"total"`
	Rows       []understandingRow `json:"rows"`
}
type understandingRegionData struct {
	Source     []understandingRow `json:"source"`
	Normalized []understandingRow `json:"normalized"`
}

func understandingKey(project, source, revision string) string {
	return "kickoff/understanding/v1/" + project + "/" + source + "/" + revision + "/"
}
func (s *Server) understandingHead(ctx context.Context, source string) (string, error) {
	var head string
	err := s.db.QueryRow(ctx, `SELECT COALESCE(e.head_revision_id::text,'') FROM kickoff_source k LEFT JOIN eng_source e ON e.id=k.id WHERE k.id::text=$1 AND k.project_id=$2 AND k.detached_at IS NULL`, source, s.project).Scan(&head)
	return head, err
}
func (s *Server) understandingRead(ctx context.Context, key string, v any) error {
	obj, err := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer obj.Close()
	return json.NewDecoder(io.LimitReader(obj, 32<<20)).Decode(v)
}
func (s *Server) understandingWrite(ctx context.Context, key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.objects.PutObject(ctx, s.bucket, key, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{ContentType: "application/json"})
	return err
}
func emptyUnderstanding(source, revision string) understandingReport {
	r := understandingReport{SourceID: source, RevisionID: revision, State: "unsupported", Origin: "none", Regions: []understandingRegion{}, Steps: []understandingStep{}, Limitations: []string{"尚未生成历史资料理解报告；复用已有证据不会调用AI或修改工程对象"}}
	for _, id := range []string{"structure", "regions", "semantics", "relationships", "fields"} {
		r.Steps = append(r.Steps, understandingStep{ID: id, State: "pending", Note: "等待历史资料理解"})
	}
	return r
}
func (s *Server) kickoffUnderstanding(w http.ResponseWriter, r *http.Request, source string, build bool) {
	head, err := s.understandingHead(r.Context(), source)
	if err != nil {
		fail(w, 404, "KICKOFF_SOURCE_NOT_FOUND")
		return
	}
	if requested := understandingRequestedRevision(r); requested != "" && requested != head {
		fail(w, 409, "REVISION_CONFLICT")
		return
	}
	report := emptyUnderstanding(source, head)
	key := understandingKey(s.project, source, head)
	err = s.understandingRead(r.Context(), key+"report.json", &report)
	if err != nil && minio.ToErrorResponse(err).Code != "NoSuchKey" {
		fail(w, 503, "KICKOFF_UNDERSTANDING_STORAGE_UNAVAILABLE")
		return
	}
	if err != nil && build {
		if head == "" {
			fail(w, 422, "KICKOFF_HISTORY_NOT_AVAILABLE")
			return
		}
		report, err = s.buildUnderstanding(r.Context(), source, head, key)
		if err != nil {
			if err.Error() == "REVISION_CONFLICT" {
				fail(w, 409, "REVISION_CONFLICT")
				return
			}
			fail(w, 500, "KICKOFF_UNDERSTANDING_UNAVAILABLE")
			return
		}
	}
	respond(w, 200, report)
}
func (s *Server) kickoffRegionPreview(w http.ResponseWriter, r *http.Request, source, region string) {
	head, err := s.understandingHead(r.Context(), source)
	if err != nil {
		fail(w, 404, "KICKOFF_SOURCE_NOT_FOUND")
		return
	}
	if v := understandingRequestedRevision(r); v != "" && v != head {
		fail(w, 409, "REVISION_CONFLICT")
		return
	}
	report := understandingReport{}
	key := understandingKey(s.project, source, head)
	if err = s.understandingRead(r.Context(), key+"report.json", &report); err != nil {
		fail(w, 404, "KICKOFF_UNDERSTANDING_NOT_READY")
		return
	}
	found := false
	for _, v := range report.Regions {
		if v.ID == region {
			found = true
			break
		}
	}
	if !found {
		fail(w, 404, "KICKOFF_REGION_NOT_FOUND")
		return
	}
	data := understandingRegionData{}
	if err = s.understandingRead(r.Context(), key+region+".json", &data); err != nil {
		fail(w, 503, "KICKOFF_REGION_UNAVAILABLE")
		return
	}
	rows := data.Source
	if r.URL.Query().Get("view") == "normalized" {
		rows = data.Normalized
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	total := len(rows)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	respond(w, 200, understandingPreview{SourceID: source, RevisionID: head, RegionID: region, Offset: offset, Limit: limit, Total: total, Rows: rows[offset:end]})
}

// Builds a disposable read product only: no engineering, input, publication or CMDB writes.
func (s *Server) buildUnderstanding(ctx context.Context, source, head, key string) (understandingReport, error) {
	report := emptyUnderstanding(source, head)
	report.Origin = "history"
	report.State = "partial"
	var objectKey string
	err := s.db.QueryRow(ctx, `SELECT k.object_key FROM kickoff_source k JOIN eng_source e ON e.id=k.id WHERE k.id::text=$1 AND k.project_id=$2 AND e.head_revision_id::text=$3`, source, s.project, head).Scan(&objectKey)
	if err != nil {
		return report, err
	}
	obj, err := s.objects.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return report, err
	}
	raw, err := io.ReadAll(io.LimitReader(obj, 32<<20))
	obj.Close()
	if err != nil {
		return report, err
	}
	workbook, err := bom.Parse(raw)
	if err != nil {
		return report, err
	}
	model, evidence, err := engineeringModel(ctx, s.db, head, s.project)
	if err != nil {
		return report, err
	}
	// Replay saved explicit Columns/Records using the existing deterministic extractor.
	// Extracted values remain candidates; user-edited current objects are never replaced.
	var proposalRaw []byte
	if pe := s.db.QueryRow(ctx, `SELECT COALESCE(original.proposal,current.proposal) FROM eng_revision current JOIN eng_source es ON es.id=current.source_id LEFT JOIN kickoff_history_import h ON h.source_id=es.id LEFT JOIN eng_revision original ON original.id=h.original_revision_id WHERE current.id::text=$1`, head).Scan(&proposalRaw); pe == nil {
		var proposal engineering.EngineeringProposal
		if json.Unmarshal(proposalRaw, &proposal) == nil && len(proposal.Regions) > 0 {
			if extracted, pe := engineering.ExtractPhysical(workbook, proposal, s.project, source, head); pe == nil {
				for _, ev := range extracted.Evidence {
					ev.Field += "_candidate"
					evidence = append(evidence, ev)
				}
			}
		}
	}
	evidence = append(evidence, restoreIdentityEvidence(workbook, model)...)
	// The historical reviewed_model tag is object-level. Never upgrade it into field evidence.
	kinds := map[string]string{}
	labels := map[string]string{}
	rows, err := s.db.Query(ctx, `SELECT entity_id::text,kind FROM eng_entity_version WHERE revision_id=$1`, head)
	if err != nil {
		return report, err
	}
	for rows.Next() {
		var id, kind string
		if err = rows.Scan(&id, &kind); err != nil {
			break
		}
		if kind == "equipment" {
			kind = "device"
		}
		if kind == "assembly" {
			kind = "component"
		}
		kinds[id] = kind
		labels[id] = id
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, err
	}
	for _, v := range model.Equipment {
		labels[v.EntityID] = v.Label
		switch strings.ToLower(v.EquipmentType) {
		case "gpu", "nic", "cpu", "dimm", "disk", "optic", "psu", "fan", "component":
			kinds[v.EntityID] = "component"
		}
	}
	for _, v := range model.Facilities {
		labels[v.EntityID] = v.Label
		kinds[v.EntityID] = v.Kind
	}
	byCell := map[string][]engineering.SourceEvidence{}
	for _, ev := range evidence {
		byCell[ev.Sheet+"!"+ev.Cell] = append(byCell[ev.Sheet+"!"+ev.Cell], ev)
	}
	ir := bom.NewWorkbookIR(workbook)
	sheets := map[string]bom.Sheet{}
	for _, sh := range workbook.Sheets {
		sheets[sh.Name] = sh
	}
	classified, covered, total, objectCount := 0, 0, 0, 0
	for _, region := range ir.Regions {
		id := engineering.StableID(source, "understanding-region", region.ID+region.Range)
		v := understandingRegion{kickoffRegion: kickoffRegion{ID: id, Sheet: region.Sheet, Range: region.Range, RowCount: region.End - region.Start + 1, Category: "待分类", State: "unmapped", Note: "基于非空行与既有证据恢复；结构分块不等于完整业务表"}, Categories: []string{}, Mappings: []understandingMapping{}, Issues: []string{}, TotalCells: region.Cells}
		data := understandingRegionData{Source: []understandingRow{}, Normalized: []understandingRow{}}
		objects := map[string]*understandingObject{}
		cats := map[string]bool{}
		maps := map[string]int{}
		sourceRows := map[int][]understandingCell{}
		for _, cell := range sheets[region.Sheet].Cells {
			if cell.Row < region.Start || cell.Row > region.End || cell.Column < region.FirstColumn || cell.Column > region.LastColumn {
				continue
			}
			out := understandingCell{Cell: cell.Ref, Column: cell.Column, Value: cell.Value, Formula: cell.Formula, Sensitive: cell.Sensitive, Fields: []string{}, EntityIDs: []string{}}
			if cell.Sensitive {
				out.Value = "[已隐藏]"
				out.Formula = ""
			} else {
				evs := byCell[region.Sheet+"!"+cell.Ref]
				if len(evs) > 0 {
					v.CoveredCells++
				}
				for _, ev := range evs {
					kind := kinds[ev.EntityID]
					if kind == "" {
						continue
					}
					v.EvidenceCount++
					out.EntityIDs = append(out.EntityIDs, ev.EntityID)
					obj := objects[ev.EntityID]
					if obj == nil {
						obj = &understandingObject{ID: ev.EntityID, Kind: kind, Label: labels[ev.EntityID], Fields: []understandingField{}}
						objects[ev.EntityID] = obj
					}
					obj.Evidence++
					cats[kind] = true
					scope := "object"
					if ev.Field != "" && ev.Field != "reviewed_model" && ev.Field != "source" && ev.Field != "record" && !strings.HasSuffix(ev.Field, "_candidate") {
						scope = "field"
						out.Fields = append(out.Fields, ev.Field)
						obj.Fields = append(obj.Fields, understandingField{Field: ev.Field, Value: cell.Value})
						v.FieldEvidenceCount++
					}
					if strings.HasSuffix(ev.Field, "_candidate") {
						out.Fields = append(out.Fields, ev.Field)
						obj.Fields = append(obj.Fields, understandingField{Field: ev.Field, Value: cell.Value})
					}
					maps[scope+":"+ev.Field]++
				}
			}
			sourceRows[cell.Row] = append(sourceRows[cell.Row], out)
		}
		order := []int{}
		for row := range sourceRows {
			order = append(order, row)
		}
		sort.Ints(order)
		for _, row := range order {
			cells := sourceRows[row]
			sort.Slice(cells, func(i, j int) bool { return cells[i].Column < cells[j].Column })
			data.Source = append(data.Source, understandingRow{Row: row, Cells: cells, Objects: []understandingObject{}})
		}
		ids := []string{}
		for id := range objects {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			data.Normalized = append(data.Normalized, understandingRow{Cells: []understandingCell{}, Objects: []understandingObject{*objects[id]}})
		}
		for category := range cats {
			v.Categories = append(v.Categories, category)
		}
		sort.Strings(v.Categories)
		v.ObjectCount = len(objects)
		for key, count := range maps {
			bits := strings.SplitN(key, ":", 2)
			v.Mappings = append(v.Mappings, understandingMapping{Field: bits[1], Kind: bits[0], Count: count})
		}
		sort.Slice(v.Mappings, func(i, j int) bool {
			return v.Mappings[i].Kind+v.Mappings[i].Field < v.Mappings[j].Kind+v.Mappings[j].Field
		})
		if len(cats) > 0 {
			v.Category = strings.Join(v.Categories, " / ")
			v.State = "needs_input"
			classified++
		}
		if v.FieldEvidenceCount == 0 && v.ObjectCount > 0 {
			v.Issues = append(v.Issues, "仅有对象级证据，不能证明字段映射或整表覆盖")
		}
		if v.CoveredCells < v.TotalCells {
			v.Issues = append(v.Issues, "存在尚未关联的单元格，不能认定整表已处理")
		}
		total += v.TotalCells
		covered += v.CoveredCells
		objectCount += len(objects)
		if err = s.understandingWrite(ctx, key+id+".json", data); err != nil {
			return report, err
		}
		report.Regions = append(report.Regions, v)
	}
	issues := append(engineering.ValidatePhysical(model), engineering.ValidateConnectivity(model)...)
	invalid := map[string]bool{}
	for _, issue := range issues {
		invalid[issue.EntityID] = true
	}
	connectionIDs := map[string]bool{}
	for _, v := range model.Connections {
		connectionIDs[v.EntityID] = true
	}
	supported := map[string]bool{}
	for _, ev := range evidence {
		if connectionIDs[ev.EntityID] && !strings.HasPrefix(ev.Sheet, "[") {
			if _, _, err := excelize.CellNameToCoordinates(ev.Cell); err == nil {
				supported[ev.EntityID] = true
			}
		}
	}
	validated := 0
	for id := range supported {
		if !invalid[id] {
			validated++
		}
	}
	semanticState := understandingCompletion(classified, len(report.Regions), false)
	relationState := understandingCompletion(validated, len(model.Connections), len(supported) > 0)
	if len(model.Connections) == 0 {
		relationState = "unsupported"
	}
	actualCells := map[string]bool{}
	for _, sheet := range workbook.Sheets {
		for _, cell := range sheet.Cells {
			if !cell.Sensitive && (cell.Value != "" || cell.Formula != "") {
				actualCells[sheet.Name+"!"+cell.Ref] = true
			}
		}
	}
	mapped := map[string]bool{}
	mappedValid := map[string]bool{}
	failedFields := map[string]bool{}
	for _, issue := range issues {
		failedFields[issue.EntityID+"/"+issue.Field] = true
	}
	for _, ev := range evidence {
		if !actualCells[ev.Sheet+"!"+ev.Cell] || kinds[ev.EntityID] == "" || ev.Field == "" || ev.Field == "reviewed_model" || strings.HasSuffix(ev.Field, "_candidate") || strings.HasPrefix(ev.Sheet, "[") {
			continue
		}
		if _, _, e := excelize.CellNameToCoordinates(ev.Cell); e != nil {
			continue
		}
		k := ev.EntityID + "/" + ev.Field
		mapped[k] = true
		if !failedFields[k] {
			mappedValid[k] = true
		}
	}
	fieldState := understandingCompletion(len(mappedValid), len(mapped), covered > 0)
	report.Steps = []understandingStep{
		{ID: "structure", State: "complete", Count: len(workbook.Sheets), Completed: len(workbook.Sheets), Total: len(workbook.Sheets), Note: "已恢复工作表、合并单元格和公式；敏感值不返回"},
		{ID: "regions", State: "complete", Count: len(report.Regions), Completed: len(report.Regions), Total: len(report.Regions), Note: "已按空白分隔和有界行分块定位有效区域；不宣称完整业务表识别"},
		{ID: "semantics", State: semanticState, Count: classified, Completed: classified, Total: len(report.Regions), Note: "类别仅来自现有类型化对象的精确单元格证据；未按Sheet名称猜测"},
		{ID: "relationships", State: relationState, Count: len(model.Connections), Completed: validated, Total: len(model.Connections), Note: fmt.Sprintf("现有%d条连接；恢复同原始行双端唯一身份候选%d条，其中确定性检查无问题%d条，未关联%d条；候选未经人工确认，不代表全部跨表关系已发现", len(model.Connections), len(supported), validated, len(model.Connections)-len(supported))},
		{ID: "fields", State: fieldState, Count: len(mapped), Completed: len(mappedValid), Total: len(mapped), Note: fmt.Sprintf("校验范围为已有明确映射的%d个对象字段，通过%d个；原件唯一身份匹配只作重建候选、不纳入已映射字段分母；全局模型问题%d项另行保留", len(mapped), len(mappedValid), len(issues))},
	}
	if semanticState == "complete" && (relationState == "complete" || relationState == "unsupported") && fieldState == "complete" {
		report.State = "historical"
	}
	report.Limitations = []string{fmt.Sprintf("有证据关联%d/%d个非空单元格，区域对象引用共%d个（可跨区域重复）；不等于整表覆盖", covered, total, objectCount), "本报告复用历史资料与候选，未调用AI，未修改已确认数据或发布资产", "connection_candidate仅为同原始行的两个唯一物理端口与已有连接端点精确组合匹配；不会由设备名推断连接", "serial_candidate/mac_candidate仅为原件与当前对象唯一身份值精确匹配的候选定位，尚未人工确认；其余对象级历史证据未定位到单元格，不伪称字段映射", "字段映射缺失和未分类区域需后续确认或Agent处理；历史已入库不是识别完成的依据"}
	current, e := s.understandingHead(ctx, source)
	if e != nil {
		return report, e
	}
	if current != head {
		return report, errors.New("REVISION_CONFLICT")
	}
	if err = s.understandingWrite(ctx, key+"report.json", report); err != nil {
		return report, err
	}
	return report, nil
}

// Exact unique identity equality is a reconstructed candidate, not proof that a
// whole row was mapped. Duplicate identities deliberately produce no match.
func restoreIdentityEvidence(w bom.Workbook, m engineering.PhysicalModel) []engineering.SourceEvidence {
	type identity struct{ id, field string }
	index := map[string][]identity{}
	for _, v := range m.Equipment {
		if v.Serial != nil && strings.TrimSpace(*v.Serial) != "" {
			key := strings.TrimSpace(*v.Serial)
			index[key] = append(index[key], identity{v.EntityID, "serial_candidate"})
		}
	}
	for _, v := range m.Ports {
		if v.MAC != nil && strings.TrimSpace(*v.MAC) != "" {
			key := strings.ToLower(strings.TrimSpace(*v.MAC))
			index[key] = append(index[key], identity{v.EntityID, "mac_candidate"})
		}
	}
	out := []engineering.SourceEvidence{}
	for _, sh := range w.Sheets {
		for _, c := range sh.Cells {
			if c.Sensitive || c.Formula != "" {
				continue
			}
			key := strings.TrimSpace(c.Value)
			matches := index[key]
			if len(matches) == 0 {
				for _, match := range index[strings.ToLower(key)] {
					if match.field == "mac_candidate" {
						matches = append(matches, match)
					}
				}
			}
			if len(matches) != 1 {
				continue
			}
			v := matches[0]
			out = append(out, engineering.SourceEvidence{ID: engineering.StableID(v.id, "historical-identity", sh.Name+"!"+c.Ref), EntityID: v.id, Sheet: sh.Name, Cell: c.Ref, Field: v.field})
		}
	}
	// Only two unique physical-port identities on the same original row can
	// reconstruct a connection candidate. No labels or fuzzy matching are used.
	ports := map[string]bool{}
	for _, v := range m.Ports {
		ports[v.EntityID] = true
	}
	rowPorts := map[string]map[string]engineering.SourceEvidence{}
	for _, ev := range out {
		if !ports[ev.EntityID] {
			continue
		}
		_, row, err := excelize.CellNameToCoordinates(ev.Cell)
		if err != nil {
			continue
		}
		key := ev.Sheet + "!" + strconv.Itoa(row)
		if rowPorts[key] == nil {
			rowPorts[key] = map[string]engineering.SourceEvidence{}
		}
		rowPorts[key][ev.EntityID] = ev
	}
	links := map[string][]string{}
	pair := func(a, b string) string {
		if a > b {
			a, b = b, a
		}
		return a + "/" + b
	}
	for _, v := range m.Connections {
		key := pair(v.APortID, v.ZPortID)
		links[key] = append(links[key], v.EntityID)
	}
	for _, row := range rowPorts {
		if len(row) != 2 {
			continue
		}
		ids := []string{}
		for id := range row {
			ids = append(ids, id)
		}
		matches := links[pair(ids[0], ids[1])]
		if len(matches) != 1 {
			continue
		}
		ev := row[ids[0]]
		ev.EntityID = matches[0]
		ev.Field = "connection_candidate"
		ev.ID = engineering.StableID(ev.EntityID, "historical-link", ev.Sheet+"!"+ev.Cell)
		out = append(out, ev)
	}
	return out
}

func understandingRequestedRevision(r *http.Request) string {
	if v := r.URL.Query().Get("revisionId"); v != "" {
		return v
	}
	return r.URL.Query().Get("revision_id")
}

func understandingCompletion(completed, total int, candidates bool) string {
	if total > 0 && completed == total {
		return "complete"
	}
	if completed > 0 || candidates {
		return "partial"
	}
	return "pending"
}
