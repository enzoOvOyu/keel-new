package cmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
)

// engineeringAssetWrite is collected by projectEngineering. Attrs includes all
// planning facts; only declared attributes enter the asset's current payload.
type engineeringAssetWrite struct {
	ID, Kind, Label string
	Attrs           map[string]any
}

const engineeringAssetBatchSize = 500
const engineeringSourceBatchSize = 2000
const engineeringBatchBytes = 4 << 20

type engineeringAssetState struct {
	Kind, Ref, Schema     string
	Payload               json.RawMessage
	Version               int64
	BOM                   *string
	InProject, Nonplanned bool
	Fields                map[string]bool
}

type engineeringAssetRow struct {
	ID                                           string          `json:"id"`
	Kind                                         string          `json:"kind"`
	BOM                                          json.RawMessage `json:"bom"`
	Payload                                      map[string]any  `json:"payload"`
	Before                                       json.RawMessage `json:"before"`
	Version                                      int64           `json:"version"`
	Exists                                       bool            `json:"exists"`
	Changed                                      bool            `json:"changed"`
	Protected                                    bool            `json:"protected"`
	Advance                                      bool            `json:"advance"`
	Legacy                                       bool            `json:"legacy"`
	OldVersion                                   int64           `json:"old_version"`
	OldSchema                                    string          `json:"old_schema"`
	OldRef                                       string          `json:"old_ref"`
	Name, Serial, Lifecycle, Health, Maintenance string
	Catalog                                      bool              `json:"catalog"`
	UniqueFields                                 []string          `json:"unique_fields"`
	UniqueValues                                 map[string]string `json:"unique_values"`
}

type engineeringSourceRow struct {
	ID       string `json:"asset_id"`
	Field    string `json:"field"`
	Value    any    `json:"value"`
	Revision string `json:"revision"`
}

