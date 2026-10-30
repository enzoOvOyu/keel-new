package cmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Scope struct {
	Projects []string
	Pairs    []string
	Actor    string
	UserID   string
}

func Visible(alias string) string {
	return `EXISTS(SELECT 1 FROM cmdb_asset_scope sc WHERE sc.asset_id=` + alias + `.item_uid AND (sc.project_id=ANY($1::text[]) OR sc.project_id||':'||sc.asset_id=ANY($2::text[])))`
}
func RelationshipVisible(alias string) string {
	return `(` + alias + `.project_id=ANY($1::text[]) OR (` + alias + `.project_id||':'||` + alias + `.from_uid=ANY($2::text[]) AND ` + alias + `.project_id||':'||` + alias + `.to_uid=ANY($2::text[])))`
}
func (a Scope) Args() []any {
	p, q := a.Projects, a.Pairs
	if p == nil {
		p = []string{}
	}
	if q == nil {
		q = []string{}
	}
	return []any{p, q}
}

type Asset struct {
	ID               string  `json:"id"`
	ProjectID        string  `json:"project_id"`
	Type             string  `json:"asset_type"`
	Name             string  `json:"name"`
	Serial           *string `json:"serial"`
	Model            *string `json:"model"`
	Address          *string `json:"address"`
	RackID           *string `json:"rack_id"`
	ParentID         *string `json:"parent_id"`
	DeliveryStage    *string `json:"delivery_stage"`
	MaintenanceState string  `json:"maintenance_state"`
	Health           string  `json:"health"`
	Freshness        string  `json:"freshness"`
	Completeness     string  `json:"completeness"`
	Source           string  `json:"source"`
	Revision         int64   `json:"revision"`
	LifecycleState   string  `json:"lifecycle_state"`
	SchemaVersion    string  `json:"schema_version"`
}

const assetSelect = `SELECT e.item_uid,(SELECT sc.project_id FROM cmdb_asset_scope sc WHERE sc.asset_id=e.item_uid AND (sc.project_id=ANY($1::text[]) OR sc.project_id||':'||sc.asset_id=ANY($2::text[])) ORDER BY sc.project_id LIMIT 1),e.asset_type,e.name,e.serial,NULLIF(e.payload->>'model',''),NULLIF(COALESCE(e.payload->>'management_address',e.payload->>'management_ip',e.payload->>'mac',e.payload->>'permanent_mac'),''),NULLIF(e.payload->>'rack_id',''),NULLIF(COALESCE(e.payload->>'component_id',e.payload->>'server_id',e.payload->>'device_id',e.payload->>'parent_uid',e.payload->>'parent_port_id'),''),NULLIF(e.payload->>'delivery_stage',''),e.maintenance_state,e.health,e.freshness,CASE WHEN e.asset_type IN('server','switch') AND (COALESCE(e.serial,'')='' OR COALESCE(e.payload->>'model',e.payload->>'model_ref','')='') THEN 'incomplete' WHEN e.asset_type='cable' AND (COALESCE(e.payload->>'endpoint_a_uid','')='' OR COALESCE(e.payload->>'endpoint_b_uid','')='') THEN 'incomplete' ELSE 'modeled' END,e.source_type,e.version,e.lifecycle_state,COALESCE(e.schema_version,'legacy') FROM cmdb_entity e `

func scanAsset(row pgx.Row) (Asset, error) {
	var a Asset
	e := row.Scan(&a.ID, &a.ProjectID, &a.Type, &a.Name, &a.Serial, &a.Model, &a.Address, &a.RackID, &a.ParentID, &a.DeliveryStage, &a.MaintenanceState, &a.Health, &a.Freshness, &a.Completeness, &a.Source, &a.Revision, &a.LifecycleState, &a.SchemaVersion)
	return a, e
}
func (s Service) Asset(ctx context.Context, scope Scope, id string) (Asset, error) {
	a, e := scanAsset(s.DB.QueryRow(ctx, assetSelect+" WHERE e.item_uid=$3 AND "+Visible("e"), append(scope.Args(), id)...))
	if e == pgx.ErrNoRows {
		return a, Reject("ASSET_NOT_FOUND", 404)
	}
	return a, e
}

type CatalogRequest struct {
	Q, Type, Status, Rack, Sort, Cursor string
	Limit                               int
}

