package main

import (
	"fmt"
	"strings"

	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/kickoff"
)

// Versioned read projection only. Neither task success nor model confidence is
// evidence of complete candidates; optional absence is not an execution failure.
func kickoffTerminationUnderstanding(report *understandingReport, wb bom.Workbook, result ai.KickoffResult) {
	report.FlowVersion = "termination-v1"
	cells := map[string]bom.Cell{}
	for _, sheet := range wb.Sheets {
		for _, cell := range sheet.Cells {
			cells[sheet.Name+"!"+cell.Ref] = cell
		}
	}
	supported := func(ref ai.KickoffAgentCell) bool {
		cell, ok := cells[ref.Sheet+"!"+ref.Cell]
		if !ok || cell.Sensitive || (cell.Value == "" && cell.Formula == "") {
			return false
		}
		if (ref.Formula || cell.Formula != "" || cell.FormulaBacked) && ref.CalculationRunID == "" {
			return false
		}
		for _, evidence := range ref.RelationshipEvidence {
			c, exists := cells[evidence.Sheet+"!"+evidence.Cell]
			if !exists || c.Sensitive || (c.Value == "" && c.Formula == "") {
				return false
			}
		}
		return true
	}
	type rowEvidence struct {
		table  string
		row    map[string]string
		fields map[string]ai.KickoffAgentCell
		issues []string
	}
	rows := []rowEvidence{}
	devices := map[string]int{}
	ports := map[string]int{}
	racks := map[string]int{}
	for _, candidate := range result.Candidates {
		evidenceByRow := map[int]map[string]ai.KickoffAgentCell{}
		for _, ev := range candidate.Evidence {
			if evidenceByRow[ev.Row] == nil {
				evidenceByRow[ev.Row] = map[string]ai.KickoffAgentCell{}
			}
			for field, ref := range ev.Fields {
				evidenceByRow[ev.Row][field] = ref
			}
		}
		issuesByRow := map[int][]int{}
		globalIssueFields := map[string]bool{}
		for index, issue := range candidate.Issues {
			if issue.Row == 0 {
				globalIssueFields[issue.Field] = true
			} else {
				issuesByRow[issue.Row] = append(issuesByRow[issue.Row], index)
			}
		}
		for i, row := range candidate.Rows {
			r := rowEvidence{table: candidate.TableID, row: row, fields: map[string]ai.KickoffAgentCell{}}
			for field, ref := range evidenceByRow[i+1] {
				r.fields[field] = ref
			}
			for field := range globalIssueFields {
				if field == "" || strings.TrimSpace(row[field]) != "" || terminationRequired(candidate.TableID, field) {
					r.issues = append(r.issues, "candidate_issue")
					break
				}
			}
			for _, index := range issuesByRow[i+1] {
				issue := candidate.Issues[index]
				// A missing optional value is retained in limitations, not a failed row.
				if issue.Field != "" && strings.TrimSpace(row[issue.Field]) == "" && !terminationRequired(candidate.TableID, issue.Field) {
					continue
				}
				r.issues = append(r.issues, issue.Code)
			}
			// Serial remains an explicit supported identity, never a manufactured value.
			for canonical, legacy := range map[string]string{"device_ref": "serial", "a_device_ref": "a_serial", "b_device_ref": "b_serial"} {
				if row[canonical] == "" && row[legacy] != "" {
					copied := map[string]string{}
					for key, value := range row {
						copied[key] = value
					}
					row = copied
					r.row = row
					row[canonical] = row[legacy]
					if ref, ok := r.fields[legacy]; ok {
						r.fields[canonical] = ref
					}
				}
			}
			rows = append(rows, r)
			switch r.table {
			case "devices":
				if row["device_ref"] != "" {
					devices[kickoff.NormalizeDeviceRef(row["device_ref"])]++
				}
			case "ports":
				ports[kickoff.NormalizeDeviceRef(row["device_ref"])+"\x00"+row["port"]]++
			case "racks":
				if row["rack_id"] != "" {
					racks[row["rack_id"]]++
				}
			}
		}
	}
	fieldOK := func(r rowEvidence, field string) bool {
		ref, ok := r.fields[field]
		return strings.TrimSpace(r.row[field]) != "" && ok && supported(ref)
	}
	deviceOK := map[string]bool{}
	portOK := map[string]bool{}
	for _, r := range rows {
		if r.table == "devices" && fieldOK(r, "device_ref") && devices[kickoff.NormalizeDeviceRef(r.row["device_ref"])] == 1 && len(r.issues) == 0 {
			deviceOK[kickoff.NormalizeDeviceRef(r.row["device_ref"])] = true
		}
	}
	for _, r := range rows {
		if r.table == "ports" && fieldOK(r, "device_ref") && fieldOK(r, "port") && deviceOK[kickoff.NormalizeDeviceRef(r.row["device_ref"])] && len(r.issues) == 0 {
			key := kickoff.NormalizeDeviceRef(r.row["device_ref"]) + "\x00" + r.row["port"]
			portOK[key] = ports[key] == 1
		}
	}
	relationOK := func(r rowEvidence) bool {
		for _, field := range []string{"a_device_ref", "a_port", "b_device_ref", "b_port"} {
			if !fieldOK(r, field) {
				return false
			}
		}
		a, b := kickoff.NormalizeDeviceRef(r.row["a_device_ref"])+"\x00"+r.row["a_port"], kickoff.NormalizeDeviceRef(r.row["b_device_ref"])+"\x00"+r.row["b_port"]
		return a != b && portOK[a] && portOK[b] && len(r.issues) == 0
	}
	done, total := [5]int{}, [5]int{}
	// Localization requires an actual bounded route and a recorded coverage result;
	// merely parsing a workbook cannot complete this business step.
	irRegions := bom.NewWorkbookIR(wb).InspectWorkbook()
	for _, region := range report.Regions {
		total[0]++
		located := false
		for _, route := range result.Regions {
			if route.Sheet != region.Sheet {
				continue
			}
			for _, coverage := range result.Coverage {
				if coverage.Region != route.ID {
					continue
				}
				// Region IDs in this report come directly from WorkbookIR. Bounds may be
				// narrower after semantic routing, so check their overlap with the IR.
				for _, raw := range irRegions {
					if raw.ID == region.ID && route.Bounds.StartRow <= raw.End && route.Bounds.EndRow >= raw.Start && route.Bounds.StartColumn <= raw.LastColumn && route.Bounds.EndColumn >= raw.FirstColumn {
						located = true
					}
				}
			}
		}
		if located {
			done[0]++
		}
	}
	for _, r := range rows {
		valid := len(r.issues) == 0
		if r.table != "devices" && r.table != "ports" && r.table != "connections" && r.table != "components" {
			valid = valid && kickoff.ValidateInput(r.table, []map[string]string{r.row}) == nil
		}
		for field, value := range r.row {
			if strings.TrimSpace(value) != "" && !fieldOK(r, field) {
				valid = false
			}
		}
		switch r.table {
		case "devices":
			total[1]++
			valid = valid && deviceOK[kickoff.NormalizeDeviceRef(r.row["device_ref"])]
			if valid {
				done[1]++
			}
		case "ports":
			total[1]++
			valid = valid && portOK[kickoff.NormalizeDeviceRef(r.row["device_ref"])+"\x00"+r.row["port"]]
			if valid {
				done[1]++
			}
		case "connections":
			total[2]++
			valid = valid && relationOK(r)
			if valid {
				done[2]++
			}
			for _, field := range []string{"cable_model", "cable_length_m", "medium", "a_optic_model", "b_optic_model"} {
				if strings.TrimSpace(r.row[field]) != "" {
					total[3]++
					if relationOK(r) && fieldOK(r, field) {
						done[3]++
					}
				}
			}
		case "placements":
			total[3]++
			valid = valid && fieldOK(r, "device_id") && fieldOK(r, "rack_id") && deviceOK[kickoff.NormalizeDeviceRef(r.row["device_id"])] && racks[r.row["rack_id"]] == 1
			if valid {
				done[3]++
			}
		}
		total[4]++
		if valid {
			done[4]++
		}
	}
	// Unexpanded applicable regions remain genuine gaps even with some good rows.
	for _, coverage := range result.Coverage {
		if coverage.State == "needs_input" && coverage.Rows == 0 {
			total[4]++
		}
	}
	ids := []string{"structure", "regions", "semantics", "relationships", "fields"}
	notes := []string{
		"端接资料定位：按实际区域路由与覆盖记录核对；Parser成功不等于完成定位",
		"设备与端口识别：核对device_ref唯一身份、端口归属与来源；SN、型号等可选信息缺失不作失败",
		"关系推理与佐证：核对连接两端设备/端口唯一引用和来源，公式须有重算证据；不计模型自评",
		"位置与线缆关联：只核对实际提供的位置和线缆信息；未提供机柜/U位/长度不猜测、不作失败",
		"候选完整性核对：核对实际候选行的必需身份、已提供字段证据、候选问题及未展开区域；可选缺口仅提示",
	}
	report.Steps = []understandingStep{}
	complete := total[0] > 0
	for i, id := range ids {
		state := understandingCompletion(done[i], total[i], total[i] > 0)
		if total[i] == 0 && i > 0 {
			state = "not_applicable"
			if i == 1 && total[2] > 0 {
				state = "pending"
			}
		}
		report.Steps = append(report.Steps, understandingStep{ID: id, State: state, Count: done[i], Completed: done[i], Total: total[i], Note: fmt.Sprintf("%s（%d/%d）", notes[i], done[i], total[i])})
		complete = complete && (state == "complete" || state == "not_applicable")
	}
	// No candidates with applicable coverage must never look like recognition.
	for _, coverage := range result.Coverage {
		if coverage.State != "not_applicable" && len(rows) == 0 {
			complete = false
			report.Steps[4].State = "partial"
		}
	}
	report.State = "partial"
	if complete {
		report.State = "recognized"
	}
	report.Limitations = append([]string{"termination-v1端接理解候选；完成不代表人工采用、CMDB发布或任务执行成功。可选信息缺失保留提示，不记为任务失败。"}, result.Missing...)
	if len(report.Limitations) > 101 {
		report.Limitations = append(report.Limitations[:100], "更多缺口保留在完整分析artifact中")
	}
}

func terminationRequired(table, field string) bool {
	required := map[string][]string{"devices": {"device_ref"}, "ports": {"device_ref", "port"}, "connections": {"a_device_ref", "a_port", "b_device_ref", "b_port"}, "placements": {"device_id", "rack_id"}}
	for _, f := range required[table] {
		if f == field {
			return true
		}
	}
	return false
}