// writeEngineeringAssets flushes a collected publication within the caller's
// transaction. The caller owns project authorization, publication serialization,
// rollback on any error, events and relations. No transaction is committed here.
func (s Service) writeEngineeringAssets(ctx context.Context, tx pgx.Tx, schema Schema, project, publication, actor string, assets []engineeringAssetWrite) error {
	if len(assets) == 0 {
		return nil
	}
	if project == "" || publication == "" {
		return Reject("ASSET_SOURCE_REQUIRED", 422)
	}
	ordered := append([]engineeringAssetWrite(nil), assets...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	// Duplicate IDs must not make one INSERT update the same row twice. Identical
	// collection entries are harmless; conflicting identities/facts are not merged.
	unique := ordered[:0]
	for _, in := range ordered {
		if in.ID == "" {
			return Reject("ASSET_SOURCE_REQUIRED", 422)
		}
		if len(unique) > 0 && unique[len(unique)-1].ID == in.ID {
			if Hash(unique[len(unique)-1]) != Hash(in) {
				return Reject("ENGINEERING_ASSET_INPUT_CONFLICT", 409)
			}
			continue
		}
		unique = append(unique, in)
	}
	for offset := 0; offset < len(unique); {
		end, size := offset, 2
		for end < len(unique) && end-offset < engineeringAssetBatchSize {
			raw, err := json.Marshal(unique[end])
			if err != nil {
				return err
			}
			if len(raw)+2 > engineeringBatchBytes {
				return Reject("ENGINEERING_ASSET_BATCH_ITEM_TOO_LARGE", 422)
			}
			if size+len(raw)+1 > engineeringBatchBytes {
				break
			}
			size += len(raw) + 1
			end++
		}
		batch := unique[offset:end]
		states, err := loadEngineeringAssetStates(ctx, tx, project, publication, batch)
		if err != nil {
			return err
		}
		rows := make([]engineeringAssetRow, 0, len(batch))
		sources := []engineeringSourceRow{}
		for _, in := range batch {
			row, fields, err := prepareEngineeringAsset(schema, project, publication, in, states[in.ID])
			if err != nil {
				return err
			}
			rows = append(rows, row)
			sources = append(sources, fields...)
		}
		// Encoded rows include both BOM and revision payloads; limit these again,
		// rather than assuming the input byte limit covers the expanded write set.
		if err = writeEngineeringAssetRows(ctx, tx, schema, project, publication, actor, rows); err != nil {
			return err
		}
		if err = writeEngineeringSourceRows(ctx, tx, project, publication, actor, sources); err != nil {
			return err
		}
		offset = end
	}
	return nil
}

func loadEngineeringAssetStates(ctx context.Context, tx pgx.Tx, project, publication string, assets []engineeringAssetWrite) (map[string]*engineeringAssetState, error) {
	ids := make([]string, len(assets))
	for i, in := range assets {
		ids[i] = in.ID
	}
	states := map[string]*engineeringAssetState{}
	// Lock in global ID order. Read sources in a second statement after acquiring
	// locks, so a writer we waited for cannot leave us with a stale source snapshot.
	rows, err := tx.Query(ctx, `SELECT item_uid,asset_type,payload,version,source_ref,bom_item_uid::text,COALESCE(schema_version,'legacy-bom-v0') FROM cmdb_entity WHERE item_uid=ANY($1::text[]) ORDER BY item_uid FOR UPDATE`, ids)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		state := &engineeringAssetState{Fields: map[string]bool{}}
		if err = rows.Scan(&id, &state.Kind, &state.Payload, &state.Version, &state.Ref, &state.BOM, &state.Schema); err != nil {
			rows.Close()
			return nil, err
		}
		states[id] = state
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `WITH sources AS (
 SELECT asset_id,bool_or(fact_layer<>'planned') AS nonplanned,
 COALESCE(array_agg(field_name) FILTER (WHERE fact_layer='planned' AND source_ref=$3),ARRAY[]::text[]) AS fields
 FROM cmdb_field_source WHERE asset_id=ANY($1::text[]) GROUP BY asset_id)
 SELECT i.id,s.asset_id IS NOT NULL,COALESCE(f.nonplanned,false),COALESCE(f.fields,ARRAY[]::text[])
 FROM unnest($1::text[]) AS i(id)
 LEFT JOIN cmdb_asset_scope s ON s.asset_id=i.id AND s.project_id=$2
 LEFT JOIN sources f ON f.asset_id=i.id`, ids, project, "engineering:"+publication)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var scope, nonplanned bool
		var fields []string
		if err = rows.Scan(&id, &scope, &nonplanned, &fields); err != nil {
			rows.Close()
			return nil, err
		}
		if state := states[id]; state != nil {
			state.InProject = scope
			state.Nonplanned = nonplanned
			for _, field := range fields {
				state.Fields[field] = true
			}
		}
	}
	err = rows.Err()
	rows.Close()
	return states, err
}

