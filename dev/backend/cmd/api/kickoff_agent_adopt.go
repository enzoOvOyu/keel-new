package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
)

type kickoffAdoptBody struct {
	JobID            string   `json:"jobId"`
	ExpectedRevision int      `json:"expectedRevision"`
	TableIDs         []string `json:"tableIds"`
}

func (s *Server) applyKickoffAdoption(ctx context.Context, source string, body kickoffAdoptBody, actor, task, claim string) (map[string]any, error) {
	if body.JobID == "" {
		return nil, errors.New("KICKOFF_AGENT_JOB_REQUIRED")
	}
	job, state, _, request, err := s.kickoffAgentJob(ctx, source, body.JobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("KICKOFF_ANALYSIS_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	if state != "succeeded" {
		return nil, errors.New("KICKOFF_ANALYSIS_NOT_READY")
	}
	var result ai.KickoffResult
	if err = s.kickoffAgentArtifact(ctx, job, "kickoff_domain_result", &result); err != nil {
		return nil, err
	}
	candidates := map[string]ai.KickoffAgentCandidate{}
	selected := map[string]bool{}
	for _, c := range result.Candidates {
		candidates[c.TableID] = c
	}
	if len(body.TableIDs) == 0 {
		for table := range candidates {
			selected[table] = true
		}
	} else {
		for _, table := range body.TableIDs {
			if _, ok := candidates[table]; !ok {
				return nil, errors.New("KICKOFF_AGENT_TABLE_INVALID")
			}
			selected[table] = true
		}
	}
	if len(selected) == 0 {
		return nil, errors.New("KICKOFF_AGENT_NO_CANDIDATES")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = kickoffLock(ctx, tx, s.project, body.ExpectedRevision); err != nil {
		return nil, err
	}
	var raw []byte
	var analysisSource string
	var expectedSource, actualSource int
	err = tx.QueryRow(ctx, `SELECT j.proposal,j.source_id::text,j.expected_revision,e.revision FROM eng_job j JOIN eng_source e ON e.id=j.source_id WHERE j.id=$1 AND j.project_id=$2 AND j.state='succeeded' FOR UPDATE OF j,e`, job, s.project).Scan(&raw, &analysisSource, &expectedSource, &actualSource)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	if request.SourceID != source || request.Kind != kickoffAgentKind {
		return nil, errors.New("KICKOFF_ANALYSIS_NOT_FOUND")
	}
	already := map[string]bool{}
	for _, id := range request.AdoptedTableIDs {
		already[id] = true
	}
	adopted, skipped := []string{}, []string{}
	for table := range selected {
		if already[table] {
			delete(selected, table)
			skipped = append(skipped, table)
		}
	}
	if len(selected) == 0 {
		sort.Strings(skipped)
		receipt := map[string]any{"revision": body.ExpectedRevision, "adoptedTableIds": adopted, "skippedTableIds": skipped}
		if task != "" {
			if _, e := tx.Exec(ctx, `INSERT INTO eng_step(job_id,sequence,tool,state,detail) SELECT $1,COALESCE(max(sequence),0)+1,'adoption_completed','succeeded',$2 FROM eng_step WHERE job_id=$1`, task, "候选采用已提交，工程版本和来源证据已保存"); e != nil {
				return nil, e
			}
			tag, e := tx.Exec(ctx, `UPDATE eng_job SET proposal=jsonb_set(proposal,'{result}',$4::jsonb),state='succeeded',updated_at=now() WHERE id=$1 AND project_id=$2 AND claim_id=$3 AND state='running'`, task, s.project, claim, jsonBytes(receipt))
			if e != nil {
				return nil, e
			}
			if tag.RowsAffected() != 1 {
				return nil, errors.New("ENGINEERING_JOB_CLAIM_LOST")
			}
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return receipt, nil
	}
	if request.AdoptionSourceRevision > 0 {
		expectedSource = request.AdoptionSourceRevision
	}
	if actualSource != expectedSource {
		return nil, errors.New("REVISION_CONFLICT")
	}
	inputs, err := loadKickoffInputs(ctx, tx, s.project)
	if err != nil {
		return nil, err
	}
	current := map[string]kickoff.ModelInput{}
	for _, input := range inputs {
		current[input.TableID] = input
	}
	for table := range selected {
		if current[table].Version != request.BaseVersions[table] {
			return nil, errors.New("REVISION_CONFLICT")
		}
	}
	model, evidence, target, err := s.kickoffModel(ctx, tx)
	if err != nil {
		return nil, err
	}
	if target == "" {
		target, err = s.ensureKickoffDomain(ctx, tx, actor)
		if err != nil {
			return nil, err
		}
	}
	before := engineering.Digest(model)
	// Physical dependencies are applied in one explicit human adoption, in order.
	for _, table := range []string{"devices", "components", "ports", "connections"} {
		if !selected[table] {
			continue
		}
		candidate := candidates[table]
		changed := kickoffAgentMergePhysical(&model, &evidence, candidate, analysisSource)
		if changed {
			adopted = append(adopted, table)
		} else {
			skipped = append(skipped, table)
		}
		delete(selected, table)
	}
	tables := []string{}
	for table := range selected {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	for _, table := range tables {
		candidate := candidates[table]
		validRows, validationErr := kickoffAgentAdoptionRows(candidate)
		if validationErr != nil {
			skipped = append(skipped, table)
			continue
		}
		rows, changed := kickoffAgentMergeRows(table, current[table].Rows, validRows, model)
		if !changed {
			skipped = append(skipped, table)
			continue
		}
		if _, err = tx.Exec(ctx, `INSERT INTO kickoff_input_revision(project_id,table_id,version,workspace_revision,rows,source,actor) VALUES($1,$2,$3,$4,$5,$6,$7)`, s.project, table, current[table].Version+1, body.ExpectedRevision+1, jsonBytes(rows), "Agent候选人工采用 · "+job, actor); err != nil {
			return nil, err
		}
		adopted = append(adopted, table)
	}
	revision := body.ExpectedRevision
	if len(adopted) > 0 {
		// Validate all selected inputs together, so room/rack/placement references
		// resolve against the same adopted revision; no automatic publication occurs.
		inputs, err = loadKickoffInputs(ctx, tx, s.project)
		if err != nil {
			return nil, err
		}
		inputs = kickoff.ResolveReferences(model, inputs)
		if err = kickoff.ApplyInputs(&model, inputs); err != nil {
			return nil, errors.New("KICKOFF_CANDIDATE_INVALID")
		}
		if before != engineering.Digest(model) {
			var version int
			var proposalRaw []byte
			if err = tx.QueryRow(ctx, `SELECT e.revision,r.proposal FROM eng_source e JOIN eng_revision r ON r.id=e.head_revision_id WHERE e.id=$1 AND e.project_id=$2 FOR UPDATE OF e`, target, s.project).Scan(&version, &proposalRaw); err != nil {
				return nil, err
			}
			var proposal engineering.EngineeringProposal
			if err = json.Unmarshal(proposalRaw, &proposal); err != nil {
				return nil, err
			}
			proposal.Summary = "人工采用Kickoff分域候选"
			var adoptedRevision string
			adoptedRevision, err = s.saveEngineeringDraftTx(ctx, tx, target, version, "human", actor, "Kickoff candidate adoption: "+job, engineering.Extraction{Model: model, Evidence: evidence}, proposal)
			if err != nil {
				return nil, err
			}
			// Human revision handling adds manual change markers. Keep the immutable
			// source coordinates as well, rather than replacing them with those markers.
			for _, ev := range evidence {
				if ev.SourceID == "" {
					continue
				}
				if _, err = tx.Exec(ctx, `INSERT INTO eng_evidence(revision_id,entity_id,id,sheet,cell,field,formula,calculation_run_id,source_id) SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9 WHERE EXISTS(SELECT 1 FROM eng_entity_version WHERE revision_id=$1 AND entity_id=$2) ON CONFLICT DO NOTHING`, adoptedRevision, ev.EntityID, ev.ID, ev.Sheet, ev.Cell, ev.Field, ev.Formula, ev.CalculationRunID, ev.SourceID); err != nil {
					return nil, err
				}
			}
		}
		revision++
		if _, err = tx.Exec(ctx, `UPDATE kickoff_workspace SET revision=$2 WHERE project_id=$1`, s.project, revision); err != nil {
			return nil, err
		}
		if err = tx.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1 AND project_id=$2`, analysisSource, s.project).Scan(&request.AdoptionSourceRevision); err != nil {
			return nil, err
		}
		request.AdoptedTableIDs = append(request.AdoptedTableIDs, adopted...)
		if _, err = tx.Exec(ctx, `UPDATE eng_job SET proposal=$3 WHERE id=$1 AND project_id=$2`, job, s.project, jsonBytes(request)); err != nil {
			return nil, err
		}
		if err = s.kickoffAudit(ctx, tx, "kickoff.analysis.adopted", source, map[string]any{"jobId": job, "tableIds": adopted, "revision": revision}); err != nil {
			return nil, err
		}
	}
	receipt := map[string]any{"revision": revision, "adoptedTableIds": adopted, "skippedTableIds": skipped}
	if task != "" {
		if _, e := tx.Exec(ctx, `INSERT INTO eng_step(job_id,sequence,tool,state,detail) SELECT $1,COALESCE(max(sequence),0)+1,'adoption_completed','succeeded',$2 FROM eng_step WHERE job_id=$1`, task, "候选采用已提交，工程版本和来源证据已保存"); e != nil {
			return nil, e
		}
		tag, e := tx.Exec(ctx, `UPDATE eng_job SET proposal=jsonb_set(proposal,'{result}',$4::jsonb),state='succeeded',updated_at=now() WHERE id=$1 AND project_id=$2 AND claim_id=$3 AND state='running'`, task, s.project, claim, jsonBytes(receipt))
		if e != nil {
			return nil, e
		}
		if tag.RowsAffected() != 1 {
			return nil, errors.New("ENGINEERING_JOB_CLAIM_LOST")
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	sort.Strings(adopted)
	sort.Strings(skipped)
	return receipt, nil
}

// The preview and executor share eligibility; preview truncation must not
// turn invalid rows into selectable candidates.
func kickoffAgentAdoptionRows(c ai.KickoffAgentCandidate) ([]map[string]string, error) {
	rows := kickoffAgentValidRows(c)
	switch c.TableID {
	case "devices", "components", "ports", "connections":
		return rows, nil
	default:
		if err := kickoff.ValidateInput(c.TableID, rows); err != nil {
			return nil, err
		}
		eligible := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			if kickoffAgentRowKey(c.TableID, row) != "" {
				eligible = append(eligible, row)
			}
		}
		if len(rows) > 0 && len(eligible) == 0 {
			return nil, fmt.Errorf("候选缺少采用所需的关联字段：%s", strings.ReplaceAll(kickoffAgentRowKeyFields(c.TableID), "|", "、"))
		}
		return eligible, nil
	}
}

func kickoffAgentValidRows(c ai.KickoffAgentCandidate) []map[string]string {
	invalid := map[int]bool{}
	for _, issue := range c.Issues {
		invalid[issue.Row] = true
	}
	out := []map[string]string{}
	if invalid[0] {
		return out
	}
	for i, row := range c.Rows {
		if invalid[i+1] {
			continue
		}
		sensitive := false
		for key, value := range row {
			if strings.Contains(key, "credential") {
				if value != "" && !strings.HasPrefix(value, "vault://") {
					sensitive = true
				}
			} else if bom.Sensitive(value) {
				sensitive = true
			}
		}
		if !sensitive {
			out = append(out, row)
		}
	}
	return out
}
func kickoffAgentRowKeyFields(table string) string {
	return map[string]string{
		"rooms": "room_id", "racks": "rack_id", "equipment_specs": "model", "placements": "device_id", "management": "device_id",
		"network_planes": "fabric|plane", "network_roles": "device_id|fabric|plane", "network_interfaces": "device_id|port|interface",
		"network_addresses": "device_id|interface|address", "network_bgp": "device_id|interface|peer", "network_policies": "scope|profile",
		"boot_interfaces": "device_id|port", "install_profiles": "scope", "route_nodes": "code", "route_edges": "code", "rack_access": "rack_id|node",
		"cable_specs": "code|length_m", "optic_specs": "model|port_type", "cabling_rules": "code|version", "power_profiles": "device_id|basis",
		"rack_capacity": "rack_id", "power_circuits": "code|circuit", "power_connections": "device_id|psu|circuit", "power_rules": "code|version",
	}[table]
}
func kickoffAgentRowKey(table string, row map[string]string) string {
	spec := kickoffAgentRowKeyFields(table)
	if spec == "" {
		return ""
	}
	values := []string{}
	for _, field := range strings.Split(spec, "|") {
		value := strings.TrimSpace(row[field])
		if table == "rooms" && value == "" {
			value = strings.TrimSpace(row["name"])
		}
		if value == "" {
			return ""
		}
		values = append(values, value)
	}
	return strings.Join(values, "\x00")
}

// Existing nonempty fields always win, including manually confirmed inputs.
func kickoffAgentMergeRows(table string, current, candidates []map[string]string, model engineering.PhysicalModel) ([]map[string]string, bool) {
	keyOf := func(row map[string]string) string {
		normalized := kickoff.ResolveReferences(model, []kickoff.ModelInput{{TableID: table, Rows: []map[string]string{row}}})[0].Rows[0]
		// Own room/rack keys also match only an existing unique canonical reference.
		own, kind := "", ""
		if table == "rooms" {
			own, kind = "room_id", "room"
		}
		if table == "racks" {
			own, kind = "rack_id", "rack"
		}
		if own != "" {
			raw := normalized[own]
			if table == "rooms" && raw == "" {
				raw = normalized["name"]
			}
			found := ""
			ambiguous := false
			for _, f := range model.Facilities {
				if f.Kind == kind && raw != "" && (f.EntityID == raw || f.Code == raw || f.Label == raw) {
					if found != "" && found != f.EntityID {
						ambiguous = true
					}
					found = f.EntityID
				}
			}
			if found != "" && !ambiguous {
				normalized[own] = found
			}
		}
		return kickoffAgentRowKey(table, normalized)
	}
	out := make([]map[string]string, 0, len(current)+len(candidates))
	index := map[string]int{}
	for _, row := range current {
		copy := map[string]string{}
		for k, v := range row {
			copy[k] = v
		}
		out = append(out, copy)
		key := keyOf(row)
		if key != "" {
			if _, ok := index[key]; ok {
				index[key] = -1
			} else {
				index[key] = len(out) - 1
			}
		}
	}
	changed := false
	for _, row := range candidates {
		key := keyOf(row)
		if key == "" {
			continue
		}
		i, ok := index[key]
		if ok && i < 0 {
			continue
		}
		if !ok {
			copy := map[string]string{}
			for k, v := range row {
				copy[k] = v
			}
			out = append(out, copy)
			index[key] = len(out) - 1
			changed = true
			continue
		}
		for k, v := range row {
			if out[i][k] == "" && v != "" {
				out[i][k] = v
				changed = true
			}
		}
	}
	return out, changed
}

// Physical candidates reuse canonical project IDs and unique exact references.
// They merge only after the user calls adopt; never from the background worker.
func kickoffAgentMergePhysical(m *engineering.PhysicalModel, evidence *[]engineering.SourceEvidence, c ai.KickoffAgentCandidate, source string) bool {
	before := engineering.Digest(*m)
	version := func(kind, key string) engineering.EntityVersion {
		return engineering.EntityVersion{EntityID: engineering.StableID(m.ProjectID, kind, key), Maturity: engineering.Estimated, EvidenceIDs: []string{}, Attributes: []engineering.AttributeValue{}}
	}
	deviceIndex := map[string]int{}
	portIndex := map[string]int{}
	register := func(index map[string]int, key string, at int) {
		if key == "" {
			return
		}
		if previous, ok := index[key]; ok && previous != at {
			index[key] = -2
		} else {
			index[key] = at
		}
	}
	for i, d := range m.Equipment {
		if d.Serial != nil {
			register(deviceIndex, *d.Serial, i)
		}
	}
	for i, p := range m.Ports {
		register(portIndex, p.EquipmentID+"\x00"+p.Name, i)
	}
	findDevice := func(serial string) int {
		if serial == "" {
			return -1
		}
		if at, ok := deviceIndex[serial]; ok {
			return at
		}
		return -1
	}
	findPort := func(device, name string) int {
		if at, ok := portIndex[device+"\x00"+name]; ok {
			return at
		}
		return -1
	}
	knownEvidence := map[string]bool{}
	for _, e := range *evidence {
		knownEvidence[e.ID] = true
	}
	evidenceByRow := map[string][]ai.KickoffAgentEvidence{}
	rowDigests := make([]string, len(c.Rows))
	for i, row := range c.Rows {
		rowDigests[i] = engineering.Digest(row)
	}
	for _, ref := range c.Evidence {
		if ref.Row > 0 && ref.Row <= len(rowDigests) {
			key := rowDigests[ref.Row-1]
			evidenceByRow[key] = append(evidenceByRow[key], ref)
		}
	}
	addEvidence := func(entity string, row map[string]string) {
		for _, ref := range evidenceByRow[engineering.Digest(row)] {
			for field, cell := range ref.Fields {
				if row[field] == "" || cell.Cell == "" {
					continue
				}
				refs := append([]ai.KickoffCellRef{{Sheet: cell.Sheet, Cell: cell.Cell}}, cell.RelationshipEvidence...)
				for _, ref := range refs {
					id := engineering.StableID(source, "agent-field-evidence", entity+"/"+field+"/"+ref.Sheet+"/"+ref.Cell)
					if knownEvidence[id] {
						continue
					}
					knownEvidence[id] = true
					ev := engineering.SourceEvidence{ID: id, SourceID: source, EntityID: entity, Sheet: ref.Sheet, Cell: ref.Cell, Field: field, Formula: cell.Formula && ref.Sheet == cell.Sheet && ref.Cell == cell.Cell}
					if ev.Formula && cell.CalculationRunID != "" {
						run := cell.CalculationRunID
						ev.CalculationRunID = &run
					}
					*evidence = append(*evidence, ev)
				}
			}
		}
	}
	for _, row := range kickoffAgentValidRows(c) {
		switch c.TableID {
		case "devices", "components":
			serial := strings.TrimSpace(row["serial"])
			parent := -1
			if c.TableID == "components" {
				parent = findDevice(row["parent_serial"])
				if parent < 0 || (serial == "" && row["slot"] == "") {
					continue
				}
			} else if serial == "" {
				continue
			}
			at := findDevice(serial)
			if at == -2 {
				continue
			}
			if at < 0 {
				key := serial
				kind := "equipment_serial"
				if serial == "" {
					key = m.Equipment[parent].EntityID + "/" + row["slot"]
					kind = "component_slot"
				}
				id := version(kind, key)
				at = len(m.Equipment)
				for i, d := range m.Equipment {
					if d.EntityID == id.EntityID {
						at = i
						break
					}
				}
				if at == len(m.Equipment) {
					d := engineering.EquipmentPlan{EntityVersion: id, EquipmentType: row["equipment_type"], SourceModelLabel: row["model"], Label: row["label"], Quantity: 1, IdentityKind: "planned_instance"}
					if serial != "" {
						v := serial
						d.Serial = &v
					}
					if d.Label == "" {
						d.Label = serial
					}
					if d.Label == "" {
						d.Label = row["slot"]
					}
					if d.EquipmentType == "" {
						if c.TableID == "devices" {
							d.EquipmentType = "device"
						} else {
							d.EquipmentType = "component"
						}
					}
					m.Equipment = append(m.Equipment, d)
					register(deviceIndex, serial, at)
				}
			}
			d := &m.Equipment[at]
			if d.Label == "" {
				d.Label = row["label"]
			}
			if d.SourceModelLabel == "" {
				d.SourceModelLabel = row["model"]
			}
			if d.EquipmentType == "" {
				d.EquipmentType = row["equipment_type"]
			}
			addEvidence(d.EntityID, row)
			if parent >= 0 && parent != at {
				child := d.EntityID
				owner := m.Equipment[parent].EntityID
				exists := false
				for _, a := range m.Assemblies {
					if a.ChildID == child || a.ParentID == child && a.ChildID == owner {
						exists = true
						break
					}
				}
				ancestors := map[string]bool{child: true}
				changed := true
				for changed {
					changed = false
					for _, a := range m.Assemblies {
						if ancestors[a.ParentID] && !ancestors[a.ChildID] {
							ancestors[a.ChildID] = true
							changed = true
						}
					}
				}
				if ancestors[owner] {
					exists = true
				}
				if !exists {
					slot := row["slot"]
					m.Assemblies = append(m.Assemblies, engineering.DeviceAssembly{EntityVersion: version("assembly", owner+"/"+child), ParentID: owner, ChildID: child, RelationType: "contains", Quantity: 1, SlotLabel: &slot})
					addEvidence(m.Assemblies[len(m.Assemblies)-1].EntityID, row)
				}
			}
		case "ports":
			owner := findDevice(row["serial"])
			name := strings.TrimSpace(row["port"])
			if owner < 0 || name == "" {
				continue
			}
			device := m.Equipment[owner].EntityID
			at := findPort(device, name)
			if at == -2 {
				continue
			}
			if at < 0 {
				at = len(m.Ports)
				m.Ports = append(m.Ports, engineering.PlannedPort{EntityVersion: version("port", device+"\x00"+name), EquipmentID: device, Name: name})
				register(portIndex, device+"\x00"+name, at)
			}
			p := &m.Ports[at]
			if p.MAC == nil && row["mac"] != "" {
				if mac, err := net.ParseMAC(row["mac"]); err == nil && len(mac) == 6 {
					value := mac.String()
					p.MAC = &value
				}
			}
			if p.Medium == "" {
				p.Medium = row["medium"]
			}
			if p.Connector == "" {
				p.Connector = row["connector"]
			}
			if p.Role == "" {
				p.Role = row["role"]
			}
			if p.RateBPS == nil && row["rate"] != "" {
				if value, err := engineering.ParseRateBPS(row["rate"], row["rate_unit"]); err == nil {
					p.RateBPS = &value
				}
			}
			if p.ParentPortID == nil && row["parent_port"] != "" {
				parent := findPort(device, row["parent_port"])
				if parent >= 0 && parent != at {
					v := m.Ports[parent].EntityID
					p.ParentPortID = &v
					if n, err := strconv.Atoi(row["breakout_index"]); err == nil && n > 0 {
						p.BreakoutIndex = &n
					}
				}
			}
			addEvidence(p.EntityID, row)
		case "connections":
			a, b := findDevice(row["a_serial"]), findDevice(row["b_serial"])
			if a < 0 || b < 0 {
				continue
			}
			ap, bp := findPort(m.Equipment[a].EntityID, row["a_port"]), findPort(m.Equipment[b].EntityID, row["b_port"])
			if ap < 0 || bp < 0 || ap == bp {
				continue
			}
			aid, bid := m.Ports[ap].EntityID, m.Ports[bp].EntityID
			at, occupied := -1, false
			for i, conn := range m.Connections {
				exact := conn.APortID == aid && conn.ZPortID == bid || conn.APortID == bid && conn.ZPortID == aid
				if exact {
					if at >= 0 {
						occupied = true
					}
					at = i
				} else if conn.APortID == aid || conn.ZPortID == aid || conn.APortID == bid || conn.ZPortID == bid {
					occupied = true
				}
				for _, patch := range conn.PatchPortIDs {
					if patch == aid || patch == bid {
						occupied = true
					}
				}
			}
			if occupied {
				continue
			}
			if at < 0 {
				ids := []string{aid, bid}
				sort.Strings(ids)
				at = len(m.Connections)
				m.Connections = append(m.Connections, engineering.PlannedConnection{EntityVersion: version("connection", strings.Join(ids, "/")), APortID: aid, ZPortID: bid, PatchPortIDs: []string{}})
			}
			conn := &m.Connections[at]
			// Includes both optic model source cells; a model alone does not establish
			// an OpticDemand quantity or an observed installed transceiver.
			addEvidence(conn.EntityID, row)
			model, medium := strings.TrimSpace(row["cable_model"]), strings.TrimSpace(row["medium"])
			length := engineering.Quantity{Value: strings.TrimSpace(row["cable_length_m"]), Unit: "m"}
			lengthValue, lengthErr := strconv.ParseFloat(length.Value, 64)
			hasLength := length.Validate() == nil && lengthErr == nil && lengthValue > 0
			if model == "" && medium == "" && !hasLength {
				continue
			}
			cableAt, count := -1, 0
			for i, cable := range m.Cables {
				if cable.ConnectionID == conn.EntityID {
					cableAt = i
					count++
				}
			}
			// Unsegmented source rows cannot select a segment of a routed cable.
			if count > 1 || len(conn.PatchPortIDs) > 0 {
				continue
			}
			if cableAt < 0 {
				cableAt = len(m.Cables)
				m.Cables = append(m.Cables, engineering.CablePlan{EntityVersion: version("cable-plan", conn.EntityID), ConnectionID: conn.EntityID, Spec: engineering.CableSpec{ID: engineering.StableID(m.ProjectID, "cable-spec", conn.EntityID)}, Route: []engineering.RouteSegment{}})
			}
			cable := &m.Cables[cableAt]
			addEvidence(cable.EntityID, row)
			// Source estimates never change observed cable specifications or maturity.
			if cable.Maturity == engineering.AsBuilt || cable.AsBuilt != nil {
				continue
			}
			if cable.Spec.SourceModelLabel == "" {
				cable.Spec.SourceModelLabel = model
			}
			if cable.Spec.Medium == "" {
				cable.Spec.Medium = medium
			}
			if cable.Spec.ID == "" {
				cable.Spec.ID = engineering.StableID(m.ProjectID, "cable-spec", conn.EntityID)
			}
			if cable.Estimated == nil && hasLength {
				refs := []string{}
				for _, e := range *evidence {
					if e.EntityID == cable.EntityID && e.SourceID == source && e.Field == "cable_length_m" {
						refs = append(refs, e.ID)
					}
				}
				cable.Estimated = &engineering.LengthEvidence{Length: length, EvidenceIDs: refs}
			}
		}
	}
	// Attach original source evidence to each touched object, keeping existing IDs.
	ids := map[string][]string{}
	for _, e := range *evidence {
		ids[e.EntityID] = append(ids[e.EntityID], e.ID)
	}
	for _, object := range engineering.ModelObjects(m) {
		if refs := ids[object.Version.EntityID]; len(refs) > 0 {
			object.Version.EvidenceIDs = refs
		}
	}
	return before != engineering.Digest(*m)
}
