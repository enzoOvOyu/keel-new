package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"net/http"
	"strconv"
)

type kickoffPublicationOption struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}
type kickoffPublicationFacilityOption struct {
	kickoffPublicationOption
	Label string `json:"label"`
	Kind  string `json:"kind"`
}
type kickoffPublicationOptions struct {
	Revision   int                                `json:"revision"`
	Devices    []kickoffPublicationOption         `json:"devices"`
	Facilities []kickoffPublicationFacilityOption `json:"facilities"`
}
type kickoffPublicationBaseline struct {
	pub  kickoff.Publication
	head string
}

func (s *Server) kickoffPublicationOptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		fail(w, 500, "KICKOFF_PUBLICATION_OPTIONS_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	var revision int
	if err = tx.QueryRow(ctx, `SELECT revision FROM kickoff_workspace WHERE project_id=$1`, s.project).Scan(&revision); err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	if expected := r.URL.Query().Get("revision"); expected != "" && expected != strconv.Itoa(revision) {
		fail(w, 409, "REVISION_CONFLICT")
		return
	}
	read, err := s.kickoffReadSnapshot(ctx, tx, revision)
	if err != nil {
		fail(w, 500, "KICKOFF_MODEL_UNAVAILABLE")
		return
	}
	// The read snapshot already binds project, model head and input revision.
	// Receipt transitions do not increment workspace revision, so check the ledger too.
	var ledgerKey string
	if err = tx.QueryRow(ctx, `SELECT COALESCE(string_agg(id::text||':'||state,',' ORDER BY id),'') FROM kickoff_publication WHERE project_id=$1`, s.project).Scan(&ledgerKey); err != nil {
		fail(w, 500, "KICKOFF_PUBLICATION_OPTIONS_UNAVAILABLE")
		return
	}
	read.optionsMu.Lock()
	defer read.optionsMu.Unlock()
	if read.options != nil && read.optionsKey == ledgerKey {
		respond(w, 200, *read.options)
		return
	}
	options, err := s.kickoffPublicationDeviceOptions(ctx, tx, read.model, read.inputs, read.detail(), revision)
	if err != nil {
		fail(w, 500, "KICKOFF_PUBLICATION_OPTIONS_UNAVAILABLE")
		return
	}
	read.optionsKey = ledgerKey
	read.options = &options
	respond(w, 200, options)
}