func prepareEngineeringAsset(schema Schema, project, publication string, in engineeringAssetWrite, old *engineeringAssetState) (engineeringAssetRow, []engineeringSourceRow, error) {
	row := engineeringAssetRow{ID: in.ID, Kind: canonicalKind(in.Kind), Version: 1, UniqueFields: []string{}, UniqueValues: map[string]string{}}
	fail := func(code string, status int) (engineeringAssetRow, []engineeringSourceRow, error) {
		return row, nil, Reject(code, status)
	}
	declared := schema.Attributes(row.Kind)
	incoming := map[string]any{}
	for key, value := range in.Attrs {
		if _, ok := declared[key]; ok && value != nil {
			incoming[key] = value
		}
	}
	if issues := schema.Validate(row.Kind, incoming, "", false); len(issues) > 0 {
		return row, nil, &Error{422, "ASSET_SCHEMA_VIOLATION", issues}
	}
	row.Payload = map[string]any{}
	for key, value := range incoming {
		row.Payload[key] = value
	}
	if old != nil {
		if !old.InProject {
			return fail("CROSS_PROJECT_ASSET_ASSOCIATION_DENIED", 403)
		}
		if old.Kind != row.Kind {
			return fail("ASSET_TYPE_IDENTITY_CONFLICT", 409)
		}
		if old.BOM != nil && *old.BOM != in.ID {
			return fail("BOM_ASSET_IDENTITY_CONFLICT", 409)
		}
		previous := map[string]any{}
		if err := json.Unmarshal(old.Payload, &previous); err != nil {
			return row, nil, err
		}
		if a, b := stringValue(previous["serial"]), stringValue(incoming["serial"]); a != "" && b != "" && a != b {
			return fail("ASSET_IDENTITY_IMMUTABLE", 409)
		}
		row.Exists = true
		row.Version = old.Version
		row.Before = old.Payload
		row.OldVersion = old.Version
		row.OldSchema = old.Schema
		row.OldRef = old.Ref
		row.Protected = old.Nonplanned || !strings.HasPrefix(old.Ref, "engineering:")
		_, row.Legacy = previous["item_uid"]
		if row.Protected {
			row.Payload = previous
			row.Legacy = false
		} else {
			for key, value := range previous {
				if row.Legacy {
					if _, ok := declared[key]; !ok || key == "status" {
						continue
					}
				}
				if _, provided := incoming[key]; !provided {
					row.Payload[key] = value
				}
			}
			row.Changed = Hash(previous) != Hash(row.Payload)
		}
	} else {
		row.Changed = true
	}
	sources := []engineeringSourceRow{}
	for _, field := range sortedKeys(in.Attrs) {
		value := in.Attrs[field]
		if value == nil {
			continue
		}
		if old != nil && old.Fields[field] {
			continue
		}
		sources = append(sources, engineeringSourceRow{ID: in.ID, Field: field, Value: value})
	}
	// New planning evidence advances a protected asset's read revision once,
	// without replacing its observed/confirmed state. Replays add no revision.
	row.Advance = row.Protected && len(sources) > 0
	if old != nil && (row.Changed || row.Advance) {
		row.Version++
	}
	for i := range sources {
		sources[i].Revision = strconv.FormatInt(row.Version, 10)
	}
	item := bom.Item{UID: in.ID, Category: row.Kind, Hostname: in.Label, Fields: map[string]string{}}
	for key, value := range in.Attrs {
		if value == nil {
			continue
		}
		if _, typed := value.(engineering.TypedValue); typed {
			item.Fields[key] = string(Encode(value))
		} else {
			item.Fields[key] = fmt.Sprint(value)
		}
	}
	if model, ok := in.Attrs["model"].(string); ok {
		item.Model = model
	}
	var err error
	row.BOM, err = json.Marshal(item)
	if err != nil {
		return row, nil, err
	}
	row.Name = stringValue(row.Payload["hostname"])
	if row.Name == "" {
		row.Name = stringValue(row.Payload["name"])
	}
	if row.Name == "" {
		row.Name = in.ID
	}
	row.Serial = stringValue(row.Payload["serial"])
	row.Health = stringValue(row.Payload["health_status"])
	if row.Health == "" {
		row.Health = stringValue(row.Payload["oper_state"])
	}
	if row.Health == "" {
		row.Health = "unknown"
	}
	row.Maintenance = stringValue(row.Payload["maintenance_state"])
	if row.Maintenance == "" {
		row.Maintenance = "unknown"
	}
	row.Lifecycle = stringValue(row.Payload["lifecycle_state"])
	if row.Lifecycle == "" {
		row.Lifecycle = validLifecycle(stringValue(row.Payload["lifecycle_status"]))
	}
	if row.Payload["lifecycle_state"] == nil {
		row.Lifecycle = "planned"
	}
	if row.Lifecycle == "maintained" {
		row.Maintenance = "maintained"
	}
	assetType, _ := schema.Type(row.Kind)
	row.Catalog = true
	if visible, ok := assetType.Display["catalog"].(bool); ok {
		row.Catalog = visible
	}
	if row.Changed {
		for field, a := range declared {
			if a.Unique {
				row.UniqueFields = append(row.UniqueFields, field)
				if row.Payload[field] != nil && stringValue(row.Payload[field]) != "" {
					row.UniqueValues[field] = Hash(row.Payload[field])
				}
			}
		}
		sort.Strings(row.UniqueFields)
	}
	return row, sources, nil
}