func (s Service) Catalog(ctx context.Context, scope Scope, q CatalogRequest) (map[string]any, error) {
	if q.Limit < 1 || q.Limit > 100 {
		return nil, Reject("INVALID_LIMIT", 400)
	}
	offset, e := strconv.Atoi(q.Cursor)
	if e != nil || offset < 0 || offset > 1000000 {
		return nil, Reject("INVALID_CURSOR", 400)
	}
	order := "e.name,e.item_uid"
	switch q.Sort {
	case "name", "":
	case "-name":
		order = "e.name DESC,e.item_uid"
	case "type":
		order = "e.asset_type,e.name,e.item_uid"
	case "-type":
		order = "e.asset_type DESC,e.name,e.item_uid"
	default:
		return nil, Reject("INVALID_SORT", 400)
	}
	where := ` WHERE e.catalog_visible AND ($5='retired' OR e.lifecycle_state<>'retired') AND ` + Visible("e") + ` AND ($3='' OR e.asset_type=$3) AND ($4='' OR e.payload->>'rack_id'=$4 OR e.item_uid=$4) AND ($5='' OR e.health=$5 OR e.maintenance_state=$5 OR e.freshness=$5 OR e.lifecycle_state=$5 OR ($5='incomplete' AND e.asset_type IN('server','switch') AND (COALESCE(e.serial,'')='' OR COALESCE(e.payload->>'model',e.payload->>'model_ref','')=''))) AND ($6='' OR concat_ws(' ',e.item_uid,e.name,e.serial,e.payload->>'management_address',e.payload->>'management_ip',e.payload->>'mac',e.payload->>'permanent_mac') ILIKE '%'||$6||'%')`
	args := append(scope.Args(), q.Type, q.Rack, q.Status, q.Q)
	var total int
	if e = s.DB.QueryRow(ctx, "SELECT count(*) FROM cmdb_entity e"+where, args...).Scan(&total); e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(ctx, assetSelect+where+" ORDER BY "+order+" LIMIT $7 OFFSET $8", append(args, q.Limit, offset)...)
	if e != nil {
		return nil, e
	}
	data := []Asset{}
	for rows.Next() {
		a, e := scanAsset(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		data = append(data, a)
	}
	rows.Close()
	counts := map[string]int{"server": 0, "switch": 0, "gpu": 0, "nic": 0, "port": 0, "optic": 0, "cable": 0, "rack": 0}
	rows, e = s.DB.Query(ctx, "SELECT e.asset_type,count(*) FROM cmdb_entity e WHERE e.catalog_visible AND e.lifecycle_state<>'retired' AND "+Visible("e")+" GROUP BY e.asset_type", scope.Args()...)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var t string
		var n int
		rows.Scan(&t, &n)
		counts[t] = n
	}
	rows.Close()
	var next any
	if offset+q.Limit < total {
		next = strconv.Itoa(offset + q.Limit)
	}
	schema, e := s.Active(ctx)
	if e != nil {
		return nil, e
	}
	return map[string]any{"data": data, "counts": counts, "total": total, "page": map[string]any{"limit": q.Limit, "has_more": next != nil, "next_cursor": next}, "schema_summary": schemaSummary(schema)}, nil
}
func (s Service) RawAssets(ctx context.Context, scope Scope, kinds []string) ([]map[string]any, error) {
	schema, e := s.Active(ctx)
	if e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(ctx, `SELECT e.item_uid,e.asset_type,e.payload,e.lifecycle_state,e.health,e.freshness,e.version,(SELECT sc.project_id FROM cmdb_asset_scope sc WHERE sc.asset_id=e.item_uid AND (sc.project_id=ANY($1) OR sc.project_id||':'||sc.asset_id=ANY($2)) ORDER BY sc.project_id LIMIT 1) FROM cmdb_entity e WHERE e.lifecycle_state<>'retired' AND `+Visible("e")+` AND (cardinality($3::text[])=0 OR e.asset_type=ANY($3)) ORDER BY e.name LIMIT 10000`, append(scope.Args(), kinds)...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, kind, life, health, fresh, project string
		var version int64
		var raw []byte
		rows.Scan(&id, &kind, &raw, &life, &health, &fresh, &version, &project)
		r := map[string]any{}
		json.Unmarshal(raw, &r)
		r["id"] = id
		r["category"] = inventoryKind(schema, kind)
		r["cmdb_asset_type"] = kind
		r["project_id"] = project
		r["revision"] = version
		if r["lifecycle_status"] == nil || r["lifecycle_state"] != nil {
			r["lifecycle_status"] = life
		}
		r["health_status"] = health
		r["freshness"] = fresh
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s Service) Device(ctx context.Context, scope Scope, id string) (map[string]any, error) {
	schema, err := s.Active(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	var project, kind, device string
	e := s.DB.QueryRow(ctx, `WITH RECURSIVE owner(id,depth,path) AS(SELECT $3::text,0,ARRAY[$3::text] UNION ALL SELECT r.from_uid,o.depth+1,o.path||r.from_uid FROM owner o JOIN cmdb_relationship r ON r.to_uid=o.id AND r.definition_id IN('contains','optic_on_port') AND r.valid_to IS NULL WHERE o.depth<8 AND `+RelationshipVisible("r")+` AND NOT r.from_uid=ANY(o.path)) SELECT e.item_uid,e.asset_type,e.payload,(SELECT sc.project_id FROM cmdb_asset_scope sc WHERE sc.asset_id=e.item_uid AND (sc.project_id=ANY($1) OR sc.project_id||':'||sc.asset_id=ANY($2)) ORDER BY sc.project_id LIMIT 1) FROM owner o JOIN cmdb_entity e ON e.item_uid=o.id WHERE e.asset_type=ANY($4::text[]) AND `+Visible("e")+` ORDER BY o.depth LIMIT 1`, append(scope.Args(), id, deviceKinds(schema))...).Scan(&device, &kind, &raw, &project)
	if e == pgx.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out := map[string]any{}
	json.Unmarshal(raw, &out)
	out["id"] = device
	out["category"] = inventoryKind(schema, kind)
	out["cmdb_asset_type"] = kind
	out["project_id"] = project
	return out, nil
}
func (s Service) Tree(ctx context.Context, scope Scope) (map[string]any, error) {
	schema, e := s.Active(ctx)
	if e != nil {
		return nil, e
	}
	records, e := s.RawAssets(ctx, scope, append(deviceKinds(schema), "rack"))
	if e != nil {
		return nil, e
	}
	sites := []map[string]any{}
	siteBy := map[string]map[string]any{}
	pods := map[string]map[string]any{}
	rackBy := map[string]map[string]any{}
	devices := []map[string]any{}
	for _, r := range records {
		if r["category"] != "rack" {
			devices = append(devices, r)
			continue
		}
		site := stringValue(r["site"])
		if site == "" {
			site = "机房 unknown"
		}
		pod := stringValue(r["pod_id"])
		if pod == "" {
			pod = stringValue(r["row"])
		}
		if pod == "" {
			pod = "Pod unknown"
		}
		if siteBy[site] == nil {
			siteBy[site] = map[string]any{"id": site, "label": site, "pods": []map[string]any{}}
			sites = append(sites, siteBy[site])
		}
		pk := site + ":" + pod
		if pods[pk] == nil {
			pods[pk] = map[string]any{"id": pod, "label": pod, "racks": []map[string]any{}}
			siteBy[site]["pods"] = append(siteBy[site]["pods"].([]map[string]any), pods[pk])
		}
		id := stringValue(r["id"])
		rackBy[id] = map[string]any{"id": id, "label": r["name"], "health": r["health_status"], "devices": []map[string]any{}}
		pods[pk]["racks"] = append(pods[pk]["racks"].([]map[string]any), rackBy[id])
	}
	for _, d := range devices {
		if r := rackBy[stringValue(d["rack_id"])]; r != nil {
			r["devices"] = append(r["devices"].([]map[string]any), map[string]any{"id": d["id"], "label": defaultString(first(d, "hostname", "name", "id"), "unknown"), "type": d["category"], "health": d["health_status"]})
		}
	}
	return map[string]any{"data": sites, "device_records": devices}, nil
}

type GraphRepository interface {
	Neighbours(context.Context, Scope, []string, string, int) ([]map[string]any, error)
}
type PostgresGraph struct{ Service }

func (g PostgresGraph) Neighbours(ctx context.Context, scope Scope, roots []string, kind string, limit int) ([]map[string]any, error) {
	rows, e := g.DB.Query(ctx, `SELECT r.id,r.from_uid,r.to_uid,r.kind,r.definition_id,r.fact_layer,r.source_ref,r.source_type,r.version,r.direction,r.cardinality,r.valid_from,r.valid_to,r.relationship_status FROM cmdb_relationship r JOIN cmdb_entity a ON a.item_uid=r.from_uid JOIN cmdb_entity b ON b.item_uid=r.to_uid WHERE r.valid_to IS NULL AND (r.from_uid=ANY($3) OR r.to_uid=ANY($3)) AND ($4='' OR r.kind=$4) AND `+RelationshipVisible("r")+` AND `+Visible("a")+` AND `+Visible("b")+` ORDER BY r.kind,r.from_uid,r.to_uid LIMIT $5`, append(scope.Args(), roots, kind, limit)...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, from, to, kind, definition, fact, ref, source, direction, cardinality, status string
		var revision int64
		var at time.Time
		var until *time.Time
		rows.Scan(&id, &from, &to, &kind, &definition, &fact, &ref, &source, &revision, &direction, &cardinality, &at, &until, &status)
		out = append(out, map[string]any{"id": id, "source": from, "target": to, "kind": kind, "label": definition, "fact": fact, "source_ref": ref, "source_type": source, "revision": strconv.FormatInt(revision, 10), "direction": direction, "cardinality": cardinality, "valid_from": at, "valid_to": until, "relationship_status": status})
	}
	return out, rows.Err()
}
func (s Service) Relations(ctx context.Context, scope Scope, root string, expanded []string, kind string, limit int) (map[string]any, error) {
	if _, e := s.Asset(ctx, scope, root); e != nil {
		return nil, e
	}
	if len(expanded) > 10 || limit < 1 || limit > 120 {
		return nil, Reject("GRAPH_BOUND_EXCEEDED", 400)
	}
	roots := append([]string{root}, expanded...)
	graph := PostgresGraph{s}
	edges, e := graph.Neighbours(ctx, scope, roots, kind, limit*3+1)
	if e != nil {
		return nil, e
	}
	ids := []string{root}
	seen := map[string]bool{root: true}
	for _, r := range edges {
		for _, id := range []string{r["source"].(string), r["target"].(string)} {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	total := len(ids)
	if len(ids) > limit {
		ids = ids[:limit]
	}
	rows, e := s.DB.Query(ctx, `SELECT e.item_uid,e.name,e.asset_type,(SELECT sc.project_id FROM cmdb_asset_scope sc WHERE sc.asset_id=e.item_uid AND (sc.project_id=ANY($1) OR sc.project_id||':'||sc.asset_id=ANY($2)) ORDER BY sc.project_id LIMIT 1) FROM cmdb_entity e WHERE `+Visible("e")+` AND e.item_uid=ANY($3)`, append(scope.Args(), ids)...)
	if e != nil {
		return nil, e
	}
	nodes := []map[string]any{}
	shown := map[string]bool{}
	for rows.Next() {
		var id, name, kind, project string
		rows.Scan(&id, &name, &kind, &project)
		shown[id] = true
		nodes = append(nodes, map[string]any{"id": id, "label": name, "asset_type": kind, "project_id": project, "href": objectHref(kind, id, project)})
	}
	rows.Close()
	filtered := []map[string]any{}
	for _, r := range edges {
		if shown[r["source"].(string)] && shown[r["target"].(string)] {
			filtered = append(filtered, r)
		}
	}
	return map[string]any{"nodes": nodes, "edges": filtered, "total_nodes": total, "truncated": total > limit || len(edges) > limit*3}, nil
}
func (s Service) Provenance(ctx context.Context, scope Scope, id string) ([]map[string]any, error) {
	asset, e := s.Asset(ctx, scope, id)
	if e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(ctx, `SELECT id,field_name,value,source_type,source_ref,source_location,fact_layer,observed_at,revision,operator,approval_status,support,freshness,evidence_refs,updated_at,confidence FROM cmdb_field_source WHERE asset_id=$3 AND (project_id=ANY($1) OR project_id||':'||asset_id=ANY($2)) ORDER BY field_name,updated_at DESC`, append(scope.Args(), id)...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var sid, field, source, ref, layer, rev, operator, status, support, fresh string
		var value, location, refs []byte
		var observed *time.Time
		var updated time.Time
		var confidence *float64
		rows.Scan(&sid, &field, &value, &source, &ref, &location, &layer, &observed, &rev, &operator, &status, &support, &fresh, &refs, &updated, &confidence)
		loc := map[string]any{}
		json.Unmarshal(location, &loc)
		var href any
		if source == "bom" && !strings.HasPrefix(ref, "fixture:") {
			href = "/p/" + asset.ProjectID + "/deploy/bom/" + ref
		}
		if resource, ok := loc["resource"].(string); ok {
			if record, ok := loc["record_id"].(string); ok {
				href = "/cmdb/configuration?asset=" + id + "&resource=" + url.QueryEscape(resource) + "&evidence=" + url.QueryEscape(record)
			}
		}
		out = append(out, map[string]any{"id": sid, "asset_id": id, "field": field, "value_json": string(value), "source": source, "source_type": source, "source_ref": ref, "source_location": loc, "revision": rev, "observed_at": observed, "updated_at": updated, "operator": operator, "confidence": confidence, "freshness": fresh, "support": support, "evidence_refs": json.RawMessage(refs), "import_id": loc["import_id"], "sheet": loc["sheet"], "cell": loc["cell"], "status": status, "fact_layer": layer, "href": href})
	}
	return out, rows.Err()
}
func assetProject(ctx context.Context, s Service, scope Scope, id string) string {
	a, e := s.Asset(ctx, scope, id)
	if e != nil {
		return ""
	}
	return a.ProjectID
}
func (s Service) Configuration(ctx context.Context, scope Scope, id string) (map[string]any, error) {
	if _, e := s.Asset(ctx, scope, id); e != nil {
		return nil, e
	}
	owner, e := s.Device(ctx, scope, id)
	if e != nil {
		return nil, e
	}
	assetID := id
	if owner != nil {
		assetID = stringValue(owner["id"])
	}
	rows, e := s.DB.Query(ctx, `SELECT id,layer,revision,source_ref,source_type,observed_at,freshness,support,fields FROM cmdb_config_reference WHERE asset_id=$3 AND (project_id=ANY($1) OR project_id||':'||asset_id=ANY($2)) ORDER BY layer,revision`, append(scope.Args(), assetID)...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	data := []map[string]any{}
	intended := "unknown"
	for rows.Next() {
		var id, layer, rev, ref, source, fresh, support string
		var at *time.Time
		var fields []byte
		rows.Scan(&id, &layer, &rev, &ref, &source, &at, &fresh, &support, &fields)
		if layer == "intended" {
			intended = "available"
		}
		data = append(data, map[string]any{"id": id, "layer": layer, "revision": rev, "resource": strings.SplitN(ref, ":", 2)[0], "source": source, "source_ref": ref, "observed_at": at, "freshness": fresh, "support": support, "fields": json.RawMessage(fields)})
	}
	return map[string]any{"data": data, "intended_availability": intended, "intended_reason": map[bool]string{true: "已加载独立渲染版本", false: "未提供独立 intended 证据，不能以批准基线代替"}[intended == "available"]}, rows.Err()
}
func (s Service) Lifecycle(ctx context.Context, scope Scope, id string) (map[string]any, error) {
	if _, e := s.Asset(ctx, scope, id); e != nil {
		return nil, e
	}
	owner, e := s.Device(ctx, scope, id)
	if e != nil {
		return nil, e
	}
	ownerID := id
	if owner != nil {
		ownerID = stringValue(owner["id"])
	}
	rows, e := s.DB.Query(ctx, `SELECT id,event_type,label,status,stage_id,source_type,source_ref,occurred_at,links FROM cmdb_lifecycle_event WHERE asset_id=ANY($3::text[]) AND (project_id=ANY($1) OR project_id||':'||asset_id=ANY($2)) ORDER BY occurred_at DESC,id LIMIT 500`, append(scope.Args(), []string{id, ownerID})...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var eid, kind, label, status, source, ref string
		var stage *string
		var at time.Time
		var links []byte
		rows.Scan(&eid, &kind, &label, &status, &stage, &source, &ref, &at, &links)
		data = append(data, map[string]any{"id": eid, "asset_id": id, "event_type": kind, "label": label, "status": status, "stage_id": stage, "source": source, "source_ref": ref, "occurred_at": at, "links": json.RawMessage(links), "historical": false})
	}
	stages := []map[string]any{}
	rows.Close()
	rows, e = s.DB.Query(ctx, "SELECT summary FROM cmdb_external_reference WHERE resource='delivery_stage_records' AND $3=ANY(asset_ids) AND (project_id=ANY($1) OR project_id||':'||$3=ANY($2))", append(scope.Args(), ownerID)...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		rows.Scan(&raw)
		var stage map[string]any
		json.Unmarshal(raw, &stage)
		stages = append(stages, stage)
	}
	return map[string]any{"data": data, "stages": stages}, rows.Err()
}
func (s Service) History(ctx context.Context, scope Scope, id string) ([]map[string]any, error) {
	if _, e := s.Asset(ctx, scope, id); e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(ctx, "SELECT id,version,schema_version,payload,before_payload,source_type,source_ref,operator,created_at FROM cmdb_revision WHERE asset_id=$3 AND (project_id=ANY($1) OR project_id||':'||asset_id=ANY($2)) ORDER BY version DESC LIMIT 200", append(scope.Args(), id)...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, schema, source, ref, actor string
		var version int64
		var payload, before []byte
		var at time.Time
		rows.Scan(&id, &version, &schema, &payload, &before, &source, &ref, &actor, &at)
		out = append(out, map[string]any{"id": id, "version": version, "schema_version": schema, "attributes": json.RawMessage(payload), "before": json.RawMessage(before), "source": source, "source_ref": ref, "operator": actor, "created_at": at})
	}
	return out, rows.Err()
}
func (s Service) Diff(ctx context.Context, scope Scope, id string, from, to int64) (map[string]any, error) {
	if _, e := s.Asset(ctx, scope, id); e != nil {
		return nil, e
	}
	var beforeRaw, afterRaw []byte
	if e := s.DB.QueryRow(ctx, "SELECT payload FROM cmdb_revision WHERE asset_id=$3 AND version=$4 AND (project_id=ANY($1) OR project_id||':'||asset_id=ANY($2))", append(scope.Args(), id, from)...).Scan(&beforeRaw); e != nil {
		return nil, Reject("REVISION_NOT_FOUND", 404)
	}
	if e := s.DB.QueryRow(ctx, "SELECT payload FROM cmdb_revision WHERE asset_id=$3 AND version=$4 AND (project_id=ANY($1) OR project_id||':'||asset_id=ANY($2))", append(scope.Args(), id, to)...).Scan(&afterRaw); e != nil {
		return nil, Reject("REVISION_NOT_FOUND", 404)
	}
	var before, after map[string]any
	json.Unmarshal(beforeRaw, &before)
	json.Unmarshal(afterRaw, &after)
	keys := map[string]bool{}
	for k := range before {
		keys[k] = true
	}
	for k := range after {
		keys[k] = true
	}
	changes := []map[string]any{}
	for key := range keys {
		if Hash(before[key]) != Hash(after[key]) {
			changes = append(changes, map[string]any{"field": key, "before": before[key], "after": after[key]})
		}
	}
	return map[string]any{"asset_id": id, "from": from, "to": to, "changes": changes}, nil
}
func (s Service) Quality(ctx context.Context, scope Scope, id string) ([]map[string]any, error) {
	args := scope.Args()
	q := `SELECT e.item_uid,e.asset_type,e.serial,e.payload,e.freshness FROM cmdb_entity e WHERE ` + Visible("e")
	if id != "" {
		q += " AND e.item_uid=$3"
		args = append(args, id)
	}
	rows, e := s.DB.Query(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	add := func(asset, field, code, summary, source, href string) {
		out = append(out, map[string]any{"id": code + ":" + asset + ":" + field, "asset_id": asset, "field": field, "code": code, "summary": summary, "source": source, "severity": "warning", "href": href, "status": "open"})
	}
	for rows.Next() {
		var id, kind, fresh string
		var sn *string
		var raw []byte
		rows.Scan(&id, &kind, &sn, &raw, &fresh)
		var r map[string]any
		json.Unmarshal(raw, &r)
		if (kind == "server" || kind == "switch") && sn == nil {
			add(id, "serial", "MISSING_SN", "设备 SN 缺失", "cmdb", "/cmdb/configuration?asset="+id)
		}
		if (kind == "server" || kind == "switch") && stringValue(r["model"]) == "" {
			add(id, "model", "MODEL_UNBOUND", "设备型号未绑定", "bom", "/cmdb/configuration?asset="+id)
		}
		if kind == "cable" && (r["endpoint_a_uid"] == nil || r["endpoint_b_uid"] == nil) {
			add(id, "endpoints", "CABLE_ENDPOINT_MISSING", "线缆双端信息不完整", "bom", "/cmdb/relations?asset="+id)
		}
		if fresh == "stale" {
			add(id, "freshness", "STALE_COLLECTION", "观测数据陈旧", "collection", "/cmdb/history?asset="+id)
		}
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	extra, e := s.extraQuality(ctx, scope, id)
	if e != nil {
		return nil, e
	}
	return append(out, extra...), nil
}
func (s Service) Manual(ctx context.Context, scope Scope, id string, expected int64, notes string) (map[string]any, error) {
	a, e := s.Asset(ctx, scope, id)
	if e != nil {
		return nil, e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	schema, e := txSchema(ctx, tx)
	if e != nil {
		return nil, e
	}
	var raw []byte
	tx.QueryRow(ctx, "SELECT payload FROM cmdb_entity WHERE item_uid=$1", id).Scan(&raw)
	var attrs map[string]any
	json.Unmarshal(raw, &attrs)
	patch := map[string]any{"operator_notes": notes}
	if issues := schema.Validate(a.Type, patch, "manual", true); len(issues) > 0 {
		return nil, &Error{403, "MANUAL_ATTRIBUTE_DENIED", issues}
	}
	attrs["operator_notes"] = notes
	version, e := s.UpsertAsset(ctx, tx, schema, AssetInput{ID: id, Type: a.Type, Project: a.ProjectID, Attributes: attrs, ExpectedVersion: &expected, Source: Source{Type: "manual", Ref: "note:" + fmt.Sprint(time.Now().UnixNano()), Layer: "manual", Operator: scope.Actor, Approval: "authorized_manual", Support: "supported"}})
	if e != nil {
		return nil, e
	}
	if e = s.Audit(ctx, tx, a.ProjectID, scope.Actor, "manual.updated", id, patch); e != nil {
		return nil, e
	}
	if e = s.AddEvent(ctx, tx, a.ProjectID, "note:"+id+":"+strconv.FormatInt(version, 10), id, "manual.updated", "授权人工备注更新", "published", "manual", id, scope.Actor, patch, nil); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return map[string]any{"asset_id": id, "operator_notes": notes, "revision": version}, nil
}
func (s Service) SavedViews(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, e := s.DB.Query(ctx, "SELECT v.id,v.name,v.query,v.created_at,u.username FROM cmdb_saved_view v JOIN app_user u ON u.id=v.user_id WHERE v.user_id=$1 ORDER BY v.created_at DESC", userID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, q, actor string
		var at time.Time
		rows.Scan(&id, &name, &q, &at, &actor)
		parsed, _ := url.Parse(q)
		project := ""
		if parsed != nil {
			project = parsed.Query().Get("project_id")
		}
		out = append(out, map[string]any{"id": id, "name": name, "query": q, "created_at": at, "actor": actor, "project_id": project, "execution_mode": "real"})
	}
	return out, rows.Err()
}
func safeQuery(q string) bool {
	return strings.HasPrefix(q, "/cmdb/") && !strings.Contains(q, "\n") && !strings.HasPrefix(q, "//")
}

func schemaSummary(schema Schema) map[string]any {
	types := []map[string]any{}
	for _, t := range schema.Types {
		catalog := true
		if v, ok := t.Display["catalog"].(bool); ok {
			catalog = v
		}
		types = append(types, map[string]any{"name": t.Name, "label": t.Label, "catalog": catalog})
	}
	return map[string]any{"version": schema.Version, "asset_types": types}
}

func objectHref(kind, id, project string) any {
	switch kind {
	case "execution_reference":
		return "/flows/executions/" + url.PathEscape(id)
	case "issue":
		return "/issues/pool?issue=" + url.QueryEscape(id)
	case "rack":
		return "/racks/" + url.PathEscape(id)
	case "network_segment":
		return "/p/" + url.PathEscape(project) + "/deploy/fabric/topology?view=logical&network_id=" + url.QueryEscape(id)
	case "project":
		return "/p/" + url.PathEscape(id) + "/deploy/overview"
	}
	return nil
}

func (s Service) extraQuality(ctx context.Context, scope Scope, selected string) ([]map[string]any, error) {
	out := []map[string]any{}
	add := func(id, field, code, summary, ref, href, severity string) {
		out = append(out, map[string]any{"id": code + ":" + id + ":" + field, "asset_id": id, "field": field, "code": code, "summary": summary, "source": ref, "href": href, "severity": severity, "status": "open"})
	}
	rows, e := s.DB.Query(ctx, `SELECT e.item_uid FROM cmdb_entity e WHERE e.asset_type='cable' AND `+Visible("e")+` AND ($3='' OR e.item_uid=$3) AND (SELECT count(DISTINCT r.to_uid) FROM cmdb_relationship r WHERE r.from_uid=e.item_uid AND r.definition_id IN('cable_endpoint_a','cable_endpoint_b') AND r.valid_to IS NULL)<2 LIMIT 1000`, append(scope.Args(), selected)...)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var id string
		rows.Scan(&id)
		add(id, "endpoints", "CABLE_RELATION_INCOMPLETE", "线缆缺少可验证的双端物理端口关系", "CMDB typed relationship", "/cmdb/relations?asset="+url.QueryEscape(id), "warning")
	}
	rows.Close()
	rows, e = s.DB.Query(ctx, `SELECT a.item_uid,b.item_uid,a.payload->>'rack_id' FROM cmdb_entity a JOIN cmdb_entity b ON a.item_uid<b.item_uid AND a.payload->>'rack_id'=b.payload->>'rack_id' WHERE a.asset_type IN('server','switch') AND b.asset_type IN('server','switch') AND `+Visible("a")+` AND `+Visible("b")+` AND ($3='' OR a.item_uid=$3 OR b.item_uid=$3) AND CASE WHEN a.payload->>'rack_u' ~ '^[0-9]+$' AND b.payload->>'rack_u' ~ '^[0-9]+$' AND a.payload->>'u_height' ~ '^[0-9]+$' AND b.payload->>'u_height' ~ '^[0-9]+$' THEN (a.payload->>'rack_u')::int < (b.payload->>'rack_u')::int+(b.payload->>'u_height')::int AND (b.payload->>'rack_u')::int < (a.payload->>'rack_u')::int+(a.payload->>'u_height')::int ELSE false END LIMIT 500`, append(scope.Args(), selected)...)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var a, b, rack string
		rows.Scan(&a, &b, &rack)
		if selected == "" || selected == a {
			add(a, "rack_u", "U_CONFLICT", "U 位与 "+b+" 重叠", "BOM/CMDB placement", "/racks/"+url.PathEscape(rack), "critical")
		}
		if selected == b {
			add(b, "rack_u", "U_CONFLICT", "U 位与 "+a+" 重叠", "BOM/CMDB placement", "/racks/"+url.PathEscape(rack), "critical")
		}
	}
	rows.Close()
	rows, e = s.DB.Query(ctx, `SELECT r.summary FROM cmdb_external_reference r JOIN cmdb_entity e ON e.item_uid=ANY(r.asset_ids) WHERE r.resource='cmdb_quality_findings' AND `+Visible("e")+` AND ($3='' OR e.item_uid=$3) LIMIT 1000`, append(scope.Args(), selected)...)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var raw []byte
		rows.Scan(&raw)
		var f map[string]any
		json.Unmarshal(raw, &f)
		if f["code"] != nil && f["asset_id"] != nil {
			f["status"] = defaultString(f["status"], "open")
			out = append(out, f)
		}
	}
	rows.Close()
	return out, nil
}

func (s Service) InventorySnapshot(ctx context.Context, scope Scope) (map[string]any, error) {
	schema, e := s.Active(ctx)
	if e != nil {
		return nil, e
	}
	kinds := []string{}
	for _, t := range schema.Types {
		for _, base := range []string{"server", "switch", "gpu", "nic", "port", "optic", "cable", "rack"} {
			if schema.IsA(t.Name, base) {
				kinds = append(kinds, t.Name)
				break
			}
		}
	}
	rows, e := s.RawAssets(ctx, scope, kinds)
	if e != nil {
		return nil, e
	}
	currentParents := map[string]string{}
	parentRows, e := s.DB.Query(ctx, `SELECT r.to_uid,r.from_uid FROM cmdb_relationship r JOIN cmdb_entity a ON a.item_uid=r.from_uid JOIN cmdb_entity b ON b.item_uid=r.to_uid WHERE r.valid_to IS NULL AND r.definition_id IN('contains','optic_on_port') AND `+RelationshipVisible("r")+` AND `+Visible("a")+` AND `+Visible("b")+` ORDER BY CASE r.fact_layer WHEN 'observed' THEN 0 WHEN 'recorded' THEN 1 ELSE 2 END`, scope.Args()...)
	if e != nil {
		return nil, e
	}
	for parentRows.Next() {
		var child, parent string
		parentRows.Scan(&child, &parent)
		if currentParents[child] == "" {
			currentParents[child] = parent
		}
	}
	parentRows.Close()
	collections := map[string][]map[string]any{"servers": {}, "switches": {}, "gpus": {}, "nics": {}, "ports": {}, "switch_ports": {}, "optics": {}, "cables": {}, "racks": {}, "rack_placements": {}}
	devices := map[string]string{}
	for _, r := range rows {
		if r["category"] == "server" || r["category"] == "switch" {
			devices[stringValue(r["id"])] = stringValue(r["category"])
		}
	}
	for _, r := range rows {
		kind := stringValue(r["category"])
		if kind == "gpu" || kind == "nic" || kind == "port" || kind == "optic" {
			parent := currentParents[stringValue(r["id"])]
			if parent != "" {
				r["parent_uid"] = parent
			}
			owner := parent
			seen := map[string]bool{}
			for owner != "" && devices[owner] == "" && !seen[owner] {
				seen[owner] = true
				owner = currentParents[owner]
			}
			if devices[owner] != "" {
				if kind == "gpu" || kind == "nic" {
					r["server_id"] = owner
				} else {
					r["device_id"] = owner
				}
			}
		}

		resource := map[string]string{"server": "servers", "switch": "switches", "gpu": "gpus", "nic": "nics", "port": "ports", "optic": "optics", "cable": "cables", "rack": "racks"}[kind]
		if kind == "port" && devices[stringValue(r["device_id"])] == "switch" {
			resource = "switch_ports"
		}
		if resource != "" {
			collections[resource] = append(collections[resource], r)
		}
		if kind == "server" || kind == "switch" {
			rack, u, height := stringValue(r["rack_id"]), first(r, "rack_u", "u_start"), r["u_height"]
			if rack != "" && u != nil && height != nil {
				collections["rack_placements"] = append(collections["rack_placements"], map[string]any{"id": "placement-" + stringValue(r["id"]), "item_id": r["id"], "rack_id": rack, "u_start": u, "u_height": height, "face": "front", "source": "cmdb_projection", "revision": r["revision"], "project_id": r["project_id"]})
			}
		}
	}
	placementRows, e := s.DB.Query(ctx, `SELECT r.summary,r.project_id FROM cmdb_external_reference r WHERE r.resource='rack_placements' AND (r.project_id=ANY($1::text[]) OR EXISTS(SELECT 1 FROM unnest(r.asset_ids) aid WHERE r.project_id||':'||aid=ANY($2::text[])))`, scope.Args()...)
	if e != nil {
		return nil, e
	}
	provided := map[string]map[string]any{}
	for placementRows.Next() {
		var raw []byte
		var project string
		placementRows.Scan(&raw, &project)
		var r map[string]any
		json.Unmarshal(raw, &r)
		if r["item_id"] != nil && r["rack_id"] != nil {
			r["project_id"] = project
			provided[stringValue(r["item_id"])] = r
		}
	}
	placementRows.Close()
	for i, p := range collections["rack_placements"] {
		if original := provided[stringValue(p["item_id"])]; original != nil {
			collections["rack_placements"][i] = original
		} else {
			delete(p, "face")
			p["diagram_revision_id"] = nil
			p["status"] = "pending_layout_review"
		}
	}
	return map[string]any{"data": collections, "schema_version": schema.Version, "source": "CMDB PostgreSQL canonical inventory", "truncated": false}, nil
}
func deviceKinds(schema Schema) []string {
	out := []string{}
	for _, t := range schema.Types {
		if schema.IsA(t.Name, "server") || schema.IsA(t.Name, "switch") {
			out = append(out, t.Name)
		}
	}
	return out
}
func inventoryKind(schema Schema, kind string) string {
	for _, base := range []string{"server", "switch", "gpu", "nic", "port", "optic", "cable", "rack"} {
		if schema.IsA(kind, base) {
			return base
		}
	}
	return kind
}

// RelationshipHistory returns immutable relationship revisions only when both endpoints are authorized.
func (s Service) RelationshipHistory(ctx context.Context, scope Scope, id string) ([]map[string]any, error) {
	var visible bool
	e := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_relationship r JOIN cmdb_entity a ON a.item_uid=r.from_uid JOIN cmdb_entity b ON b.item_uid=r.to_uid WHERE r.id::text=$3 AND `+RelationshipVisible("r")+` AND `+Visible("a")+` AND `+Visible("b")+`)`, append(scope.Args(), id)...).Scan(&visible)
	if e != nil {
		return nil, e
	}
	if !visible {
		return nil, Reject("RELATIONSHIP_NOT_FOUND", 404)
	}
	rows, e := s.DB.Query(ctx, `SELECT id::text,version,payload,source_ref,created_at FROM cmdb_relationship_revision WHERE relationship_id::text=$1 ORDER BY version DESC`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var rid, ref string
		var version int64
		var raw []byte
		var at time.Time
		if e = rows.Scan(&rid, &version, &raw, &ref, &at); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": rid, "relationship_id": id, "version": version, "attributes": json.RawMessage(raw), "source_ref": ref, "created_at": at})
	}
	return out, rows.Err()
}
