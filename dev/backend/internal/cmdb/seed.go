package cmdb

import (
	"context"
	"encoding/json"
	"fmt"

	"time"
)

type SeedCollections map[string][]map[string]any

var AssetCollections = map[string]string{"servers": "server", "switches": "switch", "gpus": "gpu", "nics": "nic", "ports": "port", "switch_ports": "port", "optics": "optic", "cables": "cable", "racks": "rack"}
var DomainCollections = map[string]string{"executions": "execution_reference", "issues": "issue", "test_reports": "test_report", "handover_records": "cleanup_record", "firmware_inventory": "firmware_inventory", "network_baselines": "network_baseline", "topology_fabrics": "fabric", "topology_networks": "network_segment", "mapping_revisions": "mapping_revision", "bom_imports": "bom_batch"}
var EvidenceResources = []string{"servers", "switches", "gpus", "nics", "ports", "switch_ports", "optics", "cables", "racks", "rack_placements", "bom_imports", "bom_items", "bom_mapping_candidates", "bom_issues", "collection_jobs", "lifecycle_events", "delivery_stage_events", "delivery_stage_records", "test_reports", "handover_records", "executions", "issues", "alerts", "server_config_baselines", "server_observed_configs", "switch_config_baselines", "switch_running_configs", "switch_config_diffs", "server_runtime", "switch_runtime", "metric_series", "metrics", "monitoring_sources", "bmc_inventory", "bmc_sensors", "bmc_sel_events", "firmware_inventory", "mapping_pairs", "server_lldp_observations", "lldp_neighbors", "fdb_entries", "arp_entries", "switch_bgp_neighbors", "switch_interfaces", "ddm_samples", "switch_logs", "network_baselines", "push_jobs", "audit_events", "network_assertions", "execution_logs", "execution_outputs", "parameters", "cmdb_field_provenance", "cmdb_quality_findings", "lossless_baselines", "lossless_evidence", "lossless_checks", "lossless_profiles", "lossless_raw_outputs", "lossless_host_evidence", "topology_memberships", "topology_observed_networks"}