// Every write below consumes a set, never one SQL statement per asset/field.
const engineeringAssetRecords = `jsonb_to_recordset($1::jsonb) AS x(id text,kind text,bom jsonb,payload jsonb,before jsonb,version bigint,"exists" boolean,changed boolean,protected boolean,advance boolean,legacy boolean,old_version bigint,old_schema text,old_ref text,"Name" text,"Serial" text,"Lifecycle" text,"Health" text,"Maintenance" text,catalog boolean,unique_fields jsonb,unique_values jsonb)`

func writeEngineeringAssetRows(ctx context.Context, tx pgx.Tx, schema Schema, project, publication, actor string, rows []engineeringAssetRow) error {
	return engineeringJSONBatches(rows, engineeringAssetBatchSize, func(data []byte) error {
		// All statements share the same parameter contract, including statements
		// which need only a subset. The CTE fixes PostgreSQL parameter types.
		prefix := `WITH parameters AS (SELECT $2::text AS project,$3::uuid AS publication,$4::text AS actor,$5::text AS schema,$6::text AS ref) `
		statements := []string{
			`INSERT INTO bom_item(item_uid,identity_key,category,payload) SELECT x.id::uuid,'engineering:'||p.project||':'||x.id,x.kind,x.bom FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p ON CONFLICT(item_uid) DO NOTHING`,
			`INSERT INTO bom_item_revision(revision_id,item_uid,payload) SELECT p.publication,x.id::uuid,x.bom FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p ON CONFLICT DO NOTHING`,
			`INSERT INTO project_asset(project_id,item_uid,revision_id) SELECT p.project,x.id::uuid,p.publication FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p ON CONFLICT DO NOTHING`,
			`INSERT INTO cmdb_revision(id,asset_id,version,schema_version,payload,before_payload,source_type,source_ref,operator,project_id) SELECT gen_random_uuid(),x.id,x.old_version,x.old_schema,x.before,NULL,e.source_type,x.old_ref,'legacy-migration',p.project FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p JOIN cmdb_entity e ON e.item_uid=x.id WHERE x.legacy ON CONFLICT(asset_id,version) DO NOTHING`,
			`DELETE FROM cmdb_unique_attribute u USING ` + engineeringAssetRecords + ` WHERE x.changed AND u.asset_id=x.id AND x.unique_fields ? u.field_name`,
			// No ON CONFLICT for new identities: a concurrent insertion must abort,
			// never bypass the scope/source checks performed on existing locked rows.
			`INSERT INTO cmdb_entity(item_uid,bom_item_uid,payload,asset_type,name,serial,lifecycle_state,health,maintenance_state,freshness,version,schema_version,source_type,source_ref,revision_id,catalog_visible) SELECT x.id,x.id::uuid,x.payload,x.kind,x."Name",NULLIF(x."Serial",''),x."Lifecycle",x."Health",x."Maintenance",'unknown',x.version,p.schema,'bom',p.ref,p.publication,x.catalog FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p WHERE NOT x."exists"`,
			`UPDATE cmdb_entity e SET payload=x.payload,asset_type=x.kind,name=x."Name",serial=NULLIF(x."Serial",''),lifecycle_state=x."Lifecycle",health=x."Health",maintenance_state=x."Maintenance",freshness='unknown',version=x.version,schema_version=p.schema,source_type='bom',source_ref=p.ref,bom_item_uid=x.id::uuid,revision_id=p.publication,catalog_visible=x.catalog,updated_at=now() FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p WHERE e.item_uid=x.id AND x."exists" AND x.changed`,
			`UPDATE cmdb_entity e SET version=x.version,updated_at=now() FROM ` + engineeringAssetRecords + ` WHERE e.item_uid=x.id AND x.advance`,
			`UPDATE cmdb_entity e SET bom_item_uid=x.id::uuid,revision_id=p.publication FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p WHERE e.item_uid=x.id AND x."exists" AND NOT x.protected AND NOT x.changed AND (e.bom_item_uid IS DISTINCT FROM x.id::uuid OR e.revision_id IS DISTINCT FROM p.publication)`,
			`INSERT INTO cmdb_revision(id,asset_id,version,schema_version,payload,before_payload,source_type,source_ref,operator,project_id) SELECT gen_random_uuid(),x.id,x.version,p.schema,x.payload,x.before,'bom',p.ref,p.actor,p.project FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p WHERE x.changed OR x.advance`,
			`INSERT INTO cmdb_unique_attribute(asset_type,field_name,value_key,asset_id) SELECT x.kind,u.key,u.value,x.id FROM ` + engineeringAssetRecords + ` CROSS JOIN LATERAL jsonb_each_text(x.unique_values) u WHERE x.changed`,
			`INSERT INTO cmdb_asset_scope(project_id,asset_id,source_ref) SELECT p.project,x.id,p.ref FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p ON CONFLICT(project_id,asset_id) DO NOTHING`,
			`UPDATE eng_entity e SET bom_item_uid=x.id::uuid FROM ` + engineeringAssetRecords + ` CROSS JOIN parameters p WHERE e.project_id=p.project AND e.id=x.id::uuid AND e.bom_item_uid IS DISTINCT FROM x.id::uuid`,
		}
		for _, sql := range statements {
			if _, err := tx.Exec(ctx, prefix+sql, data, project, publication, actor, schema.Version, "engineering:"+publication); err != nil {
				if strings.HasPrefix(sql, "INSERT INTO cmdb_unique_attribute") {
					if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "23505" {
						return Reject("UNIQUE_ATTRIBUTE_CONFLICT", 409)
					}
				}
				return err
			}
		}
		return nil
	})
}

