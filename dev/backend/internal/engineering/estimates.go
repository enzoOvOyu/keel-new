package engineering

import (
	"fmt"
	"github.com/google/uuid"
	"math/big"
	"sort"
	"strings"
	"time"
)

func extractEstimates(project string, rows []rowData, out *Extraction, version func(string, rowData, ...string) EntityVersion, issue func(string, string, string, string, string)) {
	materials := map[string]int{}
	workloads := map[string]int{}
	for _, r := range rows {
		if r.kind != "material" && r.kind != "workload" {
			continue
		}
		v := r.values
		label := v["model"]
		if label == "" {
			label = v["label"]
		}
		id := StableID(project, r.kind, label+"/"+v["work_type"]+"/"+v["standard_length"]+"/"+v["standard_length_unit"])
		target := MaterialTarget
		if r.kind == "workload" {
			target = WorkloadTarget
		}
		qty := func(field, unitField string) *Quantity {
			if v[field] == "" {
				return nil
			}
			unit := r.units[field]
			if unit == "" {
				unit = v[unitField]
			}
			q := Quantity{v[field], unit}
			if q.Validate() != nil {
				issue(id, "ESTIMATE_QUANTITY_UNRESOLVED", field, "估算数值或单位不明确", target)
				return nil
			}
			return &q
		}
		base := version(id, r, "label", "model", "required_quantity", "purchased_quantity", "spare_quantity", "purchase_gap", "quantity_unit", "work_type", "work_quantity", "work_unit", "standard_length", "standard_length_unit", "labor_hours", "labor_unit")
		if label == "" {
			issue(id, "ESTIMATE_SPEC_REQUIRED", "model", "估算缺少规格或工作说明", target)
		}
		if r.kind == "material" {
			required := qty("required_quantity", "quantity_unit")
			if required == nil {
				issue(id, "MATERIAL_REQUIRED_MISSING", "required_quantity", "需求数量与单位待明确", target)
				continue
			}
			m := MaterialEstimate{EntityVersion: base, SpecRef: StableID(project, "material-spec", label), SourceModelLabel: label, Required: *required, Purchased: qty("purchased_quantity", "quantity_unit"), Spares: qty("spare_quantity", "quantity_unit"), PurchaseGap: qty("purchase_gap", "quantity_unit")}
			if i, ok := materials[id]; ok {
				old := out.Model.Materials[i]
				old.EntityVersion = m.EntityVersion
				if Digest(old) != Digest(m) {
					issue(id, "MATERIAL_SOURCE_CONFLICT", "required_quantity", "同一规格多处需求冲突，不能默认为可相加", MaterialTarget)
				}
				out.Model.Materials[i].EvidenceIDs = append(out.Model.Materials[i].EvidenceIDs, m.EvidenceIDs...)
			} else {
				materials[id] = len(out.Model.Materials)
				out.Model.Materials = append(out.Model.Materials, m)
			}
		} else {
			q := qty("work_quantity", "work_unit")
			if q == nil {
				issue(id, "WORKLOAD_QUANTITY_MISSING", "work_quantity", "施工数量与单位待明确", target)
				continue
			}
			w := WorkloadEstimate{EntityVersion: base, SourceModelLabel: label, WorkType: v["work_type"], Quantity: *q, StandardLength: qty("standard_length", "standard_length_unit"), LaborHours: qty("labor_hours", "labor_unit")}
			if i, ok := workloads[id]; ok {
				old := out.Model.Workloads[i]
				old.EntityVersion = w.EntityVersion
				if Digest(old) != Digest(w) {
					issue(id, "WORKLOAD_SOURCE_CONFLICT", "work_quantity", "同一施工条目来源冲突", WorkloadTarget)
				}
				out.Model.Workloads[i].EvidenceIDs = append(out.Model.Workloads[i].EvidenceIDs, w.EvidenceIDs...)
			} else {
				workloads[id] = len(out.Model.Workloads)
				out.Model.Workloads = append(out.Model.Workloads, w)
			}
		}
	}
}
func ValidateEstimates(m PhysicalModel) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(v EntityVersion, code, field, message, target string) {
		issues = append(issues, ValidationIssue{ID: StableID(m.ProjectID, "estimate-issue", v.EntityID+"/"+code+"/"+field), EntityID: v.EntityID, ProjectID: m.ProjectID, RevisionID: v.RevisionID, Code: code, Severity: "BLOCK", ModuleID: "material_workload", Field: field, Message: message, PublishTarget: target, EvidenceIDs: v.EvidenceIDs})
	}
	nonnegative := func(q Quantity) bool { n, e := measure(q); return e == nil && n.value.Sign() >= 0 }
	for _, v := range m.Materials {
		if v.SourceModelLabel == "" || v.SpecRef == "" {
			add(v.EntityVersion, "MATERIAL_SPEC_REQUIRED", "source_model_label", "物料需求必须有明确规格", MaterialTarget)
		}
		if !nonnegative(v.Required) {
			add(v.EntityVersion, "MATERIAL_QUANTITY_INVALID", "required", "需求数量与单位无效", MaterialTarget)
		}
		for field, q := range map[string]*Quantity{"purchased": v.Purchased, "spares": v.Spares, "purchase_gap": v.PurchaseGap} {
			if q == nil {
				continue
			}
			a, ea := measure(v.Required)
			b, eb := measure(*q)
			if !nonnegative(*q) || ea != nil || eb != nil || a.dimension != b.dimension {
				add(v.EntityVersion, "MATERIAL_UNIT_MISMATCH", field, "需求、采购、备件和缺口需同维度且非负", MaterialTarget)
			}
		}
	}
	for _, v := range m.Workloads {
		if v.WorkType == "" {
			add(v.EntityVersion, "WORK_TYPE_REQUIRED", "work_type", "工作类型必须有明确依据", WorkloadTarget)
		}
		if !nonnegative(v.Quantity) {
			add(v.EntityVersion, "WORKLOAD_QUANTITY_INVALID", "quantity", "工程量必须是非负数值且单位明确", WorkloadTarget)
		}
		if v.StandardLength != nil {
			n, e := measure(*v.StandardLength)
			if e != nil || n.dimension != "length" || n.value.Sign() < 0 {
				add(v.EntityVersion, "WORKLOAD_LENGTH_INVALID", "standard_length", "标准线长需长度单位", WorkloadTarget)
			}
		}
		if v.LaborHours != nil {
			n, e := measure(*v.LaborHours)
			if e != nil || n.dimension != "time" || n.value.Sign() < 0 {
				add(v.EntityVersion, "WORKLOAD_TIME_INVALID", "labor_hours", "工时需明确时间单位", WorkloadTarget)
			}
		}
	}
	return issues
}

