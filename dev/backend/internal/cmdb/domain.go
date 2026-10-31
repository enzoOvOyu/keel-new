package cmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/identity"
	"strconv"
	"time"
)

type Source struct {
	Type       string         `json:"source_type"`
	Ref        string         `json:"source_ref"`
	Layer      string         `json:"fact_layer"`
	Operator   string         `json:"operator"`
	ObservedAt *time.Time     `json:"observed_at"`
	Location   map[string]any `json:"source_location"`
	Approval   string         `json:"approval_status"`
	Freshness  string         `json:"freshness"`
	Support    string         `json:"support"`
	Evidence   []string       `json:"evidence_refs"`
}
type AssetInput struct {
	ID              string                    `json:"id"`
	Type            string                    `json:"asset_type"`
	Project         string                    `json:"project_id"`
	Attributes      map[string]any            `json:"attributes"`
	Source          Source                    `json:"source"`
	ExpectedVersion *int64                    `json:"expected_revision,omitempty"`
	BOMItemID       *string                   `json:"bom_item_uid,omitempty"`
	BOMRevision     *string                   `json:"bom_revision,omitempty"`
	Locations       map[string]map[string]any `json:"field_locations,omitempty"`
	FieldSources    map[string]Source         `json:"field_sources,omitempty"`
}
type RelationInput struct {
	ID              string `json:"id"`
	Definition      string `json:"relationship_type"`
	SourceID        string `json:"source_asset_id"`
	TargetID        string `json:"target_asset_id"`
	Project         string `json:"project_id"`
	Source          Source `json:"source"`
	ExpectedVersion *int64 `json:"expected_revision,omitempty"`
}