func writeEngineeringSourceRows(ctx context.Context, tx pgx.Tx, project, publication, actor string, rows []engineeringSourceRow) error {
	return engineeringJSONBatches(rows, engineeringSourceBatchSize, func(data []byte) error {
		_, err := tx.Exec(ctx, `INSERT INTO cmdb_field_source(id,asset_id,field_name,value,source_type,source_ref,source_location,fact_layer,revision,operator,approval_status,support,freshness,evidence_refs,project_id)
 SELECT gen_random_uuid(),x.asset_id,x.field,x.value,'bom',$2,'{}'::jsonb,'planned',x.revision,$3,'approved','supported','unknown',jsonb_build_array($4::text),$5
 FROM jsonb_to_recordset($1::jsonb) AS x(asset_id text,field text,value jsonb,revision text)
 ON CONFLICT(asset_id,field_name,source_ref,fact_layer) DO NOTHING`, data, "engineering:"+publication, actor, publication, project)
		return err
	})
}

func engineeringJSONBatches[T any](rows []T, limit int, flush func([]byte) error) error {
	data := make([]byte, 0, engineeringBatchBytes)
	data = append(data, '[')
	count := 0
	for _, row := range rows {
		encoded, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if len(encoded)+2 > engineeringBatchBytes {
			return Reject("ENGINEERING_ASSET_BATCH_ITEM_TOO_LARGE", 422)
		}
		if count > 0 && (count == limit || len(data)+len(encoded)+2 > engineeringBatchBytes) {
			if err = flush(append(data, ']')); err != nil {
				return err
			}
			data = data[:0]
			data = append(data, '[')
			count = 0
		}
		if count > 0 {
			data = append(data, ',')
		}
		data = append(data, encoded...)
		count++
	}
	if count > 0 {
		return flush(append(data, ']'))
	}
	return nil
}
