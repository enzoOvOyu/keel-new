package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"net/http"
	"strconv"
)

func (s *Server) loadKickoffPublication(ctx context.Context, q engineeringQuery, id string) (kickoff.Publication, error) {
	var p kickoff.Publication
	var revision, source string
	err := q.QueryRow(ctx, `SELECT p.id::text,p.revision,p.created_at,p.actor,p.reason,p.state,p.error_code,p.model_revision::text,r.source_id::text FROM kickoff_publication p JOIN eng_revision r ON r.id=p.model_revision WHERE p.project_id=$1 AND p.id::text=$2`, s.project, id).Scan(&p.ID, &p.Revision, &p.CreatedAt, &p.Actor, &p.Reason, &p.State, &p.ErrorCode, &revision, &source)
	if err != nil {
		return p, err
	}
	ids := []string{}
	rows, err := q.Query(ctx, `SELECT entity_id::text FROM kickoff_publication_object WHERE publication_id=$1 AND kind='device' ORDER BY entity_id`, id)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			break
		}
		ids = append(ids, key)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return p, err
	}
	model, ev, err := engineeringModel(ctx, q, revision, s.project)
	if err != nil {
		return p, err
	}
	inputs := []kickoff.ModelInput{}
	rows, err = q.Query(ctx, `SELECT r.table_id,r.rows,r.version,r.created_at,r.source FROM kickoff_publication_input p JOIN kickoff_input_revision r USING(project_id,table_id,version) WHERE p.publication_id=$1 ORDER BY r.table_id`, id)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var value kickoff.ModelInput
		var raw []byte
		if err = rows.Scan(&value.TableID, &raw, &value.Version, &value.UpdatedAt, &value.Source); err != nil {
			break
		}
		if err = json.Unmarshal(raw, &value.Rows); err != nil {
			break
		}
		value.Confirmed = true
		inputs = append(inputs, value)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return p, err
	}
	inputs = kickoff.ResolveReferences(model, inputs)
	_, selection, err := kickoff.Select(model, kickoff.Project(model, ev, source, inputs), ids)
	if err != nil {
		return p, err
	}
	p.DeviceIDs = selection.DeviceIDs
	p.PortIDs = selection.PortIDs
	p.ConnectionIDs = selection.ConnectionIDs
	p.ComponentIDs = selection.ComponentIDs
	p.PlacementIDs = selection.PlacementIDs
	p.DeviceSnapshot = selection.DeviceSnapshot
	p.RackSnapshot = selection.RackSnapshot
	p.Counts = selection.Counts
	p.ModelSnapshot = inputs
	p.ModelVersions = map[string]int{}
	for _, input := range inputs {
		p.ModelVersions[input.TableID] = input.Version
	}
	return p, nil
}
func (s *Server) kickoffPublication(w http.ResponseWriter, r *http.Request, id string) {
	p, e := s.loadKickoffPublication(r.Context(), s.db, id)
	if e == pgx.ErrNoRows {
		fail(w, 404, "KICKOFF_PUBLICATION_NOT_FOUND")
		return
	}
	if e != nil {
		fail(w, 500, "KICKOFF_PUBLICATION_UNAVAILABLE")
		return
	}
	respond(w, 200, p)
}
func (s *Server) loadKickoffPublications(ctx context.Context, q engineeringQuery) ([]kickoff.Publication, error) {
	ids := []string{}
	rows, e := q.Query(ctx, `SELECT id::text FROM kickoff_publication WHERE project_id=$1 ORDER BY revision`, s.project)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			break
		}
		ids = append(ids, id)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []kickoff.Publication{}
	for _, id := range ids {
		p, e := s.loadKickoffPublication(ctx, q, id)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, nil
}
func (s *Server) projectKickoffPublication(ctx context.Context, tx pgx.Tx, event, project string, raw []byte) error {
	var request struct {
		ID string `json:"publication_id"`
	}
	if e := json.Unmarshal(raw, &request); e != nil {
		return e
	}
	scoped := *s
	scoped.project = project
	s = &scoped
	var state, revision, importID string
	if e := tx.QueryRow(ctx, `SELECT state,model_revision::text,import_id::text FROM kickoff_publication WHERE id=$1 AND project_id=$2 FOR UPDATE`, request.ID, project).Scan(&state, &revision, &importID); e != nil {
		return e
	}
	if state == "published" {
		return nil
	}
	publication, e := s.loadKickoffPublication(ctx, tx, request.ID)
	if e != nil {
		return e
	}
	model, ev, e := engineeringModel(ctx, tx, revision, project)
	if e != nil {
		return e
	}
	subset, _, e := kickoff.Select(model, kickoff.Project(model, ev, "", publication.ModelSnapshot), publication.DeviceIDs)
	if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO bom_revision(id,import_id,project_id,digest) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO NOTHING`, publication.ID, importID, project, engineering.Digest(subset)); e != nil {
		return e
	}
	for _, target := range []string{engineering.EquipmentTarget, engineering.FacilityTarget, engineering.PlacementTarget, engineering.ConnectivityTarget} {
		if e = cmdbService(s).ProjectEngineering(ctx, tx, event+":"+target, project, publication.ID, target, publication.Actor, subset); e != nil {
			return e
		}
	}
	if e = s.projectKickoffConfiguration(ctx, tx, publication); e != nil {
		return e
	}
	// Read the projection inside the same transaction before recording success.
	for _, id := range append(append(append([]string{}, publication.DeviceIDs...), publication.PortIDs...), publication.ConnectionIDs...) {
		var exists bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_asset_scope WHERE project_id=$1 AND asset_id=$2)`, project, id).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return fmt.Errorf("CMDB_RECEIPT_MISSING")
		}
	}
	_, e = tx.Exec(ctx, `UPDATE kickoff_publication SET state='published',error_code=NULL WHERE id=$1`, publication.ID)
	return e
}

