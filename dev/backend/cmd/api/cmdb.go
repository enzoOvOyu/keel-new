package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"io"
	"keel.local/dev/internal/cmdb"
	"keel.local/dev/internal/identity"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func cmdbService(s *Server) cmdb.Service { return cmdb.Service{DB: s.db} }
func cmdbFailure(w http.ResponseWriter, e error) {
	var domain *cmdb.Error
	if errors.As(e, &domain) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(domain.Status)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": domain.Code, "message": domain.Code, "problems": domain.Problems, "retryable": domain.Status >= 500}, "request_id": uuid.NewString()})
		return
	}
	fail(w, 500, "CMDB_DATABASE_ERROR")
}
func cmdbEnvelope(w http.ResponseWriter, result map[string]any) {
	result["request_id"] = uuid.NewString()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
func cmdbScope(u identity.Subject, project, permission string) cmdb.Scope {
	scope := cmdb.Scope{Projects: []string{}, Pairs: []string{}, UserID: u.ID, Actor: u.Username}
	for _, p := range u.Projects {
		if project != "" && p.ID != project {
			continue
		}
		if u.Can(permission, p.ID, "", "") {
			scope.Projects = append(scope.Projects, p.ID)
		} else {
			for _, g := range u.Grants {
				if g.Project == p.ID && g.ResourceType == "cmdb_asset" && g.ResourceID != "" {
					for _, v := range g.Permissions {
						if v == permission {
							scope.Pairs = append(scope.Pairs, p.ID+":"+g.ResourceID)
						}
					}
				}
			}
		}
	}
	return scope
}
func (s *Server) cmdbRoutes(w http.ResponseWriter, r *http.Request) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	if !strings.HasPrefix(path, "cmdb/") {
		return false
	}
	if path == "cmdb/published-assets" {
		return false
	}
	u := subjectOf(r.Context())
	project := r.URL.Query().Get("project_id")
	scope := cmdbScope(u, project, "cmdb:read")
	if len(scope.Projects)+len(scope.Pairs) == 0 {
		fail(w, 403, "CMDB_SCOPE_DENIED")
		return true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	service := cmdbService(s)
	if path == "cmdb/visualization" && r.Method == "GET" {
		result, err := service.Visualization(ctx, scope, r.URL.Query().Get("rack_id"), r.URL.Query().Get("focus"))
		if err != nil {
			cmdbFailure(w, err)
		} else {
			respond(w, 200, result)
		}
		return true
	}
	p := strings.Split(path, "/")
	permission := "cmdb:edit"
	if strings.HasSuffix(path, "/approve") || strings.HasSuffix(path, "/reject") {
		permission = "cmdb:approve"
	}
	if strings.HasSuffix(path, "/publish") {
		permission = "cmdb:execute"
	}
	writeScope := cmdbScope(u, project, permission)
	requireWrite := func(p string, id string) bool {
		if !u.Can(permission, p, "cmdb_asset", id) {
			fail(w, 403, "CMDB_WRITE_DENIED")
			return false
		}
		return true
	}
	if len(p) == 4 && p[1] == "relationships" && p[3] == "revisions" && r.Method == "GET" {
		result, e := service.RelationshipHistory(ctx, scope, p[2])
		if e != nil {
			cmdbFailure(w, e)
		} else {
			respond(w, 200, result)
		}
		return true
	}
	checkChanges := func(project string, changes []cmdb.Change) bool {
		for _, c := range changes {
			needed := "cmdb:edit"
			if c.Op == "asset.create" {
				needed = "cmdb:create"
			}
			if c.Op == "asset.retire" {
				needed = "cmdb:delete"
			}
			if !u.Can(needed, project, "cmdb_asset", c.AssetID) {
				fail(w, 403, "CMDB_OPERATION_DENIED")
				return false
			}
		}
		return true
	}
	if path == "cmdb/inventory" && r.Method == "GET" {
		result, e := service.InventorySnapshot(ctx, scope)
		if e != nil {
			cmdbFailure(w, e)
		} else {
			cmdbEnvelope(w, result)
		}
		return true
	}
	if path == "cmdb/schemas" && r.Method == "GET" {
		schema, e := service.Active(ctx)
		if e != nil {
			cmdbFailure(w, e)
		} else {
			respond(w, 200, schema)
		}
		return true
	}
	if path == "cmdb/schemas/versions" && r.Method == "GET" {
		rows, e := s.db.Query(ctx, "SELECT id,content_hash,status,created_by,created_at FROM cmdb_schema_version ORDER BY created_at DESC")
		if e != nil {
			cmdbFailure(w, e)
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, hash, status, actor string
			var at time.Time
			rows.Scan(&id, &hash, &status, &actor, &at)
			out = append(out, map[string]any{"version": id, "content_hash": hash, "status": status, "created_by": actor, "created_at": at})
		}
		respond(w, 200, out)
		return true
	}
	if (path == "cmdb/schemas/validate" || path == "cmdb/schemas") && r.Method == "POST" {
		if !u.Can("admin:edit", "", "", "") {
			fail(w, 403, "SCHEMA_ADMIN_REQUIRED")
			return true
		}
		var q struct {
			Schema   cmdb.Schema `json:"schema"`
			Expected string      `json:"expected_version"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			fail(w, 400, "INVALID_SCHEMA_REQUEST")
			return true
		}
		if strings.HasSuffix(path, "/validate") {
			issues, e := service.Compatibility(ctx, q.Schema)
			if e != nil {
				cmdbFailure(w, e)
			} else {
				respond(w, 200, map[string]any{"compatible": len(issues) == 0, "problems": issues})
			}
		} else {
			result, e := service.Install(ctx, q.Schema, q.Expected, u.Username)
			if e != nil {
				cmdbFailure(w, e)
			} else {
				respond(w, 200, result)
			}
		}
		return true
	}
	if len(p) == 3 && p[1] == "schemas" && r.Method == "GET" {
		var raw []byte
		if e := s.db.QueryRow(ctx, "SELECT document FROM cmdb_schema_version WHERE id=$1", p[2]).Scan(&raw); e != nil {
			fail(w, 404, "SCHEMA_VERSION_NOT_FOUND")
		} else {
			respond(w, 200, json.RawMessage(raw))
		}
		return true
	}
	if path == "cmdb/changesets" {
		if r.Method == "GET" {
			result, e := service.ChangeSets(ctx, scope, r.URL.Query().Get("asset_id"))
			if e != nil {
				cmdbFailure(w, e)
			} else {
				respond(w, 200, result)
			}
			return true
		}
		if r.Method == "POST" {
			var q cmdb.ChangeSetRequest
			r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
			if json.NewDecoder(r.Body).Decode(&q) != nil {
				fail(w, 400, "INVALID_CHANGESET")
				return true
			}
			if project != "" && q.ProjectID != project {
				fail(w, 403, "PROJECT_SCOPE_MISMATCH")
				return true
			}
			if !requireWrite(q.ProjectID, "") {
				return true
			}
			if !checkChanges(q.ProjectID, q.Changes) {
				return true
			}
			result, e := service.CreateChangeSet(ctx, cmdbScope(u, q.ProjectID, "cmdb:edit"), q, r.Header.Get("Idempotency-Key"))
			if e != nil {
				cmdbFailure(w, e)
			} else {
				respond(w, 200, result)
			}
			return true
		}
	}
	if len(p) >= 3 && p[1] == "changesets" {
		if len(p) == 4 && p[3] == "diff" && r.Method == "GET" {
			result, e := service.ChangeSetDiff(ctx, scope, p[2])
			if e != nil {
				cmdbFailure(w, e)
			} else {
				respond(w, 200, result)
			}
			return true
		}
		if len(p) == 4 && r.Method == "POST" {
			var q struct {
				Version int64  `json:"expected_revision"`
				Reason  string `json:"reason"`
			}
			if !decodeIdentity(w, r, &q) {
				return true
			}
			var actualProject string
			if s.db.QueryRow(ctx, "SELECT project_id FROM cmdb_changeset WHERE id::text=$1", p[2]).Scan(&actualProject) != nil {
				fail(w, 404, "CHANGESET_NOT_FOUND")
				return true
			}
			if project != "" && actualProject != project {
				fail(w, 403, "PROJECT_SCOPE_MISMATCH")
				return true
			}
			if !requireWrite(actualProject, "") {
				return true
			}
			if p[3] == "submit" || p[3] == "publish" {
				var raw []byte
				if e := s.db.QueryRow(ctx, "SELECT changes FROM cmdb_changeset WHERE id::text=$1", p[2]).Scan(&raw); e != nil {
					cmdbFailure(w, e)
					return true
				}
				var changes []cmdb.Change
				if e := json.Unmarshal(raw, &changes); e != nil {
					cmdbFailure(w, e)
					return true
				}
				if !checkChanges(actualProject, changes) {
					return true
				}
			}
			writeScope = cmdbScope(u, actualProject, permission)
			result, e := service.ChangeSetAction(ctx, writeScope, p[2], p[3], q.Reason, q.Version, u.Can("cmdb:approve", actualProject, "", ""))
			if e != nil {
				cmdbFailure(w, e)
			} else {
				respond(w, 200, result)
			}
			return true
		}
	}
	if path == "cmdb/dossier/catalog" && r.Method == "GET" {
		q := r.URL.Query()
		limit := 10
		if v := q.Get("limit"); v != "" {
			limit, _ = strconv.Atoi(v)
		}
		cursor := q.Get("cursor")
		if cursor == "" {
			cursor = "0"
		}
		result, e := service.Catalog(ctx, scope, cmdb.CatalogRequest{Q: q.Get("q"), Type: q.Get("type"), Status: q.Get("status"), Rack: q.Get("rack_id"), Sort: q.Get("sort"), Cursor: cursor, Limit: limit})
		if e != nil {
			cmdbFailure(w, e)
		} else {
			saved, _ := service.SavedViews(ctx, u.ID)
			result["saved_views"] = saved
			cmdbEnvelope(w, result)
		}
		return true
	}
	if path == "cmdb/dossier/tree" && r.Method == "GET" {
		result, e := service.Tree(ctx, scope)
		if e != nil {
			cmdbFailure(w, e)
		} else {
			cmdbEnvelope(w, result)
		}
		return true
	}
	if path == "cmdb/dossier/relations" && r.Method == "GET" {
		q := r.URL.Query()
		expanded := []string{}
		if q.Get("expanded_ids") != "" {
			expanded = strings.Split(q.Get("expanded_ids"), ",")
		}
		limit := 60
		if v := q.Get("limit"); v != "" {
			limit, _ = strconv.Atoi(v)
		}
		result, e := service.Relations(ctx, scope, q.Get("root_id"), expanded, q.Get("kind"), limit)
		if e != nil {
			cmdbFailure(w, e)
		} else {
			cmdbEnvelope(w, result)
		}
		return true
	}
	if path == "cmdb/dossier/quality" && r.Method == "GET" {
		result, e := service.Quality(ctx, scope, r.URL.Query().Get("asset_id"))
		if e != nil {
			cmdbFailure(w, e)
		} else {
			respond(w, 200, result)
		}
		return true
	}
	if path == "cmdb/dossier/saved-views" {
		if r.Method == "GET" {
			result, e := service.SavedViews(ctx, u.ID)
			if e != nil {
				cmdbFailure(w, e)
			} else {
				respond(w, 200, result)
			}
			return true
		}
		if r.Method == "POST" {
			var q struct {
				Name  string `json:"name"`
				Query string `json:"query"`
			}
			if !decodeIdentity(w, r, &q) {
				return true
			}
			parsed, e := url.Parse(q.Query)
			if e != nil || !strings.HasPrefix(q.Query, "/cmdb/") || parsed.Host != "" || len(q.Name) < 2 {
				fail(w, 422, "INVALID_SAVED_VIEW")
				return true
			}
			requested := parsed.Query().Get("project_id")
			if requested != "" && len(cmdbScope(u, requested, "cmdb:read").Projects) == 0 {
				fail(w, 403, "SAVED_VIEW_SCOPE_DENIED")
				return true
			}
			id := uuid.NewString()
			e = s.db.QueryRow(ctx, "INSERT INTO cmdb_saved_view(id,user_id,name,query) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,name) DO UPDATE SET query=excluded.query RETURNING id::text", id, u.ID, q.Name, q.Query).Scan(&id)
			if e != nil {
				cmdbFailure(w, e)
			} else {
				respond(w, 200, map[string]any{"id": id, "name": q.Name, "query": q.Query, "actor": u.Username, "project_id": parsed.Query().Get("project_id"), "created_at": time.Now().UTC(), "execution_mode": "real"})
			}
			return true
		}
	}
	if path == "cmdb/dossier/manual-fields" && r.Method == "PATCH" {
		var q struct {
			ProjectID string `json:"project_id"`
			AssetID   string `json:"asset_id"`
			Expected  int64  `json:"expected_revision"`
			Notes     string `json:"operator_notes"`
		}
		if !decodeIdentity(w, r, &q) {
			return true
		}
		if project != "" && project != q.ProjectID {
			fail(w, 403, "PROJECT_SCOPE_MISMATCH")
			return true
		}
		if !requireWrite(q.ProjectID, q.AssetID) {
			return true
		}
		result, e := service.Manual(ctx, cmdbScope(u, q.ProjectID, "cmdb:read"), q.AssetID, q.Expected, q.Notes)
		if e != nil {
			cmdbFailure(w, e)
		} else {
			respond(w, 200, result)
		}
		return true
	}
	if len(p) >= 5 && p[1] == "dossier" && p[2] == "evidence" && r.Method == "GET" {
		refs, e := service.ExternalIndex(ctx, scope, p[3], p[4], "")
		if e != nil {
			cmdbFailure(w, e)
			return true
		}
		if len(refs) == 0 {
			fail(w, 404, "EVIDENCE_NOT_FOUND")
			return true
		}
		records, e := s.cmdbReadReferences(ctx, refs)
		if e != nil {
			cmdbFailure(w, e)
			return true
		}
		if len(records) == 0 {
			fail(w, 404, "EVIDENCE_NOT_FOUND")
		} else {
			respond(w, 200, records[0])
		}
		return true
	}
	if len(p) >= 4 && p[1] == "dossier" && p[2] == "assets" {
		id := p[3]
		asset, e := service.Asset(ctx, scope, id)
		if e != nil {
			cmdbFailure(w, e)
			return true
		}
		if len(p) == 4 && r.Method == "GET" {
			device, e := service.Device(ctx, scope, id)
			if e != nil {
				cmdbFailure(w, e)
				return true
			}
			var attrs []byte
			s.db.QueryRow(ctx, "SELECT payload FROM cmdb_entity WHERE item_uid=$1", id).Scan(&attrs)
			cmdbEnvelope(w, map[string]any{"data": asset, "owner_device": device, "attributes": json.RawMessage(attrs)})
			return true
		}
		if len(p) == 5 && r.Method == "GET" {
			var result any
			switch p[4] {
			case "provenance":
				result, e = service.Provenance(ctx, scope, id)
			case "configuration":
				r, e2 := service.Configuration(ctx, scope, id)
				if e2 != nil {
					cmdbFailure(w, e2)
				} else {
					cmdbEnvelope(w, r)
				}
				return true
			case "lifecycle":
				r, e2 := service.Lifecycle(ctx, scope, id)
				if e2 != nil {
					cmdbFailure(w, e2)
				} else {
					cmdbEnvelope(w, r)
				}
				return true
			case "revisions":
				result, e = service.History(ctx, scope, id)
			case "diff":
				from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
				to, _ := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
				result, e = service.Diff(ctx, scope, id, from, to)
			default:
				fail(w, 404, "CMDB_RESOURCE_NOT_FOUND")
				return true
			}
			if e != nil {
				cmdbFailure(w, e)
			} else {
				respond(w, 200, result)
			}
			return true
		}
		if len(p) == 6 && p[4] == "drawer" && r.Method == "GET" {
			s.cmdbDrawer(w, r, scope, asset, p[5])
			return true
		}
	}
	fail(w, 404, "CMDB_RESOURCE_NOT_FOUND")
	return true
}
func (s *Server) cmdbReadReferences(ctx context.Context, refs []map[string]any) ([]map[string]any, error) {
	cache := map[string]map[string]map[string]any{}
	out := []map[string]any{}
	for _, ref := range refs {
		key := ref["object_key"].(string)
		if cache[key] == nil {
			object, e := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
			if e != nil {
				return nil, e
			}
			raw, e := io.ReadAll(io.LimitReader(object, 32<<20))
			object.Close()
			if e != nil {
				return nil, e
			}
			var rows []map[string]any
			if e = json.Unmarshal(raw, &rows); e != nil {
				return nil, e
			}
			cache[key] = map[string]map[string]any{}
			for _, row := range rows {
				cache[key][stringField(row, "id")] = row
			}
		}
		row := cache[key][ref["id"].(string)]
		if row != nil {
			if cmdb.Hash(row) != ref["content_sha256"] {
				return nil, cmdb.Reject("EVIDENCE_HASH_MISMATCH", 409)
			}
			out = append(out, row)
		}
	}
	return out, nil
}
func stringField(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func (s *Server) cmdbDrawer(w http.ResponseWriter, r *http.Request, scope cmdb.Scope, asset cmdb.Asset, tab string) {
	ctx := r.Context()
	device, e := cmdbService(s).Device(ctx, scope, asset.ID)
	if e != nil {
		cmdbFailure(w, e)
		return
	}
	if device == nil {
		fail(w, 422, "NOT_A_DEVICE")
		return
	}
	kind := stringField(device, "category")
	serverTabs := []string{"inventory", "mapping", "ports", "pcie", "monitoring"}
	networkTabs := []string{"identity", "configuration", "scripts", "events", "runtime", "lossless", "optics", "logs"}
	valid := serverTabs
	if kind == "switch" {
		valid = networkTabs
	}
	found := false
	for _, t := range valid {
		if t == tab {
			found = true
		}
	}
	if !found {
		fail(w, 422, "INVALID_DEVICE_TAB")
		return
	}
	resources := map[string][]string{"inventory": {"gpus", "nics", "bmc_inventory", "bmc_sensors", "firmware_inventory", "server_config_baselines", "server_observed_configs"}, "mapping": {"mapping_pairs", "gpus", "nics"}, "ports": {"ports", "switch_ports", "server_lldp_observations", "lldp_neighbors"}, "pcie": {"gpus", "nics"}, "monitoring": {"metrics", "metric_series", "alerts", "server_runtime", "bmc_sel_events"}, "identity": {}, "configuration": {"network_baselines", "push_jobs", "switch_config_baselines", "switch_running_configs", "switch_config_diffs"}, "scripts": {"network_baselines", "parameters"}, "events": {"push_jobs", "audit_events"}, "runtime": {"switch_runtime", "metric_series", "lldp_neighbors", "fdb_entries", "arp_entries", "switch_bgp_neighbors", "switch_interfaces", "monitoring_sources", "switch_ports"}, "lossless": {"switch_config_baselines", "switch_running_configs", "network_assertions", "monitoring_sources", "lossless_baselines", "lossless_evidence", "lossless_checks", "lossless_profiles", "lossless_raw_outputs", "lossless_host_evidence"}, "optics": {"optics", "switch_ports", "ddm_samples"}, "logs": {"execution_logs", "switch_logs"}}
	collections := map[string]any{}
	id := stringField(device, "id")
	for _, resource := range resources[tab] {
		refs, e := cmdbService(s).ExternalIndex(ctx, scope, resource, "", id)
		if e != nil {
			cmdbFailure(w, e)
			return
		}
		rows, e := s.cmdbReadReferences(ctx, refs)
		if e != nil {
			cmdbFailure(w, e)
			return
		}
		collections[resource] = rows
	}
	// Newly committed BOM inventory comes from canonical objects, never old mock rows.
	inventory, e := s.db.Query(ctx, `WITH RECURSIVE family(id,depth,path) AS(SELECT $3::text,0,ARRAY[$3::text] UNION ALL SELECT r.to_uid,f.depth+1,f.path||r.to_uid FROM family f JOIN cmdb_relationship r ON r.from_uid=f.id AND r.definition_id IN('contains','optic_on_port') AND r.valid_to IS NULL WHERE f.depth<4 AND NOT r.to_uid=ANY(f.path) AND (r.project_id=ANY($1) OR r.project_id||':'||r.from_uid=ANY($2))) SELECT e.item_uid,e.asset_type,e.payload FROM family f JOIN cmdb_entity e ON e.item_uid=f.id WHERE f.depth>0 AND `+cmdb.Visible("e")+` LIMIT 2000`, append(scope.Args(), id)...)
	if e != nil {
		cmdbFailure(w, e)
		return
	}
	by := map[string][]map[string]any{}
	for inventory.Next() {
		var aid, akind string
		var raw []byte
		inventory.Scan(&aid, &akind, &raw)
		record := map[string]any{}
		json.Unmarshal(raw, &record)
		record["id"] = aid
		record["project_id"] = asset.ProjectID
		if akind == "gpu" || akind == "nic" {
			record["server_id"] = id
		}
		if akind == "port" {
			record["device_id"] = id
		}
		res := map[string]string{"gpu": "gpus", "nic": "nics", "port": "ports", "optic": "optics"}[akind]
		if res != "" {
			by[res] = append(by[res], record)
		}
	}
	inventory.Close()
	for _, resource := range resources[tab] {
		if len(by[resource]) > 0 {
			collections[resource] = by[resource]
		}
	}
	fields := []map[string]any{}
	for _, key := range []string{"hostname", "serial", "model", "role", "nos_version", "rack", "rack_u", "gpu_count", "nic_count", "health_status", "bmc_status"} {
		if value, ok := device[key]; ok {
			fields = append(fields, map[string]any{"key": key, "label": key, "value": value, "unit": nil, "observation": map[string]any{"source": asset.Source, "observed_at": device["observed_at"], "freshness": asset.Freshness, "support": "supported", "quality": "unknown", "policy_allowed": false, "reason": "Stored CMDB data only; no device collection or execution"}})
		}
	}
	deviceType := "server"
	if kind == "switch" {
		deviceType = "switch"
	}
	cmdbEnvelope(w, map[string]any{"data": map[string]any{"project_id": asset.ProjectID, "device_id": id, "device_type": deviceType, "tab": tab, "fields": fields, "links": []any{}, "data_availability": "available"}, "resources": collections, "device": device})
}