func (s *Server) kickoffPublicationDeviceOptions(ctx context.Context, q engineeringQuery, current engineering.PhysicalModel, inputs []kickoff.ModelInput, view kickoff.View, revision int) (kickoffPublicationOptions, error) {
	result := kickoffPublicationOptions{Revision: revision, Devices: []kickoffPublicationOption{}, Facilities: []kickoffPublicationFacilityOption{}}
	if len(view.Devices) == 0 && len(current.Facilities) == 0 {
		return result, nil
	}
	var currentHead string
	if err := q.QueryRow(ctx, `SELECT head_revision_id::text FROM eng_source WHERE id=(SELECT source_id FROM kickoff_workspace WHERE project_id=$1)`, s.project).Scan(&currentHead); err != nil {
		return result, err
	}
	ledger := []kickoffPublicationBaseline{}
	indices := map[string]int{}
	rows, err := q.Query(ctx, `SELECT id::text,revision,state,model_revision::text FROM kickoff_publication WHERE project_id=$1 AND state IN ('published','pending') ORDER BY revision`, s.project)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		v := kickoffPublicationBaseline{}
		if err = rows.Scan(&v.pub.ID, &v.pub.Revision, &v.pub.State, &v.head); err != nil {
			break
		}
		indices[v.pub.ID] = len(ledger)
		ledger = append(ledger, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = q.Query(ctx, `SELECT o.publication_id::text,o.kind,o.entity_id::text FROM kickoff_publication_object o JOIN kickoff_publication p ON p.id=o.publication_id WHERE p.project_id=$1 AND p.state IN ('published','pending')`, s.project)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var id, kind, entity string
		if err = rows.Scan(&id, &kind, &entity); err != nil {
			break
		}
		p := &ledger[indices[id]].pub
		switch kind {
		case "device":
			p.DeviceIDs = append(p.DeviceIDs, entity)
		case "port":
			p.PortIDs = append(p.PortIDs, entity)
		case "component":
			p.ComponentIDs = append(p.ComponentIDs, entity)
		case "connection":
			p.ConnectionIDs = append(p.ConnectionIDs, entity)
		case "placement":
			p.PlacementIDs = append(p.PlacementIDs, entity)
		case "location":
			p.LocationIDs = append(p.LocationIDs, entity)
		case "facility":
			p.FacilityIDs = append(p.FacilityIDs, entity)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = q.Query(ctx, `SELECT i.publication_id::text,r.table_id,r.rows FROM kickoff_publication_input i JOIN kickoff_publication p ON p.id=i.publication_id JOIN kickoff_input_revision r ON r.project_id=i.project_id AND r.table_id=i.table_id AND r.version=i.version WHERE p.project_id=$1 AND p.state='published'`, s.project)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var id string
		var input kickoff.ModelInput
		var raw []byte
		if err = rows.Scan(&id, &input.TableID, &raw); err != nil {
			break
		}
		if err = json.Unmarshal(raw, &input.Rows); err != nil {
			break
		}
		p := &ledger[indices[id]].pub
		p.ModelSnapshot = append(p.ModelSnapshot, input)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return result, err
	}
	baseline := map[string]map[string]string{}
	pending := map[string]bool{}
	models := map[string]engineering.PhysicalModel{currentHead: current}
	for _, record := range ledger {
		p := record.pub
		if p.State == "pending" {
			for _, id := range append(append([]string{}, p.DeviceIDs...), p.FacilityIDs...) {
				pending[id] = true
			}
			continue
		}
		model, ok := models[record.head]
		if !ok {
			model, _, err = loadEngineeringModel(ctx, q, record.head, s.project, false)
			if err != nil {
				return result, err
			}
			models[record.head] = model
		}
		frozen := kickoff.FrozenModel(model, p)
		frozen.Placements = append(frozen.Placements, kickoff.KnownLocations(model, p).Placements...)
		facts := kickoff.PublicationFacts(frozen, p.ModelSnapshot)
		for id, values := range kickoff.FacilityPublicationFacts(frozen, p.ModelSnapshot) {
			facts[id] = values
		}
		scopeIDs := append([]string{}, p.DeviceIDs...)
		for _, f := range frozen.Facilities {
			scopeIDs = append(scopeIDs, f.EntityID)
		}
		for _, id := range scopeIDs {
			if baseline[id] == nil {
				baseline[id] = map[string]string{}
			}
			for key, value := range facts[id] {
				baseline[id][key] = value
			}
		}
		syncKickoffPublishedFacilityFacts(baseline, frozen, facts)
		if len(frozen.Connections) > 0 {
			// Referenced peer ports are intentionally outside the frozen write
			// scope. Use the approved head to recover their device ownership,
			// and advance only published connection facts on existing baselines.
			connectionModel := engineering.EmptyPhysical(model.ProjectID)
			connectionModel.Equipment, connectionModel.Assemblies = model.Equipment, model.Assemblies
			connectionModel.Ports, connectionModel.Connections = model.Ports, frozen.Connections
			for id, values := range kickoff.PublicationFacts(connectionModel, nil) {
				if baseline[id] == nil {
					continue
				}
				for _, connection := range frozen.Connections {
					key := "connection:" + connection.EntityID
					if value, ok := values[key]; ok {
						baseline[id][key] = value
					}
				}
			}
		}
	}
	actual := kickoff.PublicationFacts(current, inputs)
	for id, values := range kickoff.FacilityPublicationFacts(current, inputs) {
		actual[id] = values
	}
	blocked := map[string]bool{}
	for _, i := range view.Issues {
		if !i.Resolved && i.Scope == "identity" {
			blocked[i.EntityID] = true
		}
	}
	option := func(id string) kickoffPublicationOption {
		v := kickoffPublicationOption{ID: id, State: "new", Reason: "尚未入库"}
		switch {
		case blocked[id]:
			v.State = "blocked"
			v.Reason = "身份待确认"
		case pending[id]:
			v.State = "pending"
			v.Reason = "正在等待入库回执"
		case baseline[id] != nil:
			if kickoff.PublicationChanged(actual[id], baseline[id]) {
				v.State = "changed"
				v.Reason = "已入库，有待发布增补"
			} else {
				v.State = "unchanged"
				v.Reason = "已入库，无待发布变化"
			}
		}
		return v
	}
	for _, d := range view.Devices {
		result.Devices = append(result.Devices, option(d.ID))
	}
	for _, f := range current.Facilities {
		label := f.Label
		if label == "" {
			label = f.Code
		}
		result.Facilities = append(result.Facilities, kickoffPublicationFacilityOption{kickoffPublicationOption: option(f.EntityID), Label: label, Kind: f.Kind})
	}
	return result, nil
}

// An independently published ancestor also advances its fact in existing
// descendant baselines. Do not add relationships or copy unpublished inputs.
func syncKickoffPublishedFacilityFacts(baseline map[string]map[string]string, frozen engineering.PhysicalModel, facts map[string]map[string]string) {
	for _, facility := range frozen.Facilities {
		key := "facility:" + facility.EntityID
		value := facts[facility.EntityID][key]
		for _, values := range baseline {
			if _, exists := values[key]; exists {
				values[key] = value
			}
		}
	}
}
