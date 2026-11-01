package cmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"strconv"
	"strings"
)

// ProjectEngineering consumes an already-reviewed immutable revision. It keeps
// planning facts in cmdb_field_source; an existing observed payload is untouched.
func (s Service) ProjectEngineering(ctx context.Context, tx pgx.Tx, event, project, publication, target, actor string, m engineering.PhysicalModel) error {
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
	projectAsset := func(id, kind, label string, attrs map[string]any) error {
		var exists, inProject bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_entity WHERE item_uid=$1),EXISTS(SELECT 1 FROM cmdb_asset_scope WHERE asset_id=$1 AND project_id=$2)`, id, project).Scan(&exists, &inProject); err != nil {
			return err
		}
		if exists && !inProject {
			return Reject("CROSS_PROJECT_ASSET_ASSOCIATION_DENIED", 403)
		}
		attrs["name"] = label
		attrs["lifecycle_state"] = "planned"
		attrs["fact_layer"] = "planned"
		item := bom.Item{UID: id, Category: kind, Hostname: label, Fields: map[string]string{}}
		for k, v := range attrs {
			if v != nil {
				item.Fields[k] = fmt.Sprint(v)
			}
		}
		if model, ok := attrs["model"].(string); ok {
			item.Model = model
		}
		if _, err := tx.Exec(ctx, `INSERT INTO bom_item(item_uid,identity_key,category,payload) VALUES($1,$2,$3,$4) ON CONFLICT(item_uid) DO NOTHING`, id, "engineering:"+project+":"+id, kind, Encode(item)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO bom_item_revision(revision_id,item_uid,payload) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, publication, id, Encode(item)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO project_asset(project_id,item_uid,revision_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, project, id, publication); err != nil {
			return err
		}
		filtered := map[string]any{}
		for key, v := range attrs {
			if v != nil {
				if _, ok := schema.Attributes(kind)[key]; ok {
					filtered[key] = v
				}
			}
		}
		version := int64(1)
		if !exists {
			if _, err := s.UpsertAsset(ctx, tx, schema, AssetInput{ID: id, Type: kind, Project: project, Attributes: filtered, Source: source, BOMItemID: &id, BOMRevision: &publication}); err != nil {
				return err
			}
		} else {
			var raw []byte
			var oldRef, oldKind string
			if err := tx.QueryRow(ctx, `SELECT payload,source_ref,asset_type,version FROM cmdb_entity WHERE item_uid=$1 FOR UPDATE`, id).Scan(&raw, &oldRef, &oldKind, &version); err != nil {
				return err
			}
			if oldKind != kind {
				return Reject("ASSET_TYPE_IDENTITY_CONFLICT", 409)
			}
			var hasObserved bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_field_source WHERE asset_id=$1 AND fact_layer<>'planned')`, id).Scan(&hasObserved); err != nil {
				return err
			}
			if strings.HasPrefix(oldRef, "engineering:") && !hasObserved {
				var err error
				version, err = s.UpsertAsset(ctx, tx, schema, AssetInput{ID: id, Type: kind, Project: project, Attributes: filtered, Source: source, BOMItemID: &id, BOMRevision: &publication})
				if err != nil {
					return err
				}
			} else {
				// Advance the read revision for new planning evidence without changing the
				// recorded/observed attributes, source, lifecycle or freshness.
				version++
				if _, err := tx.Exec(ctx, `UPDATE cmdb_entity SET version=$2,updated_at=now() WHERE item_uid=$1`, id, version); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO cmdb_revision(id,asset_id,version,schema_version,payload,before_payload,source_type,source_ref,operator,project_id) VALUES($1,$2,$3,$4,$5,$5,'bom',$6,$7,$8)`, uuid.NewString(), id, version, schema.Version, raw, ref, actor, project); err != nil {
					return err
				}
			}
		}
		// Preserve all typed planning attributes including geometry not yet exposed by
		// the catalog's generic attribute editor. They remain planned source facts.
		for field, value := range attrs {
			if value == nil {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO cmdb_field_source(id,asset_id,field_name,value,source_type,source_ref,fact_layer,revision,operator,approval_status,support,freshness,evidence_refs,project_id) VALUES($1,$2,$3,$4,'bom',$5,'planned',$6,$7,'approved','supported','unknown',$8,$9) ON CONFLICT(asset_id,field_name,source_ref,fact_layer) DO NOTHING`, uuid.NewString(), id, field, Encode(value), ref, strconv.FormatInt(version, 10), actor, Encode([]string{publication}), project); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE eng_entity SET bom_item_uid=$3 WHERE project_id=$1 AND id=$2`, project, id, id); err != nil {
			return err
		}
		if _, err := s.PutRelation(ctx, tx, schema, RelationInput{Definition: "project_association", SourceID: id, TargetID: project, Project: project, Source: Source{Type: "derived", Ref: ref, Layer: "recorded", Operator: actor}}); err != nil {
			return err
		}
		return s.AddEvent(ctx, tx, project, "engineering:"+event+":"+id, id, "engineering.published", "工程规划发布", "published", "bom", ref, actor, map[string]string{"target": target, "publication_id": publication}, []map[string]string{{"label": "开局工程", "href": "/p/" + project + "/deploy/bom"}})
	}
	relation := func(kind, from, to string) error {
		// Replace only this project's prior engineering planning relation, preserving
		// observations and manually confirmed relationships from other sources.
		sql := `UPDATE cmdb_relationship SET valid_to=now() WHERE project_id=$1 AND definition_id=$2 AND to_uid=$4 AND fact_layer='planned' AND source_ref LIKE 'engineering:%' AND from_uid<>$3 AND valid_to IS NULL`
		if kind == "located_in" {
			sql = `UPDATE cmdb_relationship SET valid_to=now() WHERE project_id=$1 AND definition_id=$2 AND from_uid=$3 AND fact_layer='planned' AND source_ref LIKE 'engineering:%' AND to_uid<>$4 AND valid_to IS NULL`
		}
		if _, err := tx.Exec(ctx, sql, project, kind, from, to); err != nil {
			return err
		}
		_, err := s.PutRelation(ctx, tx, schema, RelationInput{Definition: kind, SourceID: from, TargetID: to, Project: project, Source: source})
		return err
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
			attrs := map[string]any{"width_mm": number(v.WidthMM), "height_mm": number(v.HeightMM), "depth_mm": number(v.DepthMM), "x_mm": number(v.XMM), "y_mm": number(v.YMM), "z_mm": number(v.ZMM), "orientation_degrees": number(v.OrientationDegrees)}
			if v.CapacityU != nil {
				attrs["u_height"] = float64(*v.CapacityU)
				attrs["rack_height"] = float64(*v.CapacityU)
			}
			if v.CoordinateFrameID != nil {
				attrs["coordinate_frame_id"] = *v.CoordinateFrameID
			}
			if e = projectAsset(v.EntityID, kind, v.Label, attrs); e != nil {
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
	case engineering.PlacementTarget:
		equipment := map[string]engineering.EquipmentPlan{}
		for _, v := range m.Equipment {
			equipment[v.EntityID] = v
		}
		for _, v := range m.Placements {
			eq, ok := equipment[v.EquipmentID]
			if !ok || v.StartU == nil || v.HeightU == nil {
				return Reject("PLACEMENT_INCOMPLETE", 422)
			}
			var kind string
			if e = tx.QueryRow(ctx, `SELECT asset_type FROM cmdb_entity WHERE item_uid=$1`, eq.EntityID).Scan(&kind); e != nil {
				return e
			}
			attrs := map[string]any{"rack_uid": v.RackID, "rack_id": v.RackID, "u_start": float64(*v.StartU), "u_height": float64(*v.HeightU), "mounting_face": v.MountFace, "depth_mm": number(v.DepthMM)}
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
	rows, e := tx.Query(ctx, `SELECT DISTINCT ON(asset_id,field_name) asset_id,field_name,value,source_ref FROM cmdb_field_source WHERE asset_id=ANY($1) AND project_id=ANY($2) AND source_ref LIKE 'engineering:%' AND fact_layer='planned' AND approval_status='approved' ORDER BY asset_id,field_name,updated_at DESC,id DESC`, ids, projects)
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
		case "rack_uid", "rack_id", "u_start", "u_height", "rack_height", "mounting_face", "depth_mm", "width_mm", "height_mm", "x_mm", "y_mm", "z_mm", "orientation_degrees", "coordinate_frame_id":
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
