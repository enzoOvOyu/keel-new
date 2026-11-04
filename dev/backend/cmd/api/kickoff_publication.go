package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) kickoffSelectPublication(ctx context.Context, q engineeringQuery, model engineering.PhysicalModel, view kickoff.View, deviceIDs, facilityIDs []string) (engineering.PhysicalModel, kickoff.Publication, error) {
	ports := map[string]engineering.PlannedPort{}
	if len(deviceIDs) > 0 && len(model.Connections) > 0 {
		ids := []string{}
		for _, connection := range model.Connections {
			ids = append(ids, connection.APortID, connection.ZPortID)
			ids = append(ids, connection.PatchPortIDs...)
		}
		rows, err := q.Query(ctx, `SELECT e.item_uid,COALESCE(NULLIF(e.payload->>'device_id',''),e.payload->>'parent_uid',''),COALESCE(e.payload->>'physical_name',e.name),NULLIF(e.payload->>'parent_port_id','')
 FROM cmdb_entity e JOIN cmdb_asset_scope s ON s.asset_id=e.item_uid AND s.project_id=$1
 WHERE e.item_uid=ANY($2::text[]) AND e.asset_type='port' AND e.lifecycle_state<>'retired'`, s.project, ids)
		if err != nil {
			return engineering.PhysicalModel{}, kickoff.Publication{}, err
		}
		for rows.Next() {
			var port engineering.PlannedPort
			if err = rows.Scan(&port.EntityID, &port.EquipmentID, &port.Name, &port.ParentPortID); err != nil {
				break
			}
			ports[port.EntityID] = port
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return engineering.PhysicalModel{}, kickoff.Publication{}, err
		}
	}
	return kickoff.SelectScope(model, view, deviceIDs, facilityIDs, ports)
}