// DerivedWorkloads describes planning quantities only. It never supplies labor
// rates, spare ratios, procurement policy, or claims completed installation.
func DerivedWorkloads(m PhysicalModel) ([]WorkloadEstimate, []CalculationError) {
	groups := map[string]WorkloadEstimate{}
	errors := []CalculationError{}
	add := func(kind, label string, spec *string, length *Quantity, count int64) {
		key := kind + "/" + label
		if spec != nil {
			key += "/" + *spec
		}
		if length != nil {
			v, e := measure(*length)
			if e != nil || v.dimension != "length" {
				return
			}
			normalized, _ := resultValue(v, "m")
			key += "/" + *normalized.Decimal
			length = &Quantity{*normalized.Decimal, "m"}
		}
		id := StableID(m.ProjectID, "derived-workload", key)
		w, ok := groups[id]
		if !ok {
			w = WorkloadEstimate{EntityVersion: EntityVersion{EntityID: id, Maturity: Calculated, EvidenceIDs: []string{}, Attributes: []AttributeValue{}}, WorkType: kind, SourceModelLabel: label, SpecRef: spec, StandardLength: length, Quantity: Quantity{"0", "count"}}
		}
		n, _ := new(big.Int).SetString(w.Quantity.Value, 10)
		n.Add(n, big.NewInt(count))
		w.Quantity.Value = n.String()
		groups[id] = w
	}
	for _, e := range m.Equipment {
		if e.Quantity < 1 {
			errors = append(errors, CalculationError{Code: "WORKLOAD_EQUIPMENT_QUANTITY_UNKNOWN", Message: "设备数量未明确"})
			continue
		}
		add("equipment_install", strings.TrimSpace(e.EquipmentType+" "+e.SourceModelLabel), nil, nil, e.Quantity)
	}
	cables := map[string]CablePlan{}
	for _, c := range m.Cables {
		cables[c.ConnectionID] = c
	}
	for _, c := range m.Connections {
		segments := int64(1 + len(c.PatchPortIDs)/2)
		add("cable_install", "物理外部线段", nil, nil, segments)
		plan, ok := cables[c.EntityID]
		if !ok || plan.SelectedStock == nil || len(c.PatchPortIDs) > 0 {
			errors = append(errors, CalculationError{Code: "WORKLOAD_STOCK_DISTRIBUTION_INCOMPLETE", Message: "连接 " + c.EntityID + " 缺少逐物理线段的明确标准规格"})
			continue
		}
		add("cable_stock_distribution", plan.Spec.SourceModelLabel, &plan.Spec.ID, &plan.SelectedStock.Length, 1)
	}
	result := []WorkloadEstimate{}
	for _, v := range groups {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].EntityID < result[j].EntityID })
	return result, errors
}
func WorkloadSummary(m PhysicalModel, revision string) CalculationRun {
	now := time.Now().UTC()
	run := CalculationRun{ID: uuid.NewString(), ProjectID: m.ProjectID, RevisionID: revision, ModuleInstanceID: "material_workload", ProviderID: "engineering-workload-summary", ProviderVersion: "1", Status: "succeeded", Compatibility: "compatible", Inputs: []NamedValue{}, Outputs: []NamedValue{}, Formulas: []FormulaEvidence{}, StartedAt: now, FinishedAt: &now}
	items, errs := DerivedWorkloads(m)
	run.Errors = errs
	for _, item := range items {
		label := item.WorkType + " / " + item.SourceModelLabel
		if item.StandardLength != nil {
			label += fmt.Sprintf(" / %s %s", item.StandardLength.Value, item.StandardLength.Unit)
		}
		run.Inputs = append(run.Inputs, NamedValue{Name: item.EntityID + "/label", Value: TypedValue{Kind: "text", Text: &label}})
		q := item.Quantity
		run.Outputs = append(run.Outputs, NamedValue{Name: item.EntityID, Value: TypedValue{Kind: "decimal", Decimal: &q.Value, Unit: &q.Unit}})
	}
	if len(run.Outputs) == 0 {
		run.Status = "failed"
		run.Errors = append(run.Errors, CalculationError{Code: "WORKLOAD_MODEL_EMPTY", Message: "没有可统计的设备或连接"})
	} else if len(run.Errors) > 0 {
		run.Status = "partial"
	}
	run.InputDigest = Digest([]any{m.Equipment, m.Connections, m.Cables})
	return run
}