// Store engineering revision references in CMDB once per project; retain the
// full domain rows in their owning versioned engineering tables.
func (s *Server) projectKickoffConfiguration(ctx context.Context, tx pgx.Tx, p kickoff.Publication) error {
	for _, input := range p.ModelSnapshot {
		id := "kickoff:" + p.ID + ":" + input.TableID
		fields := map[string]any{"table_id": input.TableID, "input_version": input.Version, "row_count": len(input.Rows), "publication_id": p.ID, "state": "planning", "href": "/p/" + s.project + "/kickoff/publish"}
		if _, e := tx.Exec(ctx, `INSERT INTO cmdb_config_reference(id,project_id,asset_id,layer,revision,source_ref,source_type,freshness,support,fields) VALUES($1,$2,$2,'intended',$3,$4,'bom','unknown','supported',$5) ON CONFLICT DO NOTHING`, id, s.project, fmt.Sprint(input.Version), "engineering:"+p.ID, jsonBytes(fields)); e != nil {
			return e
		}
	}
	for _, device := range p.DeviceSnapshot {
		fields := map[string]any{}
		for _, row := range kickoff.Rows(p.ModelSnapshot, "management") {
			if row["device_id"] == device.ID {
				for key, target := range map[string]string{"management_ip": "management_address", "bmc_ip": "bmc_address", "management_port": "management_port", "credential_ref": "credential_reference", "method": "management_method"} {
					if row[key] != "" {
						fields[target] = row[key]
					}
				}
			}
		}
		for _, row := range kickoff.Rows(p.ModelSnapshot, "boot_interfaces") {
			if row["device_id"] == device.ID {
				for key, target := range map[string]string{"port": "boot_interface_name", "mac": "boot_mac", "boot_mode": "boot_mode", "network": "installation_network"} {
					if row[key] != "" {
						fields[target] = row[key]
					}
				}
			}
		}
		for _, row := range kickoff.Rows(p.ModelSnapshot, "power_profiles") {
			if row["device_id"] == device.ID && row["basis"] == "planned" && row["power_kw"] != "" {
				if n, e := strconv.ParseFloat(row["power_kw"], 64); e == nil && n >= 0 {
					fields["planned_power_kw"] = n
				}
			}
		}
		for key, value := range fields {
			if _, e := tx.Exec(ctx, `INSERT INTO cmdb_field_source(id,asset_id,field_name,value,source_type,source_ref,fact_layer,revision,operator,approval_status,support,freshness,evidence_refs,project_id) VALUES($1,$2,$3,$4,'bom',$5,'planned',$6,$7,'approved','supported','unknown',$8,$9) ON CONFLICT(asset_id,field_name,source_ref,fact_layer) DO NOTHING`, uuid.NewString(), device.ID, key, jsonBytes(value), "engineering:"+p.ID, p.ID, p.Actor, jsonBytes([]string{p.ID}), s.project); e != nil {
				return e
			}
		}
	}
	return nil
}

// Polling a receipt must not reload a large immutable engineering model.
func (s *Server) kickoffPublicationStatus(w http.ResponseWriter, r *http.Request, id string) {
	var state string
	var errorCode *string
	e := s.db.QueryRow(r.Context(), `SELECT state,error_code FROM kickoff_publication WHERE project_id=$1 AND id::text=$2`, s.project, id).Scan(&state, &errorCode)
	if e == pgx.ErrNoRows {
		fail(w, 404, "KICKOFF_PUBLICATION_NOT_FOUND")
		return
	}
	if e != nil {
		fail(w, 500, "KICKOFF_PUBLICATION_UNAVAILABLE")
		return
	}
	respond(w, 200, map[string]any{"id": id, "state": state, "errorCode": errorCode})
}
