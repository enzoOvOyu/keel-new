package cmdb

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"log/slog"
	"strconv"
	"time"
)

// ProjectEngineering consumes an already-reviewed immutable revision. It keeps
// planning facts in cmdb_field_source; an existing observed payload is untouched.
func (s Service) ProjectEngineering(ctx context.Context, tx pgx.Tx, event, project, publication, target, actor string, m engineering.PhysicalModel) error {
	return s.projectEngineering(ctx, tx, event, project, publication, target, actor, m, false)
}

// ProjectKnownLocations records reviewed rack associations without claiming
// complete U occupancy. Missing geometry stays unknown and can be supplied later.
func (s Service) ProjectKnownLocations(ctx context.Context, tx pgx.Tx, event, project, publication, actor string, m engineering.PhysicalModel) error {
	return s.projectEngineering(ctx, tx, event, project, publication, engineering.PlacementTarget, actor, m, true)
}

func (s Service) projectEngineering(ctx context.Context, tx pgx.Tx, event, project, publication, target, actor string, m engineering.PhysicalModel, partialLocation bool) error {
	counter := &engineeringSQLCounter{Tx: tx}
	tx = counter
	started := time.Now()
	var assetDuration, relationDuration time.Duration
	defer func() {
		slog.InfoContext(ctx, "cmdb engineering projection", "target", target, "project", project, "publication", publication, "sql_statements", counter.Statements, "duration_ms", time.Since(started).Milliseconds(), "asset_ms", assetDuration.Milliseconds(), "relation_ms", relationDuration.Milliseconds())
	}()
	schema, e := txSchema(ctx, tx)
	if e != nil {
		return e
	}
	if e = s.ProjectReference(ctx, tx, schema, project); e != nil {
		return e
	}
	ref := "engineering:" + publication
	source := Source{Type: "bom", Ref: ref, Layer: "planned", Operator: actor, Approval: "approved", Freshness: "unknown", Support: "supported", Evidence: []string{publication}}
	number := func(v *string) any {
		if v == nil {
			return nil
		}
		n, err := strconv.ParseFloat(*v, 64)
		if err != nil {
			return nil
		}
		return n
	}
	extensionValues := map[string][]engineering.AttributeValue{}
	for _, object := range engineering.ModelObjects(&m) {
		extensionValues[object.Version.EntityID] = object.Version.Attributes
	}
	assets := []engineeringAssetWrite{}
	relations := []RelationInput{}
	projectAsset := func(id, kind, label string, attrs map[string]any) error {
		for _, value := range extensionValues[id] {
			attrs["engineering.attribute."+value.DefinitionID+"@"+strconv.Itoa(value.DefinitionVersion)] = value.Value
		}
		attrs["name"] = label
		attrs["lifecycle_state"] = "planned"
		attrs["fact_layer"] = "planned"
		assets = append(assets, engineeringAssetWrite{ID: id, Kind: kind, Label: label, Attrs: attrs})
		relations = append(relations, RelationInput{Definition: "project_association", SourceID: id, TargetID: project, Project: project, Source: Source{Type: "derived", Ref: ref, Layer: "recorded", Operator: actor}})
		return nil
	}
	relation := func(kind, from, to string) error {
		relations = append(relations, RelationInput{Definition: kind, SourceID: from, TargetID: to, Project: project, Source: source})
		return nil
	}
	switch target {
	case engineering.EquipmentTarget:
		for _, v := range m.Equipment {
			if v.IdentityKind == "group" || v.Quantity != 1 {
				return Reject("PLANNING_GROUP_REQUIRES_INSTANCE_IDENTITIES", 422)
			}
			kind := v.EquipmentType
			if _, ok := schema.Type(kind); !ok {
				kind = "device"
			}
			attrs := map[string]any{"hostname": v.Label, "model": v.SourceModelLabel}
			if v.Role != "" {
				attrs["role"] = v.Role
			}
			if v.HeightU != nil {
				attrs["height_u"] = float64(*v.HeightU)
			}
			if v.Serial != nil {
				attrs["serial"] = *v.Serial
			}
			if e = projectAsset(v.EntityID, kind, v.Label, attrs); e != nil {
				return e
			}
		}
		for _, a := range m.Assemblies {
			if e = relation("contains", a.ParentID, a.ChildID); e != nil {
				return e
			}
		}
	case engineering.FacilityTarget:
		for _, v := range m.Facilities {
			kind := map[string]string{"site": "data_center", "room": "pod", "rack": "rack"}[v.Kind]
			attrs := map[string]any{"code": v.Code, "width_mm": number(v.WidthMM), "height_mm": number(v.HeightMM), "depth_mm": number(v.DepthMM), "x_mm": number(v.XMM), "y_mm": number(v.YMM), "z_mm": number(v.ZMM), "orientation_degrees": number(v.OrientationDegrees)}
			if v.CapacityU != nil {
				attrs["u_height"] = float64(*v.CapacityU)
				attrs["rack_height"] = float64(*v.CapacityU)
			}
			if v.CoordinateFrameID != nil {
				attrs["coordinate_frame_id"] = *v.CoordinateFrameID
			}
			name := v.Label
			if name == "" {
				name = v.Code
			}
			if e = projectAsset(v.EntityID, kind, name, attrs); e != nil {
				return e
			}
		}
		for _, v := range m.Facilities {
			if v.ParentID != nil {
				if e = relation("parent", *v.ParentID, v.EntityID); e != nil {
					return e
				}
			}
		}
	case engineering.ConnectivityTarget:
		done := map[string]bool{}
		for len(done) < len(m.Ports) {
			progress := false
			for _, p := range m.Ports {
				if done[p.EntityID] {
					continue
				}
				if p.ParentPortID != nil && !done[*p.ParentPortID] {
					continue
				}
				attrs := map[string]any{"physical_name": p.Name, "device_id": p.EquipmentID, "parent_uid": p.EquipmentID, "media_type": p.Medium, "role": p.Role, "connector": p.Connector}
				if p.MAC != nil {
					attrs["mac"] = *p.MAC
				}
				if p.ParentPortID != nil {
					attrs["parent_port_id"] = *p.ParentPortID
				}
				if p.BreakoutIndex != nil {
					attrs["breakout_index"] = float64(*p.BreakoutIndex)
				}
				if p.RateBPS != nil {
					attrs["rate_bps"] = float64(*p.RateBPS)
					attrs["speed"] = strconv.FormatInt(*p.RateBPS, 10) + " bps"
				}
				if e = projectAsset(p.EntityID, "port", p.Name, attrs); e != nil {
					return e
				}
				if e = relation("contains", p.EquipmentID, p.EntityID); e != nil {
					return e
				}
				done[p.EntityID] = true
				progress = true
			}
			if !progress {
				return Reject("BREAKOUT_PARENT_UNRESOLVED", 422)
			}
		}
		cablePlans := map[string]engineering.CablePlan{}
		cableCounts := map[string]int{}
		for _, plan := range m.Cables {
			cablePlans[plan.ConnectionID] = plan
			cableCounts[plan.ConnectionID]++
		}
		for _, c := range m.Connections {
			path := append([]string{c.APortID}, c.PatchPortIDs...)
			path = append(path, c.ZPortID)
			for i := 0; i+1 < len(c.PatchPortIDs); i += 2 {
				if e = relation("patch_passthrough", c.PatchPortIDs[i], c.PatchPortIDs[i+1]); e != nil {
					return e
				}
			}
			for i := 0; i < len(path)-1; i += 2 {
				id := c.EntityID
				if i > 0 {
					id = engineering.StableID(project, "connection-segment", c.EntityID+"/"+path[i]+"/"+path[i+1])
					if _, e = tx.Exec(ctx, `INSERT INTO eng_entity(id,project_id,kind,planning_key) VALUES($1,$2,'connection_segment',$3) ON CONFLICT DO NOTHING`, id, project, id); e != nil {
						return e
					}
				}
				attrs := map[string]any{"endpoint_a_uid": path[i], "endpoint_b_uid": path[i+1], "connection_plan_id": c.EntityID, "segment_index": float64(i/2 + 1), "record_kind": "planned_connection", "termination_type": c.TerminationType, "b_rack_position_declaration": c.BRackPosition}
				// A whole-connection length is not a per-segment length. Only
				// an unambiguous direct connection carries these planned facts.
				if len(path) == 2 && cableCounts[c.EntityID] == 1 {
					plan := cablePlans[c.EntityID]
					if plan.Spec.SourceModelLabel != "" {
						attrs["model"] = plan.Spec.SourceModelLabel
					}
					if plan.Spec.Medium != "" {
						attrs["medium"] = plan.Spec.Medium
					}
					if plan.Estimated != nil && plan.Estimated.Length.Unit == "m" && plan.Estimated.Length.Validate() == nil {
						if length, err := strconv.ParseFloat(plan.Estimated.Length.Value, 64); err == nil && length > 0 {
							attrs["length_m"] = length
						}
					}
				}
				if e = projectAsset(id, "cable", "规划连接 "+strconv.Itoa(i/2+1), attrs); e != nil {
					return e
				}
				if e = relation("cable_endpoint_a", id, path[i]); e != nil {
					return e
				}
				if e = relation("cable_endpoint_b", id, path[i+1]); e != nil {
					return e
				}
			}
		}
	case engineering.PlacementTarget:
		equipment := map[string]engineering.EquipmentPlan{}
		for _, v := range m.Equipment {
			equipment[v.EntityID] = v
		}
		for _, v := range m.Placements {
			eq, ok := equipment[v.EquipmentID]
			if !ok || (!partialLocation && ((!v.UsesNonUPosition() && (v.StartU == nil || v.HeightU == nil)) || (v.UsesNonUPosition() && v.PositionCode == ""))) {
				return Reject("PLACEMENT_INCOMPLETE", 422)
			}
			var kind string
			if e = tx.QueryRow(ctx, `SELECT asset_type FROM cmdb_entity WHERE item_uid=$1`, eq.EntityID).Scan(&kind); e != nil {
				return e
			}
			attrs := map[string]any{"rack_uid": v.RackID, "rack_id": v.RackID, "mounting_face": v.MountFace, "depth_mm": number(v.DepthMM), "installation_type": v.InstallationType, "position_code": v.PositionCode}
			if partialLocation {
				attrs["placement_status"] = "needs_input"
			} else {
				attrs["placement_status"] = "validated"
			}
			if !v.UsesNonUPosition() {
				if v.StartU != nil && *v.StartU > 0 {
					attrs["u_start"] = float64(*v.StartU)
				}
				if v.HeightU != nil && *v.HeightU > 0 {
					attrs["u_height"] = float64(*v.HeightU)
				}
			}
			if e = projectAsset(eq.EntityID, kind, eq.Label, attrs); e != nil {
				return e
			}
			if e = relation("located_in", v.EquipmentID, v.RackID); e != nil {
				return e
			}
		}
	default:
		return Reject("ENGINEERING_TARGET_UNSUPPORTED", 422)
	}
	stage := time.Now()
	if e = s.writeEngineeringAssets(ctx, tx, schema, project, publication, actor, assets); e != nil {
		return e
	}
	assetDuration = time.Since(stage)
	stage = time.Now()
	if e = s.writeEngineeringRelations(ctx, tx, schema, project, relations); e != nil {
		return e
	}
	relationDuration = time.Since(stage)
	// Lifecycle entries preserve per-object IDs while sharing one bounded insert.
	for offset := 0; offset < len(assets); offset += 500 {
		end := offset + 500
		if end > len(assets) {
			end = len(assets)
		}
		events := []map[string]any{}
		for _, a := range assets[offset:end] {
			events = append(events, map[string]any{"id": "engineering:" + event + ":" + a.ID, "asset": a.ID})
		}
		_, e = tx.Exec(ctx, `INSERT INTO cmdb_lifecycle_event(id,project_id,asset_id,event_type,label,status,source_type,source_ref,operator,details,links)
 SELECT x.id,$1,x.asset,'engineering.published','工程规划发布','published','bom',$2,$3,$4,$5 FROM jsonb_to_recordset($6::jsonb) AS x(id text,asset text) ON CONFLICT(id) DO NOTHING`, project, ref, actor, Encode(map[string]string{"target": target, "publication_id": publication}), Encode([]map[string]string{{"label": "开局工程", "href": "/p/" + project + "/kickoff/publish"}}), Encode(events))
		if e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO cmdb_ingest_event(event_id,source_type,source_ref) VALUES($1,'bom',$2) ON CONFLICT DO NOTHING`, event, ref)
	return e
}

// Planned facts are an explicit read projection. This does not mutate observed
// CMDB payloads or feed planning values into discovery/operational telemetry.
func engineeringPlanningFields(ctx context.Context, tx pgx.Tx, assets []*VisualAsset) error {
	ids := []string{}
	projects := []string{}
	index := map[string]*VisualAsset{}
	for _, a := range assets {
		ids = append(ids, a.ID)
		projects = append(projects, a.Project)
		index[a.ID] = a
	}
	rows, e := tx.Query(ctx, `SELECT DISTINCT ON(asset_id,field_name) asset_id,field_name,value,source_ref FROM cmdb_field_source WHERE asset_id=ANY($1) AND project_id=ANY($2) AND source_ref LIKE 'engineering:%' AND fact_layer='planned' AND superseded_at IS NULL AND approval_status='approved' ORDER BY asset_id,field_name,updated_at DESC,id DESC`, ids, projects)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var id, field, ref string
		var raw []byte
		if e = rows.Scan(&id, &field, &raw, &ref); e != nil {
			return e
		}
		a := index[id]
		var value any
		if e = json.Unmarshal(raw, &value); e != nil {
			return e
		}
		switch field {
		case "rack_uid", "rack_id", "u_start", "u_height", "rack_height", "placement_status", "mounting_face", "depth_mm", "width_mm", "height_mm", "x_mm", "y_mm", "z_mm", "orientation_degrees", "coordinate_frame_id", "parent_port_id", "breakout_index", "rate_bps":
			if previous, ok := a.Fields[field]; ok {
				old, _ := a.Fields["recorded_geometry"].(map[string]any)
				if old == nil {
					old = map[string]any{}
				}
				old[field] = previous
				a.Fields["recorded_geometry"] = old
			}
			a.Fields[field] = value
			a.Fields["position_fact_layer"] = "planned"
			a.Fields["position_source_ref"] = ref
		}
	}
	return rows.Err()
}