func txSchema(ctx context.Context, tx pgx.Tx) (Schema, error) {
	var b []byte
	var s Schema
	e := tx.QueryRow(ctx, "SELECT document FROM cmdb_schema_version WHERE status='active'").Scan(&b)
	if e == nil {
		e = json.Unmarshal(b, &s)
	}
	return s, e
}
func validLifecycle(v string) string {
	switch v {
	case "planned", "discovered", "provisioned", "verified", "delivered", "maintained", "retired":
		return v
	}
	return "discovered"
}
func (s Service) UpsertAsset(ctx context.Context, tx pgx.Tx, schema Schema, in AssetInput) (int64, error) {
	in.Type = canonicalKind(in.Type)
	if in.ID == "" || in.Project == "" || in.Source.Ref == "" || in.Source.Type == "" {
		return 0, Reject("ASSET_SOURCE_REQUIRED", 422)
	}
	if issues := schema.Validate(in.Type, in.Attributes, "", false); len(issues) > 0 {
		return 0, &Error{422, "ASSET_SCHEMA_VIOLATION", issues}
	}
	var old []byte
	var kind, currentFresh string
	var existingBOM *string
	var version int64
	e := tx.QueryRow(ctx, "SELECT asset_type,payload,version,freshness,bom_item_uid::text FROM cmdb_entity WHERE item_uid=$1 FOR UPDATE", in.ID).Scan(&kind, &old, &version, &currentFresh, &existingBOM)
	if e != nil && e != pgx.ErrNoRows {
		return 0, e
	}
	exists := e == nil
	if existingBOM != nil && in.BOMItemID != nil && *existingBOM != *in.BOMItemID {
		return 0, Reject("BOM_ASSET_IDENTITY_CONFLICT", 409)
	}
	if exists && kind != in.Type && kind != "device" {
		return 0, Reject("ASSET_TYPE_IDENTITY_CONFLICT", 409)
	}
	if in.ExpectedVersion != nil && *in.ExpectedVersion != version {
		return 0, Reject("ASSET_REVISION_CONFLICT", 409)
	}
	incoming := in.Attributes
	attrs := map[string]any{}
	for key, value := range incoming {
		attrs[key] = value
	}
	assetType, _ := schema.Type(in.Type)
	catalogVisible := true
	if v, ok := assetType.Display["catalog"].(bool); ok {
		catalogVisible = v
	}
	changed := !exists
	var previous map[string]any
	if exists {
		json.Unmarshal(old, &previous)
		if oldSerial, newSerial := stringValue(previous["serial"]), stringValue(incoming["serial"]); oldSerial != "" && newSerial != "" && oldSerial != newSerial {
			return 0, Reject("ASSET_IDENTITY_IMMUTABLE", 409)
		}
		_, legacy := previous["item_uid"]
		if legacy {
			if _, e = tx.Exec(ctx, "INSERT INTO cmdb_revision(id,asset_id,version,schema_version,payload,before_payload,source_type,source_ref,operator,project_id) SELECT gen_random_uuid(),item_uid,version,COALESCE(schema_version,'legacy-bom-v0'),payload,NULL,source_type,source_ref,'legacy-migration',$2 FROM cmdb_entity WHERE item_uid=$1 ON CONFLICT(asset_id,version) DO NOTHING", in.ID, in.Project); e != nil {
				return 0, e
			}
		}
		for key, value := range previous {
			if legacy {
				if _, declared := schema.Attributes(in.Type)[key]; !declared {
					continue
				}
				if key == "status" {
					continue
				}
			}
			if _, provided := incoming[key]; !provided {
				attrs[key] = value
			}
		}
		changed = Hash(previous) != Hash(attrs)
	}
	if changed {
		version++
		if !exists {
			version = 1
		}
		for field, a := range schema.Attributes(in.Type) {
			if a.Unique {
				if _, e = tx.Exec(ctx, "DELETE FROM cmdb_unique_attribute WHERE asset_id=$1 AND field_name=$2", in.ID, field); e != nil {
					return 0, e
				}
			}
		}
	}
	name := stringValue(attrs["hostname"])
	if name == "" {
		name = stringValue(attrs["name"])
	}
	if name == "" {
		name = in.ID
	}
	serial := stringValue(attrs["serial"])
	health := stringValue(attrs["health_status"])
	if health == "" {
		health = stringValue(attrs["oper_state"])
	}
	if health == "" {
		health = "unknown"
	}
	maintenance := stringValue(attrs["maintenance_state"])
	if maintenance == "" {
		maintenance = "unknown"
	}
	lifecycle := stringValue(attrs["lifecycle_state"])
	if lifecycle == "" {
		lifecycle = validLifecycle(stringValue(attrs["lifecycle_status"]))
	}
	if in.Source.Type == "bom" && attrs["lifecycle_state"] == nil {
		lifecycle = "planned"
	}
	if lifecycle == "maintained" {
		maintenance = "maintained"
	}
	fresh := in.Source.Freshness
	if fresh == "" {
		fresh = "unknown"
	}
	if exists && (in.Source.Type == "manual" || in.Source.Freshness == "") {
		fresh = currentFresh
	}
	if changed {
		_, e = tx.Exec(ctx, `INSERT INTO cmdb_entity(item_uid,bom_item_uid,payload,asset_type,name,serial,lifecycle_state,health,maintenance_state,freshness,version,schema_version,source_type,source_ref,source_updated_at,revision_id,catalog_visible) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) ON CONFLICT(item_uid) DO UPDATE SET payload=excluded.payload,asset_type=excluded.asset_type,name=excluded.name,serial=excluded.serial,lifecycle_state=excluded.lifecycle_state,health=excluded.health,maintenance_state=excluded.maintenance_state,freshness=excluded.freshness,version=excluded.version,schema_version=excluded.schema_version,source_type=excluded.source_type,source_ref=excluded.source_ref,source_updated_at=COALESCE(excluded.source_updated_at,cmdb_entity.source_updated_at),bom_item_uid=COALESCE(excluded.bom_item_uid,cmdb_entity.bom_item_uid),revision_id=COALESCE(excluded.revision_id,cmdb_entity.revision_id),catalog_visible=excluded.catalog_visible,updated_at=now()`, in.ID, in.BOMItemID, Encode(attrs), in.Type, name, serial, lifecycle, health, maintenance, fresh, version, schema.Version, in.Source.Type, in.Source.Ref, in.Source.ObservedAt, in.BOMRevision, catalogVisible)
		if e != nil {
			return 0, e
		}
		var before any
		if exists {
			before = previous
		}
		_, e = tx.Exec(ctx, "INSERT INTO cmdb_revision(id,asset_id,version,schema_version,payload,before_payload,source_type,source_ref,operator,project_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", uuid.NewString(), in.ID, version, schema.Version, Encode(attrs), Encode(before), in.Source.Type, in.Source.Ref, in.Source.Operator, in.Project)
		if e != nil {
			return 0, e
		}
		for field, a := range schema.Attributes(in.Type) {
			if a.Unique && attrs[field] != nil && stringValue(attrs[field]) != "" {
				if _, e = tx.Exec(ctx, "INSERT INTO cmdb_unique_attribute(asset_type,field_name,value_key,asset_id) VALUES($1,$2,$3,$4)", in.Type, field, Hash(attrs[field]), in.ID); e != nil {
					return 0, Reject("UNIQUE_ATTRIBUTE_CONFLICT", 409)
				}
			}
		}
	}
	if in.BOMItemID != nil {
		if _, e = tx.Exec(ctx, "UPDATE cmdb_entity SET bom_item_uid=$2,revision_id=COALESCE($3,revision_id) WHERE item_uid=$1", in.ID, *in.BOMItemID, in.BOMRevision); e != nil {
			return 0, e
		}
	}
	_, e = tx.Exec(ctx, "INSERT INTO cmdb_asset_scope(project_id,asset_id,source_ref) VALUES($1,$2,$3) ON CONFLICT(project_id,asset_id) DO NOTHING", in.Project, in.ID, in.Source.Ref)
	if e != nil {
		return 0, e
	}
	batch := &pgx.Batch{}
	count := 0
	for field, value := range incoming {
		if exists && in.Source.Type == "manual" && Hash(previous[field]) == Hash(value) {
			continue
		}
		source := in.Source
		if explicit, ok := in.FieldSources[field]; ok {
			source = explicit
		}
		location := source.Location
		if specific, ok := in.Locations[field]; ok {
			location = specific
		}
		if location == nil {
			location = map[string]any{}
		}
		layer := source.Layer
		if layer == "" {
			layer = "formal"
		}
		approval := source.Approval
		if approval == "" {
			approval = "confirmed"
		}
		support := source.Support
		if support == "" {
			support = "unknown"
		}
		fresh := source.Freshness
		if fresh == "" {
			fresh = "unknown"
		}
		refs := source.Evidence
		if refs == nil {
			refs = []string{source.Ref}
		}
		batch.Queue(`INSERT INTO cmdb_field_source(id,asset_id,field_name,value,source_type,source_ref,source_location,fact_layer,observed_at,revision,operator,approval_status,support,freshness,evidence_refs,project_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) ON CONFLICT(asset_id,field_name,source_ref,fact_layer) DO NOTHING`, uuid.NewString(), in.ID, field, Encode(value), source.Type, source.Ref, Encode(location), layer, source.ObservedAt, strconv.FormatInt(version, 10), source.Operator, approval, support, fresh, Encode(refs), in.Project)
		count++
	}
	if count > 0 {
		results := tx.SendBatch(ctx, batch)
		for i := 0; i < count; i++ {
			if _, e = results.Exec(); e != nil {
				results.Close()
				return 0, e
			}
		}
		if e = results.Close(); e != nil {
			return 0, e
		}
	}

	return version, nil
}
func (s Service) PutRelation(ctx context.Context, tx pgx.Tx, schema Schema, in RelationInput) (string, error) {
	d, ok := schema.Definition(in.Definition)
	if !ok {
		return "", Reject("UNKNOWN_RELATIONSHIP_TYPE", 422)
	}
	if in.SourceID == in.TargetID {
		return "", Reject("RELATIONSHIP_SELF_CYCLE", 422)
	}
	if !contains(d.Sources, in.Source.Type) {
		return "", Reject("RELATIONSHIP_SOURCE_DENIED", 403)
	}
	var sourceKind, targetKind string
	var sourceScope, targetScope bool
	e := tx.QueryRow(ctx, `SELECT a.asset_type,b.asset_type,EXISTS(SELECT 1 FROM cmdb_asset_scope WHERE project_id=$3 AND asset_id=a.item_uid),EXISTS(SELECT 1 FROM cmdb_asset_scope WHERE project_id=$3 AND asset_id=b.item_uid) FROM cmdb_entity a,cmdb_entity b WHERE a.item_uid=$1 AND b.item_uid=$2`, in.SourceID, in.TargetID, in.Project).Scan(&sourceKind, &targetKind, &sourceScope, &targetScope)
	if e != nil || !sourceScope || !targetScope {
		return "", Reject("RELATIONSHIP_ENDPOINT_SCOPE", 422)
	}
	matches := func(kind string, types []string) bool {
		for _, t := range types {
			if schema.IsA(kind, t) {
				return true
			}
		}
		return false
	}
	if !matches(sourceKind, d.SourceTypes) || !matches(targetKind, d.TargetTypes) {
		return "", Reject("RELATIONSHIP_ENDPOINT_TYPE", 422)
	}
	// Shared transaction lock serializes cardinality checks for either endpoint.
	locks := []string{in.SourceID, in.TargetID}
	if locks[0] > locks[1] {
		locks[0], locks[1] = locks[1], locks[0]
	}
	for _, id := range locks {
		if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,14))", id); e != nil {
			return "", e
		}
	}
	if d.Name == "contains" || d.Name == "parent" {
		var cycle, limited bool
		e = tx.QueryRow(ctx, `WITH RECURSIVE walk(id,depth,path) AS(SELECT $2::text,0,ARRAY[$2::text] UNION ALL SELECT r.to_uid,w.depth+1,w.path||r.to_uid FROM walk w JOIN cmdb_relationship r ON r.from_uid=w.id AND r.definition_id=$3 AND r.valid_to IS NULL WHERE w.depth<32 AND NOT r.to_uid=ANY(w.path)) SELECT bool_or(id=$1),bool_or(depth=32) FROM walk`, in.SourceID, in.TargetID, d.Name).Scan(&cycle, &limited)
		if e != nil {
			return "", e
		}
		if cycle {
			return "", Reject("RELATIONSHIP_CYCLE", 422)
		}
		if limited {
			return "", Reject("RELATIONSHIP_DEPTH_UNVERIFIED", 422)
		}
	}
	layer := in.Source.Layer
	if layer == "" || layer == "formal" || layer == "manual" {
		layer = "recorded"
	}
	var existing string
	var version int64
	e = tx.QueryRow(ctx, "SELECT id::text,version FROM cmdb_relationship WHERE from_uid=$1 AND to_uid=$2 AND definition_id=$3 AND fact_layer=$4 AND valid_to IS NULL", in.SourceID, in.TargetID, in.Definition, layer).Scan(&existing, &version)
	if e == nil {
		if in.ExpectedVersion != nil && *in.ExpectedVersion != version {
			return "", Reject("RELATIONSHIP_REVISION_CONFLICT", 409)
		}
		return existing, nil
	}
	if e != pgx.ErrNoRows {
		return "", e
	}
	check := func(column, value string, max int) error {
		if max == 0 {
			return nil
		}
		var count int
		e := tx.QueryRow(ctx, "SELECT count(*) FROM cmdb_relationship WHERE "+column+"=$1 AND definition_id=$2 AND fact_layer=$3 AND valid_to IS NULL", value, d.Name, layer).Scan(&count)
		if e != nil {
			return e
		}
		if count >= max {
			return Reject("RELATIONSHIP_CARDINALITY", 422)
		}
		return nil
	}
	if e = check("from_uid", in.SourceID, d.MaxTargets); e != nil {
		return "", e
	}
	if e = check("to_uid", in.TargetID, d.MaxSources); e != nil {
		return "", e
	}
	if in.Definition == "cable_endpoint_a" || in.Definition == "cable_endpoint_b" {
		var occupied bool
		e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM cmdb_relationship WHERE to_uid=$1 AND definition_id IN('cable_endpoint_a','cable_endpoint_b') AND fact_layer=$2 AND valid_to IS NULL)", in.TargetID, layer).Scan(&occupied)
		if e != nil {
			return "", e
		}
		if occupied {
			return "", Reject("PORT_ALREADY_CABLED", 422)
		}
	}
	id := in.ID
	if id == "" {
		id = uuid.NewString()
	}
	cardinality := "many-to-many"
	if d.MaxTargets == 1 && d.MaxSources == 1 {
		cardinality = "one-to-one"
	} else if d.MaxTargets == 1 {
		cardinality = "many-to-one"
	} else if d.MaxSources == 1 {
		cardinality = "one-to-many"
	}
	_, e = tx.Exec(ctx, `INSERT INTO cmdb_relationship(id,from_uid,to_uid,kind,definition_id,project_id,fact_layer,source_type,source_ref,direction,cardinality,definition_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, in.SourceID, in.TargetID, d.Kind, d.Name, in.Project, layer, in.Source.Type, in.Source.Ref, d.Direction, cardinality, schema.Version)
	if e != nil {
		return "", e
	}
	_, e = tx.Exec(ctx, "INSERT INTO cmdb_relationship_revision VALUES($1,$2,1,$3,$4,now())", uuid.NewString(), id, Encode(in), in.Source.Ref)
	return id, e
}
func (s Service) EndRelation(ctx context.Context, tx pgx.Tx, id, project, ref string, expected int64) error {
	var raw []byte
	var version int64
	e := tx.QueryRow(ctx, "SELECT to_jsonb(r),version FROM cmdb_relationship r WHERE id::text=$1 AND project_id=$2 AND valid_to IS NULL FOR UPDATE", id, project).Scan(&raw, &version)
	if e != nil {
		return Reject("RELATIONSHIP_NOT_FOUND", 404)
	}
	if expected != version {
		return Reject("RELATIONSHIP_REVISION_CONFLICT", 409)
	}
	_, e = tx.Exec(ctx, "UPDATE cmdb_relationship SET valid_to=now(),relationship_status='historical',version=version+1 WHERE id::text=$1", id)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "INSERT INTO cmdb_relationship_revision VALUES($1,$2,$3,$4,$5,now())", uuid.NewString(), id, version+1, Encode(map[string]any{"before": json.RawMessage(raw), "status": "historical"}), ref)
	return e
}
func (s Service) Audit(ctx context.Context, tx pgx.Tx, project, actor, action, id string, details any) error {
	_, e := tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,$4,$5,$6)", uuid.NewString(), project, actor, "cmdb."+action, id, Encode(details))
	return e
}
func (s Service) AddEvent(ctx context.Context, tx pgx.Tx, project, id, asset, kind, label, status, source, ref, actor string, details any, links []map[string]string) error {
	if links == nil {
		links = []map[string]string{}
	}
	_, e := tx.Exec(ctx, "INSERT INTO cmdb_lifecycle_event(id,project_id,asset_id,event_type,label,status,source_type,source_ref,operator,details,links) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(id) DO NOTHING", id, project, asset, kind, label, status, source, ref, actor, Encode(details), Encode(links))
	return e
}
func (s Service) ProjectBOM(ctx context.Context, tx pgx.Tx, event, project, revision string, items []bom.Item) error {
	var processed bool
	if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM cmdb_ingest_event WHERE event_id=$1)", event).Scan(&processed); e != nil {
		return e
	}
	if processed {
		return nil
	}
	schema, e := txSchema(ctx, tx)
	if e != nil {
		return e
	}
	if e = s.ProjectReference(ctx, tx, schema, project); e != nil {
		return e
	}
	var importID, doc, actor string
	var when time.Time
	e = tx.QueryRow(ctx, "SELECT r.import_id::text,b.source_document_id::text,r.created_at,COALESCE((SELECT actor FROM audit_event WHERE resource_id=b.id::text AND action='bom.committed' ORDER BY created_at DESC LIMIT 1),'bom-projector') FROM bom_revision r JOIN bom_import_batch b ON b.id=r.import_id WHERE r.id=$1", revision).Scan(&importID, &doc, &when, &actor)
	if e != nil {
		return e
	}
	aliases := map[string]string{}
	for _, item := range items {
		canonical := item.UID
		var found string
		err := tx.QueryRow(ctx, "SELECT item_uid FROM cmdb_entity WHERE bom_item_uid=$1", item.UID).Scan(&found)
		if err == nil {
			canonical = found
		} else if err != pgx.ErrNoRows {
			return err
		} else if item.Serial != "" {
			err = tx.QueryRow(ctx, "SELECT item_uid FROM cmdb_entity WHERE asset_type=$1 AND serial=$2", canonicalKind(bom.AssetType(item)), item.Serial).Scan(&found)
			if err == nil {
				canonical = found
			} else if err != pgx.ErrNoRows {
				return err
			}
		}
		var exists, inProject bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM cmdb_entity WHERE item_uid=$1),EXISTS(SELECT 1 FROM cmdb_asset_scope WHERE asset_id=$1 AND project_id=$2)", canonical, project).Scan(&exists, &inProject); e != nil {
			return e
		}
		if exists && !inProject {
			var userID string
			if e = tx.QueryRow(ctx, "SELECT id::text FROM app_user WHERE username=$1 AND status='active'", actor).Scan(&userID); e != nil {
				return Reject("CROSS_PROJECT_ASSET_ASSOCIATION_DENIED", 403)
			}
			subject, authErr := (identity.Service{DB: s.DB}).Load(ctx, userID)
			if authErr != nil || !subject.Can("bom:execute", project, "", "") {
				return Reject("CROSS_PROJECT_ASSET_ASSOCIATION_DENIED", 403)
			}
			scopes, e := tx.Query(ctx, "SELECT project_id FROM cmdb_asset_scope WHERE asset_id=$1", canonical)
			if e != nil {
				return e
			}
			allowed := false
			for scopes.Next() {
				var p string
				if e = scopes.Scan(&p); e != nil {
					scopes.Close()
					return e
				}
				if subject.Can("cmdb:read", p, "cmdb_asset", canonical) {
					allowed = true
				}
			}
			scopes.Close()
			if !allowed {
				return Reject("CROSS_PROJECT_ASSET_ASSOCIATION_DENIED", 403)
			}
		}
		aliases[item.UID] = canonical
	}
	resolve := func(id string) string {
		if stable := aliases[id]; stable != "" {
			return stable
		}
		return id
	}
	for _, item := range items {
		attrs := map[string]any{}
		for k, v := range item.Fields {
			// An absent optional dimension is unknown, never an empty numeric value.
			if v == "" {
				continue
			}
			if a, ok := schema.Attributes(canonicalKind(bom.AssetType(item)))[k]; ok {
				attrs[k] = v
				if a.Type == "Number" {
					if n, e := strconv.ParseFloat(bom.RackUnitValue(k, v), 64); e == nil {
						attrs[k] = n
					}
				}
			}
		}
		for k, v := range map[string]string{"hostname": item.Hostname, "serial": item.Serial, "model": item.Model, "port_index": item.Port, "parent_uid": resolve(item.Parent), "endpoint_a_uid": resolve(item.A), "endpoint_b_uid": resolve(item.B)} {
			if v != "" {
				if a, ok := schema.Attributes(canonicalKind(bom.AssetType(item)))[k]; ok {
					attrs[k] = v
					if a.Type == "Number" {
						if n, e := strconv.ParseFloat(v, 64); e == nil {
							attrs[k] = n
						}
					}
				}
			}
		}
		// A physical port label is not a numeric ordinal. Preserve its source identity.
		if bom.AssetType(item) == "physical_port" || item.Category == "port" {
			attrs["physical_name"] = item.Port
			if _, err := strconv.ParseFloat(item.Port, 64); err != nil {
				delete(attrs, "port_index")
			}
		}
		locations := map[string]map[string]any{}
		for key, c := range item.Sources {
			locations[key] = map[string]any{"import_id": importID, "document_id": doc, "sheet": c.Sheet, "cell": c.Ref, "row": c.Row, "column": c.Column}
		}
		if item.Category == "port" {
			locations["physical_name"] = locations["port_index"]
		}
		// Do not replace newer canonical asset facts when old Outbox events are replayed.
		var newer bool
		tx.QueryRow(ctx, "SELECT COALESCE(source_updated_at>$2,false) FROM cmdb_entity WHERE item_uid=$1", item.UID, when).Scan(&newer)
		if newer {
			continue
		}
		uid := resolve(item.UID)
		bomUID := item.UID
		_, e = s.UpsertAsset(ctx, tx, schema, AssetInput{ID: uid, Type: bom.AssetType(item), Project: project, Attributes: attrs, Source: Source{Type: "bom", Ref: importID, Layer: "planned", Operator: actor, ObservedAt: &when, Approval: "approved", Evidence: []string{revision, doc}}, BOMItemID: &bomUID, BOMRevision: &revision, Locations: locations})
		if e != nil {
			return e
		}
		if e = s.AddEvent(ctx, tx, project, "bom:"+event+":"+uid, uid, "bom.published", "标准 BOM 发布", "published", "bom", importID, actor, map[string]string{"bom_revision": revision}, []map[string]string{{"label": "打开 BOM", "href": "/p/" + project + "/deploy/bom/" + importID}}); e != nil {
			return e
		}
	}
	for _, item := range items {
		links := []RelationInput{}
		source := Source{Type: "bom", Ref: revision, Layer: "planned", Operator: actor}
		if item.Parent != "" {
			links = append(links, RelationInput{Definition: "contains", SourceID: resolve(item.Parent), TargetID: resolve(item.UID)})
		}
		for _, end := range []struct{ Name, ID string }{{"cable_endpoint_a", item.A}, {"cable_endpoint_b", item.B}} {
			if end.ID != "" {
				links = append(links, RelationInput{Definition: end.Name, SourceID: resolve(item.UID), TargetID: resolve(end.ID)})
			}
		}
		if port := item.Fields["port_uid"]; port != "" && bom.AssetType(item) == "optic_module" {
			links = append(links, RelationInput{Definition: "optic_on_port", SourceID: resolve(port), TargetID: resolve(item.UID)})
		}
		if rack := item.Fields["rack_uid"]; rack != "" {
			links = append(links, RelationInput{Definition: "located_in", SourceID: resolve(item.UID), TargetID: resolve(rack)})
		}
		for _, rel := range links {
			rel.Project = project
			rel.Source = source
			if _, e = s.PutRelation(ctx, tx, schema, rel); e != nil {
				return fmt.Errorf("BOM relation %s: %w", rel.Definition, e)
			}
		}
	}
	for _, item := range items {
		if _, e = s.PutRelation(ctx, tx, schema, RelationInput{Definition: "project_association", SourceID: aliases[item.UID], TargetID: project, Project: project, Source: Source{Type: "derived", Ref: "bom:" + revision, Layer: "recorded", Operator: actor}}); e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, "INSERT INTO cmdb_ingest_event(event_id,source_type,source_ref) VALUES($1,'bom',$2)", event, revision)
	return e
}

// ProjectReference projects the existing project identity; it does not create a second project authority.
func (s Service) ProjectReference(ctx context.Context, tx pgx.Tx, schema Schema, project string) error {
	var name string
	if e := tx.QueryRow(ctx, "SELECT name FROM project WHERE id=$1", project).Scan(&name); e != nil {
		return e
	}
	_, e := s.UpsertAsset(ctx, tx, schema, AssetInput{ID: project, Type: "project", Project: project, Attributes: map[string]any{"name": name, "source_ref": project, "resource": "projects"}, Source: Source{Type: "derived", Ref: "projects:" + project, Layer: "recorded", Operator: "project-projection"}})
	return e
}