func timestamp(v any) *time.Time {
	if raw, ok := v.(string); ok {
		if t, e := time.Parse(time.RFC3339, raw); e == nil {
			return &t
		}
	}
	return nil
}
func first(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if m[k] != nil && m[k] != "" {
			return m[k]
		}
	}
	return nil
}
func explicitAssets(row map[string]any, known map[string]bool) []string {
	set := map[string]bool{}
	for _, key := range []string{"id", "asset_id", "asset_ref", "stable_ref", "device_id", "server_id", "switch_id", "target_id", "subject_id", "subject_ref", "entity_id", "local_switch_id", "local_device_id", "local_port_id", "remote_port_id", "port_id", "parent_port_id", "rack_id", "item_id", "resource", "device_ref", "bom_item_id"} {
		id := stringValue(row[key])
		if known[id] {
			set[id] = true
		}
	}
	for _, key := range []string{"target_ids", "related_asset_ids", "affected_entities", "port_ids", "device_ids"} {
		if list, ok := row[key].([]any); ok {
			for _, v := range list {
				id := stringValue(v)
				if known[id] {
					set[id] = true
				}
			}
		}
	}
	out := []string{}
	for id := range set {
		out = append(out, id)
	}
	return out
}
func seedAttrs(schema Schema, kind string, row map[string]any) map[string]any {
	attrs := map[string]any{}
	allowed := schema.Attributes(kind)
	for key, value := range row {
		if a, ok := allowed[key]; ok {
			if a.Type == "Text" && value != nil {
				if _, ok := value.(string); !ok {
					value = stringValue(value)
				}
			}
			attrs[key] = value
		}
	}
	return attrs
}
func (s Service) Seed(ctx context.Context, c SeedCollections, seedHash string, objectKeys map[string]string) (map[string]any, error) {
	var done bool
	s.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM cmdb_ingest_event WHERE event_id=$1)", "fixture:"+seedHash).Scan(&done)
	if done {
		return map[string]any{"status": "already_loaded", "content_sha256": seedHash}, nil
	}
	schema, e := s.Active(ctx)
	if e != nil {
		return nil, e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	known := map[string]bool{}
	projects := map[string]string{}
	records := map[string]map[string]any{}
	counts := map[string]int{}
	source := Source{Type: "bom", Ref: "fixture:" + seedHash, Layer: "planned", Operator: "synthetic-fixture-loader", Approval: "synthetic_confirmed", Support: "supported", Location: map[string]any{"artifact_ref": "synthetic_fixture:" + seedHash}}
	// Explicit placement facts are kept on the same original device ID.
	placements := map[string]map[string]any{}
	for _, p := range c["rack_placements"] {
		placements[stringValue(p["item_id"])] = p
	}
	for resource, kind := range AssetCollections {
		for _, record := range c[resource] {
			id, project := stringValue(record["id"]), stringValue(record["project_id"])
			if project == "" {
				return nil, Reject("FIXTURE_PROJECT_MISSING", 422)
			}
			r := map[string]any{}
			for k, v := range record {
				r[k] = v
			}
			if p := placements[id]; p != nil {
				r["rack_id"] = p["rack_id"]
				r["rack_u"] = p["u_start"]
				r["u_height"] = p["u_height"]
			}
			if kind == "server" {
				for _, stage := range c["delivery_stage_records"] {
					if stage["device_id"] == id && !contains([]string{"PASSED", "WAIVED"}, stringValue(stage["status"])) {
						r["delivery_stage"] = stringValue(stage["stage_id"]) + ":" + stringValue(stage["status"])
						break
					}
				}
			}
			attrs := seedAttrs(schema, kind, r)
			fieldSources := map[string]Source{}
			observed := source
			observed.Type = "derived"
			observed.Ref = "fixture:" + resource + ":" + id
			observed.Layer = "observed"
			observed.ObservedAt = timestamp(first(r, "observed_at", "last_seen_at"))
			observed.Freshness = stringValue(r["freshness"])
			for _, key := range []string{"health_status", "oper_state", "admin_state", "freshness", "last_seen_at", "observed_at", "temperature_c", "rx_power_dbm", "tx_power_dbm", "bmc_status", "agent_status"} {
				if attrs[key] != nil {
					fieldSources[key] = observed
				}
			}
			src := source
			src.Freshness = stringValue(r["freshness"])
			if _, e = s.UpsertAsset(ctx, tx, schema, AssetInput{ID: id, Type: kind, Project: project, Attributes: attrs, Source: src, FieldSources: fieldSources}); e != nil {
				return nil, fmt.Errorf("seed %s/%s: %w", resource, id, e)
			}
			known[id] = true
			projects[id] = project
			records[id] = r
			counts[kind]++
		}
	}
	// Location identities come from the supplied location IDs and explicit rack pod IDs.
	for _, r := range c["locations"] {
		kind := "region"
		if r["type"] == "site" {
			kind = "data_center"
		}
		id := stringValue(r["id"])
		attrs := seedAttrs(schema, kind, r)
		if _, e = s.UpsertAsset(ctx, tx, schema, AssetInput{ID: id, Type: kind, Project: "proj-b300-demo", Attributes: attrs, Source: source}); e != nil {
			return nil, e
		}
		known[id] = true
		projects[id] = "proj-b300-demo"
		records[id] = r
	}
	for _, r := range c["racks"] {
		pod := stringValue(r["pod_id"])
		if pod != "" && !known[pod] {
			if _, e = s.UpsertAsset(ctx, tx, schema, AssetInput{ID: pod, Type: "pod", Project: stringValue(r["project_id"]), Attributes: map[string]any{"name": pod}, Source: source}); e != nil {
				return nil, e
			}
			known[pod] = true
			projects[pod] = stringValue(r["project_id"])
		}
	}
	for resource, kind := range DomainCollections {
		for _, r := range c[resource] {
			id := stringValue(r["id"])
			project := stringValue(r["project_id"])
			if id == "" || project == "" || known[id] {
				continue
			}
			attrs := map[string]any{"name": stringValue(first(r, "hostname", "name", "title", "id")), "source_ref": id, "resource": resource, "status": stringValue(r["status"]), "is_demo": true}
			if kind == "network_segment" || kind == "fabric" {
				attrs = seedAttrs(schema, kind, r)
			}
			if _, e = s.UpsertAsset(ctx, tx, schema, AssetInput{ID: id, Type: kind, Project: project, Attributes: attrs, Source: source}); e != nil {
				return nil, e
			}
			known[id] = true
			projects[id] = project
		}
	}
	put := func(def, from, to, project string, layer string) error {
		if from == "" || to == "" || !known[from] || !known[to] {
			return nil
		}
		src := source
		src.Layer = layer
		_, e := s.PutRelation(ctx, tx, schema, RelationInput{Definition: def, SourceID: from, TargetID: to, Project: project, Source: src})
		return e
	}
	for id, r := range records {
		project := projects[id]
		if parent := stringValue(first(r, "component_id", "server_id", "device_id", "parent_uid")); parent != "" {
			if e = put("contains", parent, id, project, "planned"); e != nil {
				return nil, fmt.Errorf("contains %s: %w", id, e)
			}
		}
		if p := stringValue(r["parent_port_id"]); p != "" {
			if e = put("optic_on_port", p, id, project, "planned"); e != nil {
				return nil, e
			}
		}
		if r["category"] == "cable" {
			for _, k := range []string{"endpoint_a_uid", "endpoint_b_uid"} {
				def := "cable_endpoint_a"
				if k == "endpoint_b_uid" {
					def = "cable_endpoint_b"
				}
				if e = put(def, id, stringValue(r[k]), project, "planned"); e != nil {
					return nil, e
				}
			}
		}
		if rack := stringValue(r["rack_id"]); rack != "" {
			if e = put("located_in", id, rack, project, "planned"); e != nil {
				return nil, e
			}
		}
		if parent := stringValue(r["parent_id"]); parent != "" {
			if e = put("parent", parent, id, project, "planned"); e != nil {
				return nil, e
			}
		}
	}
	for _, r := range c["racks"] {
		pod := stringValue(r["pod_id"])
		if pod != "" {
			if e = put("parent", pod, stringValue(r["id"]), stringValue(r["project_id"]), "planned"); e != nil {
				return nil, e
			}
			if hall := stringValue(r["hall_id"]); hall != "" {
				if e = put("parent", hall, pod, stringValue(r["project_id"]), "planned"); e != nil {
					return nil, e
				}
			}
		}
	}
	for _, r := range c["topology_memberships"] {
		if e = put("logical_membership", stringValue(r["device_id"]), stringValue(r["network_id"]), stringValue(r["project_id"]), "planned"); e != nil {
			return nil, e
		}
	}
	for _, resource := range EvidenceResources {
		for _, r := range c[resource] {
			id, project := stringValue(r["id"]), stringValue(r["project_id"])
			assets := explicitAssets(r, known)
			if project == "" {
				for _, asset := range assets {
					project = projects[asset]
					if project != "" {
						break
					}
				}
			}
			if project == "" {
				project = "proj-b300-demo"
			}
			key := objectKeys[resource+":"+project]
			if key == "" || id == "" {
				continue
			}
			summary := map[string]any{"id": id}
			for _, field := range []string{"name", "title", "status", "stage_id", "device_id", "server_id", "switch_id", "severity", "event_type", "occurred_at", "created_at", "revision"} {
				if v := r[field]; v != nil {
					summary[field] = v
				}
			}
			if resource == "delivery_stage_records" {
				summary = r
			}
			_, e = tx.Exec(ctx, `INSERT INTO cmdb_external_reference(resource,record_id,project_id,asset_ids,object_key,content_hash,summary) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(resource,record_id,project_id) DO NOTHING`, resource, id, project, assets, key, Hash(r), Encode(summary))
			if e != nil {
				return nil, e
			}
			if nodeKind := DomainCollections[resource]; nodeKind != "" && known[id] {
				for _, asset := range assets {
					if asset != id {
						if e = put("reference", asset, id, project, "recorded"); e != nil {
							return nil, e
						}
					}
				}
			}
			layer := ""
			switch resource {
			case "server_config_baselines", "switch_config_baselines":
				layer = "baseline"
			case "server_observed_configs", "switch_running_configs":
				layer = "actual"
			case "server_runtime", "switch_runtime":
				layer = "operational"
			}
			if layer != "" {
				owner := stringValue(first(r, "server_id", "switch_id", "device_id"))
				if known[owner] {
					fields := map[string]any{}
					skip := map[string]bool{"id": true, "project_id": true, "server_id": true, "switch_id": true, "device_id": true, "source": true, "revision": true, "observed_at": true, "created_at": true, "freshness": true, "is_demo": true, "raw": true, "raw_output": true, "running_config": true}
					for field, value := range r {
						if !skip[field] {
							if value == nil {
								fields[field] = nil
							} else if len(string(Encode(value))) <= 2048 {
								fields[field] = stringValue(value)
							}
						}
					}
					_, e = tx.Exec(ctx, "INSERT INTO cmdb_config_reference VALUES($1,$2,$3,$4,$5,$6,'derived',$7,$8,'supported',$9) ON CONFLICT DO NOTHING", id, project, owner, layer, stringValue(first(r, "baseline_revision", "revision")), resource+":"+id, timestamp(first(r, "observed_at", "created_at")), defaultString(r["freshness"], "unknown"), Encode(fields))
					if e != nil {
						return nil, e
					}
				}
			}
			if contains([]string{"lifecycle_events", "delivery_stage_events", "test_reports", "handover_records", "executions", "issues", "collection_jobs"}, resource) {
				for _, asset := range assets {
					kind := resource
					label := stringValue(first(r, "summary_redacted", "summary", "title", "name", "event_type", "id"))
					at := timestamp(first(r, "occurred_at", "collected_at", "started_at", "created_at"))
					if at == nil {
						continue
					}
					links := []map[string]string{{"label": "查看原始记录", "href": "/cmdb/history?asset=" + asset + "&resource=" + resource + "&evidence=" + id}}
					if resource == "executions" {
						links = append(links, map[string]string{"label": "Execution", "href": "/flows/executions/" + id})
					}
					if resource == "issues" {
						links = append(links, map[string]string{"label": "工单", "href": "/issues/pool?issue=" + id})
					}
					_, e = tx.Exec(ctx, "INSERT INTO cmdb_lifecycle_event(id,project_id,asset_id,event_type,label,status,stage_id,source_type,source_ref,operator,occurred_at,links) VALUES($1,$2,$3,$4,$5,$6,$7,'derived',$8,'synthetic-fixture-loader',$9,$10) ON CONFLICT DO NOTHING", resource+":"+id+":"+asset, project, asset, kind, label, defaultString(r["status"], "recorded"), r["stage_id"], resource+":"+id, at, Encode(links))
					if e != nil {
						return nil, e
					}
				}
			}
		}
	}
	for _, p := range c["cmdb_field_provenance"] {
		asset := stringValue(p["asset_id"])
		if !known[asset] {
			continue
		}
		var value any
		if e = json.Unmarshal([]byte(stringValue(p["value_json"])), &value); e != nil {
			return nil, e
		}
		loc := map[string]any{"sheet": p["sheet"], "cell": p["cell"], "import_id": p["import_id"], "resource": "cmdb_field_provenance", "record_id": p["id"]}
		_, e = tx.Exec(ctx, "INSERT INTO cmdb_field_source(id,asset_id,field_name,value,source_type,source_ref,source_location,fact_layer,observed_at,revision,operator,approval_status,support,freshness,evidence_refs,project_id) VALUES(gen_random_uuid(),$1,$2,$3,'bom',$4,$5,'planned',$6,$7,'synthetic-fixture-loader','synthetic_confirmed',$8,$9,$10,$11) ON CONFLICT DO NOTHING", asset, p["field"], Encode(value), "fixture:lineage:"+stringValue(p["id"]), Encode(loc), timestamp(p["observed_at"]), p["revision"], p["support"], p["freshness"], Encode(p["evidence_refs"]), projects[asset])
		if e != nil {
			return nil, e
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO cmdb_ingest_event VALUES($1,'synthetic',$2,now())", "fixture:"+seedHash, seedHash); e != nil {
		return nil, e
	}
	if e = s.Audit(ctx, tx, "proj-b300-demo", "synthetic-fixture-loader", "fixture.loaded", seedHash, counts); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return map[string]any{"status": "loaded", "counts": counts, "content_sha256": seedHash, "simulation_only": true}, nil
}
func defaultString(v any, fallback string) string {
	if s := stringValue(v); s != "" {
		return s
	}
	return fallback
}
func (s Service) ExternalIndex(ctx context.Context, scope Scope, resource, id, asset string) ([]map[string]any, error) {
	if !contains(EvidenceResources, resource) {
		return nil, Reject("EVIDENCE_RESOURCE_DENIED", 403)
	}
	query := `SELECT r.resource,r.record_id,r.project_id,r.object_key,r.content_hash,r.summary,r.asset_ids FROM cmdb_external_reference r WHERE r.resource=$3 AND ($4='' OR r.record_id=$4) AND ($5='' OR $5=ANY(r.asset_ids)) AND (r.project_id=ANY($1::text[]) OR (cardinality(r.asset_ids)>0 AND NOT EXISTS(SELECT 1 FROM unnest(r.asset_ids) aid WHERE NOT r.project_id||':'||aid=ANY($2::text[])))) ORDER BY r.record_id LIMIT 2000`
	rows, e := s.DB.Query(ctx, query, append(scope.Args(), resource, id, asset)...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var res, id, project, key, hash string
		var summary []byte
		var assets []string
		rows.Scan(&res, &id, &project, &key, &hash, &summary, &assets)
		out = append(out, map[string]any{"resource": res, "id": id, "project_id": project, "object_key": key, "content_sha256": hash, "summary": json.RawMessage(summary), "asset_ids": assets})
	}
	return out, rows.Err()
}

// Refresh only reference indexes from the same immutable synthetic source, not asset facts.
func (s Service) RefreshFixtureReferences(ctx context.Context, c SeedCollections) (map[string]any, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	known := map[string]bool{}
	rows, e := tx.Query(ctx, "SELECT item_uid FROM cmdb_entity")
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var id string
		rows.Scan(&id)
		known[id] = true
	}
	rows.Close()
	aliases := map[string]string{}
	for _, r := range c["cmdb_entities"] {
		aliases[stringValue(r["id"])] = stringValue(r["stable_ref"])
	}
	n := 0
	for _, resource := range []string{"cmdb_quality_findings", "lifecycle_events", "rack_placements"} {
		for _, r := range c[resource] {
			indexed := map[string]any{}
			for k, v := range r {
				indexed[k] = v
			}
			if stable := aliases[stringValue(r["entity_id"])]; stable != "" {
				indexed["asset_ref"] = stable
			}
			assets := explicitAssets(indexed, known)
			project := defaultString(r["project_id"], "proj-b300-demo")
			id := stringValue(r["id"])
			summary := r
			if _, e = tx.Exec(ctx, "UPDATE cmdb_external_reference SET asset_ids=$4,summary=$5 WHERE resource=$1 AND record_id=$2 AND project_id=$3 AND content_hash=$6", resource, id, project, assets, Encode(summary), Hash(r)); e != nil {
				return nil, e
			}
			if resource == "lifecycle_events" {
				for _, asset := range assets {
					at := timestamp(first(r, "occurred_at", "created_at"))
					if at == nil {
						continue
					}
					label := stringValue(first(r, "summary_redacted", "summary", "event_type", "id"))
					links := []map[string]string{{"label": "查看原始记录", "href": "/cmdb/history?asset=" + asset + "&resource=" + resource + "&evidence=" + id}}
					_, e = tx.Exec(ctx, "INSERT INTO cmdb_lifecycle_event(id,project_id,asset_id,event_type,label,status,source_type,source_ref,operator,occurred_at,links) VALUES($1,$2,$3,$4,$5,$6,'derived',$7,'synthetic-fixture-loader',$8,$9) ON CONFLICT DO NOTHING", resource+":"+id+":"+asset, project, asset, defaultString(r["event_type"], "lifecycle_events"), label, defaultString(r["status"], "recorded"), resource+":"+id, at, Encode(links))
					if e != nil {
						return nil, e
					}
				}
			}
			n++
		}
	}
	schema, e := txSchema(ctx, tx)
	if e != nil {
		return nil, e
	}
	projectRows, e := tx.Query(ctx, "SELECT id FROM project")
	if e != nil {
		return nil, e
	}
	projectIDs := []string{}
	for projectRows.Next() {
		var id string
		if e = projectRows.Scan(&id); e != nil {
			projectRows.Close()
			return nil, e
		}
		projectIDs = append(projectIDs, id)
	}
	projectRows.Close()
	for _, project := range projectIDs {
		if e = s.ProjectReference(ctx, tx, schema, project); e != nil {
			return nil, e
		}
	}
	scoped, e := tx.Query(ctx, "SELECT asset_id,project_id FROM cmdb_asset_scope WHERE asset_id<>project_id")
	if e != nil {
		return nil, e
	}
	pairs := [][2]string{}
	for scoped.Next() {
		var asset, project string
		if e = scoped.Scan(&asset, &project); e != nil {
			scoped.Close()
			return nil, e
		}
		pairs = append(pairs, [2]string{asset, project})
	}
	scoped.Close()
	for _, pair := range pairs {
		if _, e = s.PutRelation(ctx, tx, schema, RelationInput{Definition: "project_association", SourceID: pair[0], TargetID: pair[1], Project: pair[1], Source: Source{Type: "derived", Ref: "project_scope:" + pair[1], Layer: "recorded", Operator: "project-projection"}}); e != nil {
			return nil, e
		}
	}
	configRows, e := tx.Query(ctx, "SELECT id,project_id,asset_id,layer,revision,source_ref FROM cmdb_config_reference")
	if e != nil {
		return nil, e
	}
	configs := [][6]string{}
	for configRows.Next() {
		var row [6]string
		if e = configRows.Scan(&row[0], &row[1], &row[2], &row[3], &row[4], &row[5]); e != nil {
			configRows.Close()
			return nil, e
		}
		configs = append(configs, row)
	}
	configRows.Close()
	for _, r := range configs {
		kind := "config_revision"
		if r[3] == "baseline" {
			kind = "config_baseline"
		}
		source := Source{Type: "derived", Ref: r[5], Layer: r[3], Operator: "configuration-reference-projection"}
		if _, e = s.UpsertAsset(ctx, tx, schema, AssetInput{ID: r[0], Type: kind, Project: r[1], Attributes: map[string]any{"name": r[0], "source_ref": r[5], "resource": "config_reference"}, Source: source}); e != nil {
			return nil, e
		}
		if _, e = s.PutRelation(ctx, tx, schema, RelationInput{Definition: "reference", SourceID: r[2], TargetID: r[0], Project: r[1], Source: source}); e != nil {
			return nil, e
		}
	}
	_, e = tx.Exec(ctx, "UPDATE cmdb_field_source f SET project_id=(SELECT min(project_id) FROM cmdb_asset_scope sc WHERE sc.asset_id=f.asset_id) WHERE project_id IS NULL")
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return map[string]any{"reference_indexes_updated": n, "asset_identities_changed": false}, nil
}
