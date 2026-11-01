package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"sort"
)

type engineeringQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Server) attachEngineering(ctx context.Context, importID, actor string, base ...string) (string, error) {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	id := uuid.NewString()
	var baseRevision, baseSource *string
	if len(base) > 0 {
		if e = tx.QueryRow(ctx, `SELECT r.id::text,s.id::text FROM eng_revision r JOIN eng_source s ON s.id=r.source_id WHERE r.id::text=$1 AND s.head_revision_id=r.id AND s.project_id=$2 FOR SHARE OF s`, base[0], s.project).Scan(&baseRevision, &baseSource); e != nil {
			return "", e
		}
	}
	var head *string
	e = tx.QueryRow(ctx, `INSERT INTO eng_source(id,project_id,import_id,base_revision_id,base_source_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(import_id) DO UPDATE SET import_id=excluded.import_id RETURNING id::text,head_revision_id::text`, id, s.project, importID, baseRevision, baseSource).Scan(&id, &head)
	if e != nil {
		return "", e
	}
	if head == nil {
		revision := uuid.NewString()
		proposal := engineering.EngineeringProposal{Summary: "原始资料已解析，等待智能理解", Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
		if _, e = tx.Exec(ctx, `INSERT INTO eng_revision(id,source_id,sequence,actor,origin,reason,total_cells,mapped_cells,proposal) SELECT $1,$2,1,$3,'agent','immutable source anchor',COALESCE((SELECT sum(jsonb_array_length(s->'source_cells')) FROM bom_import_batch b,jsonb_array_elements(b.workbook->'sheets') s WHERE b.id=$4),0),0,$5`, revision, id, actor, importID, jsonBytes(proposal)); e != nil {
			return "", e
		}
		if e = saveEngineeringIntelligence(ctx, tx, s.project, id, revision, 1, proposal, engineering.Extraction{Model: engineering.EmptyPhysical(s.project)}, nil); e != nil {
			return "", e
		}
		if _, e = tx.Exec(ctx, `UPDATE eng_source SET head_revision_id=$2 WHERE id=$1`, id, revision); e != nil {
			return "", e
		}
	}
	return id, tx.Commit(ctx)
}

func engineeringModel(ctx context.Context, q engineeringQuery, revision, project string) (engineering.PhysicalModel, []engineering.SourceEvidence, error) {
	m := engineering.EmptyPhysical(project)
	evidence := []engineering.SourceEvidence{}
	var authorized bool
	if e := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_revision r JOIN eng_source s ON s.id=r.source_id WHERE r.id::text=$1 AND s.project_id=$2)`, revision, project).Scan(&authorized); e != nil {
		return m, evidence, e
	}
	if !authorized {
		return m, evidence, errors.New("NOT_FOUND")
	}
	for _, entry := range []struct {
		table string
		dest  any
	}{{"eng_equipment", &m.Equipment}, {"eng_assembly", &m.Assemblies}, {"eng_facility", &m.Facilities}, {"eng_placement", &m.Placements}, {"eng_port", &m.Ports}, {"eng_connection", &m.Connections}, {"eng_optic_demand", &m.Optics}} {
		extra := ""
		switch entry.table {
		case "eng_facility":
			extra = `, 'width_mm',t.width_mm::text,'height_mm',t.height_mm::text,'depth_mm',t.depth_mm::text,'x_mm',t.x_mm::text,'y_mm',t.y_mm::text,'z_mm',t.z_mm::text,'orientation_degrees',t.orientation_degrees::text`
		case "eng_placement":
			extra = `, 'depth_mm',t.depth_mm::text`
		}
		var raw []byte
		sql := `SELECT COALESCE(jsonb_agg(jsonb_strip_nulls(to_jsonb(t)||jsonb_build_object('provenance_id',v.provenance_id,'maturity',v.maturity,'attributes','[]'::jsonb,'evidence_ids',COALESCE((SELECT jsonb_agg(e.id ORDER BY e.id) FROM eng_evidence e WHERE e.revision_id=t.revision_id AND e.entity_id=t.entity_id),'[]'::jsonb)` + extra + `)) ORDER BY t.entity_id),'[]'::jsonb) FROM ` + entry.table + ` t JOIN eng_entity_version v USING(revision_id,entity_id) WHERE t.revision_id=$1`
		if e := q.QueryRow(ctx, sql, revision).Scan(&raw); e != nil {
			return m, evidence, e
		}
		if e := json.Unmarshal(raw, entry.dest); e != nil {
			return m, evidence, e
		}
	}
	if e := loadNetworkRows(ctx, q, revision, &m); e != nil {
		return m, evidence, e
	}
	if e := loadEstimateRows(ctx, q, revision, &m); e != nil {
		return m, evidence, e
	}
	if e := loadPowerRows(ctx, q, revision, &m); e != nil {
		return m, evidence, e
	}
	if e := loadCableRows(ctx, q, revision, &m); e != nil {
		return m, evidence, e
	}
	if e := loadEngineeringAttributes(ctx, q, revision, &m); e != nil {
		return m, evidence, e
	}
	rows, e := q.Query(ctx, `SELECT id::text,entity_id::text,sheet,cell,field,formula,calculation_run_id::text,source_id::text FROM eng_evidence WHERE revision_id=$1 ORDER BY id`, revision)
	if e != nil {
		return m, evidence, e
	}
	defer rows.Close()
	for rows.Next() {
		var v engineering.SourceEvidence
		if e = rows.Scan(&v.ID, &v.EntityID, &v.Sheet, &v.Cell, &v.Field, &v.Formula, &v.CalculationRunID, &v.SourceID); e != nil {
			return m, evidence, e
		}
		evidence = append(evidence, v)
	}
	return m, evidence, rows.Err()
}
func targetDigest(m engineering.PhysicalModel, target string) string {
	return engineeringTargetDigest(m, target, true)
}
func engineeringTargetDigest(m engineering.PhysicalModel, target string, maturity bool) string {
	var values any
	switch target {
	case engineering.EquipmentTarget:
		values = []any{m.Equipment, m.Assemblies}
	case engineering.FacilityTarget:
		values = m.Facilities
	case engineering.PlacementTarget:
		values = []any{m.Equipment, m.Assemblies, m.Facilities, m.Placements}
	case engineering.ConnectivityTarget:
		values = []any{m.Equipment, m.Assemblies, m.Ports, m.Connections}
	case engineering.NetworkTarget:
		values = m.NetworkPlans
	case engineering.MaterialTarget:
		values = m.Materials
	case engineering.WorkloadTarget:
		values = []any{m.Workloads, m.Equipment, m.Connections, m.Cables}
	case engineering.PowerTarget:
		values = []any{m.PowerPlans, m.Equipment, m.Facilities, m.Placements}
	case engineering.CablingTarget:
		values = []any{m.Cables, m.Optics, m.Ports, m.Connections, m.Facilities, m.Placements}
	}
	raw, _ := json.Marshal(values)
	var canonical any
	_ = json.Unmarshal(raw, &canonical)
	var normalize func(any)
	normalize = func(v any) {
		switch value := v.(type) {
		case map[string]any:
			for _, key := range []string{"revision_id", "provenance_id", "evidence_ids", "calculation_run_id", "required_calculation_run_id", "quantity_calculation_run_id"} {
				delete(value, key)
			}
			if !maturity {
				delete(value, "maturity")
			}
			for _, item := range value {
				normalize(item)
			}
		case []any:
			for _, item := range value {
				normalize(item)
			}
			if len(value) > 0 {
				if first, ok := value[0].(map[string]any); ok && first["entity_id"] != nil {
					sort.Slice(value, func(i, j int) bool {
						return value[i].(map[string]any)["entity_id"].(string) < value[j].(map[string]any)["entity_id"].(string)
					})
				}
			}
		}
	}
	normalize(canonical)
	return engineering.Digest(canonical)
}
func targetHasContent(m engineering.PhysicalModel, t string) bool {
	switch t {
	case engineering.EquipmentTarget:
		return len(m.Equipment) > 0
	case engineering.FacilityTarget:
		return len(m.Facilities) > 0
	case engineering.PlacementTarget:
		return len(m.Placements) > 0
	case engineering.ConnectivityTarget:
		return len(m.Ports) > 0 || len(m.Connections) > 0
	case engineering.NetworkTarget:
		for _, p := range m.NetworkPlans {
			if engineering.NetworkHasContent(p) {
				return true
			}
		}
		return false
	case engineering.MaterialTarget:
		return len(m.Materials) > 0
	case engineering.WorkloadTarget:
		return len(m.Workloads) > 0
	case engineering.PowerTarget:
		return len(m.PowerPlans) > 0
	case engineering.CablingTarget:
		return len(m.Cables) > 0 || len(m.Optics) > 0
	}
	return false
}
func targetBlocked(issues []engineering.ValidationIssue, t string) bool {
	for _, i := range issues {
		if i.Severity == "BLOCK" && (i.PublishTarget == t || i.PublishTarget == "") {
			return true
		}
	}
	return false
}
func (s *Server) saveEngineeringDraft(ctx context.Context, source string, expected int, origin, actor, reason string, x engineering.Extraction, p engineering.EngineeringProposal, inputs ...standardInputAttachment) (string, error) {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	var version int
	var previous *string
	if e = tx.QueryRow(ctx, `SELECT revision,head_revision_id::text FROM eng_source WHERE id::text=$1 AND project_id=$2 FOR UPDATE`, source, s.project).Scan(&version, &previous); e != nil {
		return "", e
	}
	if version != expected {
		return "", errors.New("REVISION_CONFLICT")
	}
	if x.Model.ProjectID != s.project {
		return "", errors.New("PROJECT_SCOPE_MISMATCH")
	}
	definitions, e := engineeringAttributeDefinitions(ctx, tx, s.project)
	if e != nil {
		return "", e
	}
	revision := uuid.NewString()
	if origin == "human" {
		for _, issue := range engineering.ValidatePhysical(x.Model) {
			switch issue.Code {
			case "FACILITY_DIMENSION_INVALID", "COORDINATE_INVALID", "PLACEMENT_DEPTH_INVALID":
				return "", errors.New("ENGINEERING_SCHEMA_INVALID")
			}
		}
	}

	changed := map[string]bool{}
	previousPowerInput := ""
	var previousModel *engineering.PhysicalModel
	if previous != nil {
		old, oldEvidence, err := engineeringModel(ctx, tx, *previous, s.project)
		if err != nil {
			return "", err
		}
		previousPowerInput = powerInputDigest(old)
		previousModel = &old
		if origin == "agent" && len(old.NetworkPlans) > 0 {
			for i := range x.Model.NetworkPlans {
				if x.Model.NetworkPlans[i].PhysicalRevisionID == "" {
					x.Model.NetworkPlans[i].PhysicalRevisionID = old.NetworkPlans[0].PhysicalRevisionID
				}
				physical, err := networkPhysicalContext(ctx, tx, s.project, x.Model.NetworkPlans[i])
				if err != nil {
					return "", err
				}
				engineering.ResolveNetworkPhysical(&x.Model.NetworkPlans[i], physical)
			}
		}

		if origin == "human" {
			fields := map[string]map[string]bool{}
			for _, difference := range engineeringDiff(old, x.Model) {
				changed[difference.EntityID] = true
				before, after := map[string]json.RawMessage{}, map[string]json.RawMessage{}
				_ = json.Unmarshal([]byte(difference.Before), &before)
				_ = json.Unmarshal([]byte(difference.After), &after)
				fields[difference.EntityID] = map[string]bool{}
				for key, value := range after {
					if !bytes.Equal(value, before[key]) {
						fields[difference.EntityID][key] = true
					}
				}
				for key := range before {
					if _, ok := after[key]; !ok {
						fields[difference.EntityID][key] = true
					}
				}
			}
			aliases := map[string]string{"model": "source_model_label", "coordinate_frame": "coordinate_frame_id"}
			fieldChanged := func(entity, field string) bool {
				if _, _, extension := engineering.AttributeBinding(field); extension {
					return fields[entity]["attributes"]
				}
				grouped := map[string][]string{"length": {"estimated", "calculated", "confirmed", "as_built"}, "length_unit": {"estimated", "calculated", "confirmed", "as_built"}, "length_state": {"estimated", "calculated", "confirmed", "as_built"}, "standard_length": {"selected_stock"}, "standard_length_unit": {"selected_stock"}, "cable_model": {"spec"}, "connector_a": {"spec"}, "connector_z": {"spec"}, "patch_device": {"patch_port_ids"}, "patch_port": {"patch_port_ids"}, "patch_exit_port": {"patch_port_ids"}, "route_from": {"route"}, "route_to": {"route"}, "distance": {"route"}, "distance_unit": {"route"}, "power": {"equipment_profiles"}, "power_unit": {"equipment_profiles"}, "power_basis": {"equipment_profiles"}, "cooling_type": {"equipment_profiles", "racks"}, "power_capacity": {"racks"}, "power_capacity_unit": {"racks"}, "cooling_capacity": {"racks"}, "cooling_capacity_unit": {"racks"}, "required_quantity": {"required"}, "purchased_quantity": {"purchased"}, "spare_quantity": {"spares"}, "quantity_unit": {"required", "purchased", "spares", "purchase_gap"}, "work_quantity": {"quantity"}, "work_unit": {"quantity"}, "labor_unit": {"labor_hours"}, "network_role": {"device_roles"}, "network_interface": {"interface_mappings"}, "network_allocation": {"allocations"}, "network_bgp": {"bgp"}, "network_bfd": {"bfd"}, "network_qos": {"qos"}, "network_ar": {"adaptive_routing"}, "network_evpn": {"evpn_vxlan"}}
				for _, key := range grouped[field] {
					if fields[entity][key] {
						return true
					}
				}
				if alias := aliases[field]; alias != "" {
					field = alias
				}
				return fields[entity][field]
			}
			kept := []engineering.SourceEvidence{}
			for _, e := range x.Evidence {
				if !fieldChanged(e.EntityID, e.Field) {
					kept = append(kept, e)
				}
			}
			x.Evidence = kept
			rows, err := tx.Query(ctx, `SELECT id::text,entity_id::text,code,severity,module_id,field,message,target FROM eng_issue WHERE revision_id=$1 AND module_id='workbook_intelligence'`, *previous)
			if err != nil {
				return "", err
			}
			for rows.Next() {
				var issue engineering.ValidationIssue
				if err = rows.Scan(&issue.ID, &issue.EntityID, &issue.Code, &issue.Severity, &issue.ModuleID, &issue.Field, &issue.Message, &issue.PublishTarget); err != nil {
					break
				}
				if fieldChanged(issue.EntityID, issue.Field) {
					continue
				}
				if issue.Code == "REANALYSIS_REVIEWED_CONFLICT" && targetDigest(old, issue.PublishTarget) != targetDigest(x.Model, issue.PublishTarget) {
					continue
				}
				x.Issues = append(x.Issues, issue)
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			if err != nil {
				return "", err
			}
		} else {
			var priorOrigin string
			if err = tx.QueryRow(ctx, `SELECT origin FROM eng_revision WHERE id=$1`, *previous).Scan(&priorOrigin); err != nil {
				return "", err
			}
			proposed := x.Model
			for _, target := range engineering.PublishTargets {
				var protected bool
				if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_target WHERE source_id=$1 AND target=$2 AND review_id IS NOT NULL)`, source, target).Scan(&protected); err != nil {
					return "", err
				}
				if !protected && (priorOrigin != "human" || !targetHasContent(old, target)) {
					continue
				}
				if suggestionDigest(old, target) != suggestionDigest(proposed, target) {
					x.Issues = append(x.Issues, engineering.ValidationIssue{ID: uuid.NewString(), ProjectID: s.project, EntityID: source, Code: "REANALYSIS_REVIEWED_CONFLICT", Severity: "BLOCK", ModuleID: "workbook_intelligence", Field: "revision", Message: "新识别与已审核/人工编辑版本不同，已保留原值；需人工核对来源后保存修订", PublishTarget: target, Resolution: "open", EvidenceIDs: []string{}})
				}
				switch target {
				case engineering.EquipmentTarget:
					x.Model.Equipment = old.Equipment
					x.Model.Assemblies = old.Assemblies
				case engineering.FacilityTarget:
					x.Model.Facilities = old.Facilities
				case engineering.PlacementTarget:
					x.Model.Placements = old.Placements
				case engineering.ConnectivityTarget:
					x.Model.Ports = old.Ports
					x.Model.Connections = old.Connections
				case engineering.NetworkTarget:
					x.Model.NetworkPlans = old.NetworkPlans
				case engineering.MaterialTarget:
					x.Model.Materials = old.Materials
				case engineering.WorkloadTarget:
					x.Model.Workloads = old.Workloads
				case engineering.PowerTarget:
					x.Model.PowerPlans = old.PowerPlans
				case engineering.CablingTarget:
					x.Model.Cables = old.Cables
					x.Model.Optics = old.Optics
				}
			}
			x.Evidence = append(x.Evidence, oldEvidence...)
		}
	} else if origin == "human" {
		for _, d := range engineeringDiff(engineering.EmptyPhysical(s.project), x.Model) {
			changed[d.EntityID] = true
		}
	}
	if e = applyDifferenceResolutions(ctx, tx, s.project, source, revision, actor, reason, origin, &x); e != nil {
		return "", e
	}
	for i := range x.Model.NetworkPlans {
		physical, err := networkPhysicalContext(ctx, tx, s.project, x.Model.NetworkPlans[i])
		if err != nil {
			return "", err
		}
		engineering.ResolveNetworkPhysical(&x.Model.NetworkPlans[i], physical)
	}
	if previousModel != nil {
		invalidateEstimateInputs(*previousModel, &x.Model)
		invalidateCableInputs(*previousModel, &x)
	}
	if previousPowerInput != "" && previousPowerInput != powerInputDigest(x.Model) {
		for i := range x.Model.PowerPlans {
			for j := range x.Model.PowerPlans[i].Racks {
				r := &x.Model.PowerPlans[i].Racks[j]
				r.PlannedLoad = nil
				r.Remaining = nil
				r.CalculationRunID = nil
			}
		}
		kept := []engineering.SourceEvidence{}
		for _, ev := range x.Evidence {
			if ev.Field != "power_load" && ev.Field != "power_remaining" {
				kept = append(kept, ev)
			}
		}
		x.Evidence = kept
	}
	if _, e = tx.Exec(ctx, `INSERT INTO eng_revision(id,source_id,sequence,actor,origin,reason,total_cells,mapped_cells,proposal) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, revision, source, version+1, actor, origin, reason, x.TotalCells, x.MappedCells, jsonBytes(p)); e != nil {
		return "", e
	}
	for _, input := range inputs {
		if _, e = tx.Exec(ctx, `INSERT INTO eng_standard_input_source(id,revision_id,table_id,contract_version,object_key,content_sha256) VALUES($1,$2,$3,'1',$4,$5)`, input.ID, revision, input.TableID, input.ObjectKey, input.SHA256); e != nil {
			return "", e
		}
	}
	if e = verifyEngineeringResults(ctx, tx, source, s.project, x); e != nil {
		return "", e
	}
	entities := map[string]bool{}
	insertVersion := func(v *engineering.EntityVersion, kind string) error {
		if _, err := uuid.Parse(v.EntityID); err != nil || entities[v.EntityID] {
			return errors.New("INVALID_OR_DUPLICATE_ENTITY_ID")
		}
		entities[v.EntityID] = true
		v.RevisionID = revision
		v.ProvenanceID = revision
		if origin == "human" && changed[v.EntityID] {
			v.Maturity = engineering.Confirmed
			id := engineering.StableID(source, "manual-evidence", revision+v.EntityID)
			x.Evidence = append(x.Evidence, engineering.SourceEvidence{ID: id, EntityID: v.EntityID, Sheet: "[manual]", Cell: revision, Field: "reviewed_model"})
			v.EvidenceIDs = append(v.EvidenceIDs, id)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO eng_entity(id,project_id,kind,planning_key) VALUES($1::uuid,$2,$3,$1::text) ON CONFLICT(project_id,id) DO NOTHING`, v.EntityID, s.project, kind); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO eng_entity_version(revision_id,entity_id,project_id,kind,maturity,provenance_id) VALUES($1,$2,$3,$4,$5,$6)`, revision, v.EntityID, s.project, kind, v.Maturity, revision)
		if err != nil {
			return err
		}
		return saveEngineeringAttributes(ctx, tx, s.project, revision, kind, *v, definitions)
	}
	for i := range x.Model.Equipment {
		v := &x.Model.Equipment[i]
		if e = insertVersion(&v.EntityVersion, "equipment"); e != nil {
			return "", e
		}
		_, e = tx.Exec(ctx, `INSERT INTO eng_equipment VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, revision, v.EntityID, v.EquipmentType, v.ModelRef, v.SourceModelLabel, v.Quantity, v.IdentityKind, v.Label, v.Serial)
		if e != nil {
			return "", e
		}
	}
	for i := range x.Model.Assemblies {
		v := &x.Model.Assemblies[i]
		if e = insertVersion(&v.EntityVersion, "assembly"); e != nil {
			return "", e
		}
		_, e = tx.Exec(ctx, `INSERT INTO eng_assembly VALUES($1,$2,$3,$4,$5,$6,$7)`, revision, v.EntityID, v.ParentID, v.ChildID, v.RelationType, v.Quantity, v.SlotLabel)
		if e != nil {
			return "", e
		}
	}
	for i := range x.Model.Facilities {
		v := &x.Model.Facilities[i]
		if e = insertVersion(&v.EntityVersion, "facility"); e != nil {
			return "", e
		}
		_, e = tx.Exec(ctx, `INSERT INTO eng_facility VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, revision, v.EntityID, v.Kind, v.Label, v.ParentID, v.CapacityU, v.WidthMM, v.HeightMM, v.DepthMM, v.XMM, v.YMM, v.ZMM, v.OrientationDegrees, v.CoordinateFrameID)
		if e != nil {
			return "", e
		}
	}
	for i := range x.Model.Placements {
		v := &x.Model.Placements[i]
		if e = insertVersion(&v.EntityVersion, "placement"); e != nil {
			return "", e
		}
		_, e = tx.Exec(ctx, `INSERT INTO eng_placement VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, revision, v.EntityID, v.EquipmentID, v.RackID, v.StartU, v.HeightU, v.MountFace, v.DepthMM)
		if e != nil {
			return "", e
		}
	}
	if e = saveNetworkRows(ctx, tx, revision, &x.Model, insertVersion); e != nil {
		return "", e
	}
	if e = saveEstimateRows(ctx, tx, revision, &x.Model, insertVersion); e != nil {
		return "", e
	}
	if e = savePowerRows(ctx, tx, revision, &x.Model, insertVersion); e != nil {
		return "", e
	}
	if e = saveConnectivityRows(ctx, tx, revision, &x.Model, insertVersion); e != nil {
		return "", e
	}
	for _, v := range x.Evidence {
		if v.SourceID == "" {
			v.SourceID = source
		}
		if !entities[v.EntityID] {
			continue
		}
		if _, e = tx.Exec(ctx, `INSERT INTO eng_evidence(revision_id,entity_id,id,sheet,cell,field,formula,calculation_run_id,source_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`, revision, v.EntityID, v.ID, v.Sheet, v.Cell, v.Field, v.Formula, v.CalculationRunID, v.SourceID); e != nil {
			return "", e
		}
	}
	// Always recompute structural issues after merging protected values.
	issues := []engineering.ValidationIssue{}
	for _, i := range x.Issues {
		if i.ModuleID == "workbook_intelligence" {
			issues = append(issues, i)
		}
	}
	reimportIssues, reimportErr := reimportValidationIssues(ctx, tx, s.project, source, x.Model)
	if reimportErr != nil {
		return "", reimportErr
	}
	issues = append(issues, reimportIssues...)
	issues = append(issues, engineering.ValidatePhysical(x.Model)...)
	issues = append(issues, engineering.AttributeIssues(x.Model, definitions)...)
	attributeRefs, attributeErr := attributeReferenceIssues(ctx, tx, s.project, x.Model)
	if attributeErr != nil {
		return "", attributeErr
	}
	issues = append(issues, attributeRefs...)
	networkIssues, networkErr := validateNetworkDomain(ctx, tx, s.project, x.Model)
	if networkErr != nil {
		return "", networkErr
	}
	issues = append(issues, networkIssues...)
	staleWorkload, err := staleWorkloadIssues(ctx, tx, x.Model)
	if err != nil {
		return "", err
	}
	issues = append(issues, staleWorkload...)
	for _, item := range x.Model.Equipment {
		if item.IdentityKind != "observed_reference" {
			continue
		}
		var found bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_asset_scope WHERE asset_id=$1 AND project_id=$2)`, item.EntityID, s.project).Scan(&found); e != nil {
			return "", e
		}
		if !found {
			issues = append(issues, engineering.ValidationIssue{ID: uuid.NewString(), EntityID: item.EntityID, Code: "OBSERVED_REFERENCE_UNRESOLVED", Severity: "BLOCK", ModuleID: "equipment", Field: "identity_kind", Message: "已有实物引用必须关联当前项目的真实CMDB身份", PublishTarget: engineering.EquipmentTarget})
		}
	}
	for _, i := range issues {
		if _, e = tx.Exec(ctx, `INSERT INTO eng_issue VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`, revision, i.ID, i.EntityID, i.Code, i.Severity, i.ModuleID, i.Field, i.Message, i.PublishTarget); e != nil {
			return "", e
		}
	}
	for _, target := range engineering.PublishTargets {
		state := "IN_REVIEW"
		if !targetHasContent(x.Model, target) {
			state = "NEEDS_INPUT"
			if previousModel != nil && targetHasContent(*previousModel, target) {
				state = "IN_REVIEW"
			}
			var hadBase bool
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_source s JOIN eng_difference_review r ON r.source_id=s.id AND r.base_revision_id=s.base_revision_id WHERE s.id=$1 AND r.target=$2 AND r.decision IN('accept_incoming','retain_current') AND r.incoming_digest=$3)`, source, target, ownTargetDigest(x.Model, target)).Scan(&hadBase); e != nil {
				return "", e
			}
			if hadBase {
				state = "IN_REVIEW"
			}
		}
		if targetBlocked(issues, target) {
			state = "BLOCKED"
		}
		digest := targetDigest(x.Model, target)
		// Issues participate in target freshness; a new conflict cannot inherit approval.
		relevant := []string{}
		for _, i := range issues {
			if i.Severity == "BLOCK" && (i.PublishTarget == target || i.PublishTarget == "") {
				relevant = append(relevant, i.Code+"/"+i.EntityID+"/"+i.Field)
			}
		}
		sort.Strings(relevant)
		digest = engineering.Digest([]any{digest, relevant})
		_, e = tx.Exec(ctx, `INSERT INTO eng_target(source_id,target,revision_id,digest,state) VALUES($1,$2,$3,$4,$5) ON CONFLICT(source_id,target) DO UPDATE SET revision_id=excluded.revision_id,digest=excluded.digest,state=CASE WHEN eng_target.state IN('APPROVED','PUBLISHED','STALE') AND excluded.state='IN_REVIEW' THEN 'STALE' ELSE excluded.state END,review_id=NULL,publication_id=NULL WHERE eng_target.digest<>excluded.digest`, source, target, revision, digest, state)
		if e != nil {
			return "", e
		}
	}
	if e = saveEngineeringIntelligence(ctx, tx, s.project, source, revision, version+1, p, x, previous); e != nil {
		return "", e
	}
	if _, e = tx.Exec(ctx, `UPDATE eng_source SET revision=revision+1,head_revision_id=$2,summary=$3 WHERE id=$1`, source, revision, p.Summary); e != nil {
		return "", e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,'engineering.revised',$4,$5)`, uuid.NewString(), s.project, actor, source, jsonBytes(map[string]any{"revision_id": revision, "origin": origin, "reason": reason})); e != nil {
		return "", e
	}
	return revision, tx.Commit(ctx)
}
func (s *Server) engineeringWorkspace(ctx context.Context, id string) (engineering.Workspace, error) {
	v := engineering.Workspace{Modules: []engineering.ModuleDetection{}, Targets: []engineering.TargetStatus{}, Issues: []engineering.ValidationIssue{}, Questions: []string{}, Steps: []engineering.EngineeringStep{}}
	e := s.db.QueryRow(ctx, `SELECT s.id::text,s.project_id,s.import_id::text,b.filename,b.content_sha256,s.revision,s.head_revision_id::text,s.summary,s.created_at,s.base_revision_id::text,s.base_source_id::text FROM eng_source s JOIN bom_import_batch b ON b.id=s.import_id WHERE s.id::text=$1 AND s.project_id=$2`, id, s.project).Scan(&v.ID, &v.ProjectID, &v.ImportID, &v.Filename, &v.SHA256, &v.Revision, &v.HeadRevisionID, &v.Summary, &v.CreatedAt, &v.BaseRevisionID, &v.BaseSourceID)
	if e != nil {
		return v, e
	}
	if v.HeadRevisionID != nil {
		var raw []byte
		if e = s.db.QueryRow(ctx, `SELECT proposal,total_cells,mapped_cells FROM eng_revision WHERE id=$1`, *v.HeadRevisionID).Scan(&raw, &v.TotalCells, &v.MappedCells); e != nil {
			return v, e
		}
		v.RetainedCells = v.TotalCells
		var p engineering.EngineeringProposal
		if e = json.Unmarshal(raw, &p); e != nil {
			return v, e
		}
		v.Modules = p.Modules
		v.Questions = p.Questions
		if v.Modules == nil {
			v.Modules = []engineering.ModuleDetection{}
		}
		if v.Questions == nil {
			v.Questions = []string{}
		}
		rows, err := s.db.Query(ctx, `SELECT id::text,entity_id::text,code,severity,module_id,field,message,target FROM eng_issue WHERE revision_id=$1 ORDER BY severity,code,id`, *v.HeadRevisionID)
		if err != nil {
			return v, err
		}
		for rows.Next() {
			i := engineering.ValidationIssue{ProjectID: s.project, RevisionID: *v.HeadRevisionID, Resolution: "open", EvidenceIDs: []string{}}
			if e = rows.Scan(&i.ID, &i.EntityID, &i.Code, &i.Severity, &i.ModuleID, &i.Field, &i.Message, &i.PublishTarget); e != nil {
				rows.Close()
				return v, e
			}
			v.Issues = append(v.Issues, i)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return v, e
		}
	}
	rows, e := s.db.Query(ctx, `SELECT t.target,t.state,t.revision_id::text,t.digest,t.review_id::text,t.publication_id::text,p.error_code FROM eng_target t LEFT JOIN eng_publication p ON p.id=t.publication_id WHERE t.source_id=$1 ORDER BY target`, id)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		var t engineering.TargetStatus
		if e = rows.Scan(&t.Target, &t.State, &t.RevisionID, &t.Digest, &t.ReviewID, &t.PublicationID, &t.ErrorCode); e != nil {
			rows.Close()
			return v, e
		}
		v.Targets = append(v.Targets, t)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	var j engineering.EngineeringJob
	e = s.db.QueryRow(ctx, `SELECT id::text,state,error_code,created_at,updated_at FROM eng_job WHERE source_id=$1 ORDER BY created_at DESC LIMIT 1`, id).Scan(&j.ID, &j.State, &j.ErrorCode, &j.CreatedAt, &j.UpdatedAt)
	if e == pgx.ErrNoRows {
		return v, nil
	}
	if e != nil {
		return v, e
	}
	v.LatestJob = &j
	rows, e = s.db.Query(ctx, `SELECT sequence,tool,detail,state,created_at FROM eng_step WHERE job_id=$1 ORDER BY sequence DESC LIMIT 40`, j.ID)
	if e != nil {
		return v, e
	}
	defer rows.Close()
	for rows.Next() {
		var step engineering.EngineeringStep
		if e = rows.Scan(&step.Sequence, &step.Tool, &step.Detail, &step.State, &step.At); e != nil {
			return v, e
		}
		v.Steps = append(v.Steps, step)
	}
	sort.Slice(v.Steps, func(i, j int) bool { return v.Steps[i].Sequence < v.Steps[j].Sequence })
	return v, rows.Err()
}

func suggestionDigest(m engineering.PhysicalModel, target string) string {
	return engineeringTargetDigest(m, target, false)
}