func (s *Server) loadKickoffPublication(ctx context.Context, q engineeringQuery, id string) (kickoff.Publication, error) {
	var p kickoff.Publication
	var revision, source string
	err := q.QueryRow(ctx, `SELECT p.id::text,p.revision,p.created_at,p.actor,p.reason,p.state,p.error_code,p.model_revision::text,r.source_id::text FROM kickoff_publication p JOIN eng_revision r ON r.id=p.model_revision WHERE p.project_id=$1 AND p.id::text=$2`, s.project, id).Scan(&p.ID, &p.Revision, &p.CreatedAt, &p.Actor, &p.Reason, &p.State, &p.ErrorCode, &revision, &source)
	if err != nil {
		return p, err
	}
	objects := map[string][]string{"device": {}, "port": {}, "component": {}, "connection": {}, "placement": {}, "facility": {}, "location": {}}
	rows, err := q.Query(ctx, `SELECT kind,entity_id::text FROM kickoff_publication_object WHERE publication_id=$1 ORDER BY kind,entity_id`, id)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var kind, key string
		if err = rows.Scan(&kind, &key); err != nil {
			break
		}
		objects[kind] = append(objects[kind], key)
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
	p.DeviceIDs, p.PortIDs, p.ComponentIDs = objects["device"], objects["port"], objects["component"]
	p.ConnectionIDs, p.PlacementIDs = objects["connection"], objects["placement"]
	p.FacilityIDs, p.LocationIDs = objects["facility"], objects["location"]
	subset := kickoff.FrozenModel(model, p)
	view := kickoff.Project(model, ev, source, inputs)
	selected := map[string]bool{}
	for _, id := range p.DeviceIDs {
		selected[id] = true
	}
	located := map[string]bool{}
	for _, id := range append(append([]string{}, p.PlacementIDs...), p.LocationIDs...) {
		located[id] = true
	}
	p.DeviceSnapshot, p.RackSnapshot = []kickoff.Device{}, []kickoff.Rack{}
	facilities := map[string]bool{}
	for _, f := range subset.Facilities {
		facilities[f.EntityID] = true
	}
	for _, rack := range view.Racks {
		if facilities[rack.ID] {
			p.RackSnapshot = append(p.RackSnapshot, rack)
		}
	}
	for _, d := range view.Devices {
		if selected[d.ID] {
			if !located[d.ID] {
				d.RackID = ""
				d.StartU = nil
			}
			p.DeviceSnapshot = append(p.DeviceSnapshot, d)
		}
	}
	p.Counts = map[string]int{"devices": len(p.DeviceIDs), "components": len(p.ComponentIDs), "ports": len(p.PortIDs), "connections": len(p.ConnectionIDs), "placements": len(p.PlacementIDs), "facilities": len(p.FacilityIDs), "locations": len(p.LocationIDs)}
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
	out := []kickoff.Publication{}
	rows, err := q.Query(ctx, `SELECT p.id::text,p.revision,p.created_at,p.actor,p.reason,p.state,p.error_code,
 COALESCE((SELECT jsonb_object_agg(kind,n) FROM (SELECT kind,count(*) n FROM kickoff_publication_object WHERE publication_id=p.id GROUP BY kind) c),'{}'::jsonb),
 COALESCE((SELECT jsonb_agg(entity_id::text ORDER BY entity_id) FROM kickoff_publication_object WHERE publication_id=p.id AND kind='device'),'[]'::jsonb),
 COALESCE((SELECT jsonb_agg(entity_id::text ORDER BY entity_id) FROM kickoff_publication_object WHERE publication_id=p.id AND kind='facility'),'[]'::jsonb),
 COALESCE((SELECT jsonb_object_agg(table_id,version) FROM kickoff_publication_input WHERE publication_id=p.id),'{}'::jsonb)
 FROM kickoff_publication p WHERE p.project_id=$1 ORDER BY p.revision`, s.project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		p := kickoff.Publication{PortIDs: []string{}, ConnectionIDs: []string{}, ComponentIDs: []string{}, PlacementIDs: []string{}, FacilityIDs: []string{}, ModelSnapshot: []kickoff.ModelInput{}, RackSnapshot: []kickoff.Rack{}, DeviceSnapshot: []kickoff.Device{}, Counts: map[string]int{}}
		var counts, ids, facilities, versions []byte
		if err := rows.Scan(&p.ID, &p.Revision, &p.CreatedAt, &p.Actor, &p.Reason, &p.State, &p.ErrorCode, &counts, &ids, &facilities, &versions); err != nil {
			return nil, err
		}
		var kinds map[string]int
		if err := json.Unmarshal(counts, &kinds); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(ids, &p.DeviceIDs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(facilities, &p.FacilityIDs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(versions, &p.ModelVersions); err != nil {
			return nil, err
		}
		for singular, plural := range map[string]string{"device": "devices", "port": "ports", "component": "components", "connection": "connections", "placement": "placements", "facility": "facilities", "location": "locations"} {
			p.Counts[plural] = kinds[singular]
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Server) projectKickoffPublication(ctx context.Context, tx pgx.Tx, event, project string, raw []byte) (resultErr error) {
	started := time.Now()
	defer func() {
		slog.InfoContext(ctx, "kickoff CMDB publication", "project", project, "event", event, "duration_ms", time.Since(started).Milliseconds(), "success", resultErr == nil)
	}()
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
	model, _, e := engineeringModel(ctx, tx, revision, project)
	if e != nil {
		return e
	}
	subset := kickoff.FrozenModel(model, publication)
	if _, e = tx.Exec(ctx, `INSERT INTO bom_revision(id,import_id,project_id,digest) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO NOTHING`, publication.ID, importID, project, engineering.Digest(subset)); e != nil {
		return e
	}
	for _, target := range []string{engineering.EquipmentTarget, engineering.FacilityTarget, engineering.PlacementTarget, engineering.ConnectivityTarget} {
		if e = cmdbService(s).ProjectEngineering(ctx, tx, event+":"+target, project, publication.ID, target, publication.Actor, subset); e != nil {
			return e
		}
	}
	if len(publication.LocationIDs) > 0 {
		if e = cmdbService(s).ProjectKnownLocations(ctx, tx, event+":known-locations", project, publication.ID, publication.Actor, kickoff.KnownLocations(model, publication)); e != nil {
			return e
		}
	}
	if e = s.projectKickoffConfiguration(ctx, tx, publication); e != nil {
		return e
	}
	// Read the projection inside the same transaction before recording success.
	ids := []string{}
	for _, group := range [][]string{publication.DeviceIDs, publication.ComponentIDs, publication.PortIDs, publication.ConnectionIDs, publication.FacilityIDs} {
		ids = append(ids, group...)
	}
	var missing bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM unnest($2::text[]) AS expected(id) WHERE NOT EXISTS(SELECT 1 FROM cmdb_asset_scope s WHERE s.project_id=$1 AND s.asset_id=expected.id))`, project, ids).Scan(&missing); e != nil {
		return e
	}
	if missing {
		return fmt.Errorf("CMDB_RECEIPT_MISSING")
	}

	_, e = tx.Exec(ctx, `UPDATE kickoff_publication SET state='published',error_code=NULL WHERE id=$1`, publication.ID)
	return e
}

// Store engineering revision references in CMDB once per project; retain the
// full domain rows in their owning versioned engineering tables.
func (s *Server) projectKickoffConfiguration(ctx context.Context, tx pgx.Tx, p kickoff.Publication) error {
	refs := []map[string]any{}
	for _, input := range p.ModelSnapshot {
		refs = append(refs, map[string]any{"id": "kickoff:" + p.ID + ":" + input.TableID, "version": fmt.Sprint(input.Version), "fields": map[string]any{"table_id": input.TableID, "input_version": input.Version, "row_count": len(input.Rows), "publication_id": p.ID, "state": "planning", "href": "/p/" + s.project + "/kickoff/publish"}})
	}
	if len(refs) > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO cmdb_config_reference(id,project_id,asset_id,layer,revision,source_ref,source_type,freshness,support,fields)
 SELECT x.id,$1,$1,'intended',x.version,$2,'bom','unknown','supported',x.fields FROM jsonb_to_recordset($3::jsonb) AS x(id text,version text,fields jsonb) ON CONFLICT DO NOTHING`, s.project, "engineering:"+p.ID, jsonBytes(refs)); err != nil {
			return err
		}
	}
	// Index the publication snapshot once instead of scanning every input per device.
	devices := map[string]map[string]any{}
	for _, d := range p.DeviceSnapshot {
		devices[d.ID] = map[string]any{}
	}
	mappings := map[string]map[string]string{
		"management":      {"management_ip": "management_address", "bmc_ip": "bmc_address", "management_port": "management_port", "credential_ref": "credential_reference", "method": "management_method"},
		"boot_interfaces": {"port": "boot_interface_name", "mac": "boot_mac", "boot_mode": "boot_mode", "network": "installation_network"},
	}
	for table, mapping := range mappings {
		for _, row := range kickoff.Rows(p.ModelSnapshot, table) {
			if fields, ok := devices[row["device_id"]]; ok {
				for from, to := range mapping {
					if row[from] != "" {
						fields[to] = row[from]
					}
				}
			}
		}
	}
	for _, row := range kickoff.Rows(p.ModelSnapshot, "power_profiles") {
		if fields, ok := devices[row["device_id"]]; ok && row["basis"] == "planned" {
			if n, err := strconv.ParseFloat(row["power_kw"], 64); err == nil && n >= 0 {
				fields["planned_power_kw"] = n
			}
		}
	}
	facts := []map[string]any{}
	for id, fields := range devices {
		for field, value := range fields {
			facts = append(facts, map[string]any{"asset": id, "field": field, "value": value})
		}
	}
	racks := map[string]bool{}
	for _, rack := range p.RackSnapshot {
		racks[rack.ID] = true
	}
	for _, row := range kickoff.Rows(p.ModelSnapshot, "rack_capacity") {
		if racks[row["rack_id"]] {
			if value, err := strconv.ParseFloat(row["capacity_kw"], 64); err == nil && value > 0 {
				facts = append(facts, map[string]any{"asset": row["rack_id"], "field": "planned_power_capacity_kw", "value": value})
			}
		}
	}
	for offset := 0; offset < len(facts); offset += 1000 {
		end := offset + 1000
		if end > len(facts) {
			end = len(facts)
		}
		_, err := tx.Exec(ctx, `INSERT INTO cmdb_field_source(id,asset_id,field_name,value,source_type,source_ref,fact_layer,revision,operator,approval_status,support,freshness,evidence_refs,project_id)
 SELECT gen_random_uuid(),x.asset,x.field,x.value,'bom',$1,'planned',$2,$3,'approved','supported','unknown',$4,$5 FROM jsonb_to_recordset($6::jsonb) AS x(asset text,field text,value jsonb)
 ON CONFLICT(asset_id,field_name,source_ref,fact_layer) DO NOTHING`, "engineering:"+p.ID, p.ID, p.Actor, jsonBytes([]string{p.ID}), s.project, jsonBytes(facts[offset:end]))
		if err != nil {
			return err
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
