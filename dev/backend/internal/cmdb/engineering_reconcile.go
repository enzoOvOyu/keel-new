package cmdb

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"strconv"
	"strings"
)

// Supersede only prior publications in the same source lineage and domain.
// Observed/manual sources and immutable audit/revision history remain intact.
func (s Service) ReconcileEngineering(ctx context.Context, tx pgx.Tx, project, publication, target, actor string, refs []string, m engineering.PhysicalModel) error {
	if len(refs) == 0 {
		return nil
	}
	current := map[string]bool{}
	switch target {
	case engineering.EquipmentTarget:
		for _, v := range m.Equipment {
			current[v.EntityID] = true
		}
	case engineering.FacilityTarget:
		for _, v := range m.Facilities {
			current[v.EntityID] = true
		}
	case engineering.ConnectivityTarget:
		for _, v := range m.Ports {
			current[v.EntityID] = true
		}
		for _, v := range m.Connections {
			current[v.EntityID] = true
			path := append([]string{v.APortID}, v.PatchPortIDs...)
			path = append(path, v.ZPortID)
			for i := 2; i+1 < len(path); i += 2 {
				current[engineering.StableID(project, "connection-segment", v.EntityID+"/"+path[i]+"/"+path[i+1])] = true
			}
		}
	}
	rows, e := tx.Query(ctx, `SELECT DISTINCT asset_id FROM cmdb_field_source WHERE project_id=$1 AND source_ref=ANY($2) AND fact_layer='planned' AND superseded_at IS NULL`, project, refs)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE cmdb_relationship SET valid_to=now(),relationship_status='historical',version=version+1 WHERE project_id=$1 AND source_ref=ANY($2) AND fact_layer='planned' AND valid_to IS NULL`, project, refs); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE cmdb_field_source SET superseded_at=now() WHERE project_id=$1 AND source_ref=ANY($2) AND fact_layer='planned' AND superseded_at IS NULL`, project, refs); e != nil {
		return e
	}
	for _, id := range ids {
		var raw []byte
		var version int64
		var schema, source string
		var observed bool
		if e = tx.QueryRow(ctx, `SELECT payload,version,schema_version,source_ref,EXISTS(SELECT 1 FROM cmdb_field_source WHERE asset_id=e.item_uid AND fact_layer<>'planned') FROM cmdb_entity e WHERE item_uid=$1 FOR UPDATE`, id).Scan(&raw, &version, &schema, &source, &observed); e != nil {
			return e
		}
		if observed || !strings.HasPrefix(source, "engineering:") {
			continue
		}
		var attrs map[string]any
		if e = json.Unmarshal(raw, &attrs); e != nil {
			return e
		}
		fields, e := tx.Query(ctx, `SELECT DISTINCT old.field_name FROM cmdb_field_source old WHERE old.asset_id=$1 AND old.source_ref=ANY($2) AND old.fact_layer='planned' AND NOT EXISTS(SELECT 1 FROM cmdb_field_source active WHERE active.asset_id=old.asset_id AND active.field_name=old.field_name AND active.superseded_at IS NULL AND active.fact_layer='planned')`, id, refs)
		if e != nil {
			return e
		}
		for fields.Next() {
			var key string
			if e = fields.Scan(&key); e != nil {
				fields.Close()
				return e
			}
			if key != "name" && key != "hostname" && key != "fact_layer" && key != "lifecycle_state" {
				delete(attrs, key)
			}
		}
		e = fields.Err()
		fields.Close()
		if e != nil {
			return e
		}
		retired := false
		if target != engineering.PlacementTarget && !current[id] {
			var otherOwner bool
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_field_source f JOIN eng_publication p ON f.source_ref='engineering:'||p.id::text WHERE f.asset_id=$1 AND f.superseded_at IS NULL AND p.target IN('CMDB_PLANNED_EQUIPMENT','CMDB_PLANNED_FACILITY','CMDB_PLANNED_CONNECTIVITY'))`, id).Scan(&otherOwner); e != nil {
				return e
			}
			if !otherOwner {
				attrs["lifecycle_state"] = "retired"
				retired = true
			}
		}
		var before map[string]any
		if e = json.Unmarshal(raw, &before); e != nil {
			return e
		}
		if Hash(attrs) == Hash(before) {
			continue
		}
		next := version + 1
		ref := "engineering:" + publication
		body := Encode(attrs)
		if _, e = tx.Exec(ctx, `UPDATE cmdb_entity SET payload=$2,version=$3,lifecycle_state=COALESCE($2::jsonb->>'lifecycle_state',lifecycle_state),updated_at=now() WHERE item_uid=$1`, id, body, next); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO cmdb_revision(id,asset_id,version,schema_version,payload,before_payload,source_type,source_ref,operator,project_id) VALUES($1,$2,$3,$4,$5,$6,'bom',$7,$8,$9)`, uuid.NewString(), id, next, schema, body, raw, ref, actor, project); e != nil {
			return e
		}
		if retired {
			if e = s.AddEvent(ctx, tx, project, "engineering:withdraw:"+publication+":"+id, id, "engineering.withdrawn", "工程规划移除", "retired", "bom", ref, actor, map[string]string{"target": target, "version": strconv.FormatInt(next, 10)}, nil); e != nil {
				return e
			}
		}
	}
	return nil
}

func activeVisualAsset(alias string) string {
	return "(" + Visible(alias) + ") AND " + alias + ".lifecycle_state<>'retired'"
}
