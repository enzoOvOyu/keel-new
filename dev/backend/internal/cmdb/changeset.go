package cmdb

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/identity"

	"time"
)

type Change struct {
	Op               string         `json:"op"`
	AssetID          string         `json:"asset_id"`
	AssetType        string         `json:"asset_type,omitempty"`
	ExpectedRevision int64          `json:"expected_revision"`
	Attributes       map[string]any `json:"attributes,omitempty"`
	Relationship     *RelationInput `json:"relationship,omitempty"`
	RelationshipID   string         `json:"relationship_id,omitempty"`
	ReplacementID    string         `json:"replacement_id,omitempty"`
	Before           map[string]any `json:"before,omitempty"`
}
type ChangeSetRequest struct {
	ProjectID     string   `json:"project_id"`
	Title         string   `json:"title"`
	Reason        string   `json:"reason"`
	SchemaVersion string   `json:"schema_version"`
	Changes       []Change `json:"changes"`
	BranchRef     string   `json:"branch_ref,omitempty"`
	BaseRef       string   `json:"base_ref,omitempty"`
}

func (s Service) ChangeSets(ctx context.Context, scope Scope, asset string) ([]map[string]any, error) {
	rows, e := s.DB.Query(ctx, "SELECT c.id,c.project_id,c.title,c.reason,c.status,c.version,c.schema_version,c.changes,c.requester_id,c.reviewer_id,c.review_reason,c.error_code,c.created_at,c.published_at FROM cmdb_changeset c WHERE c.project_id=ANY($1) AND ($2='' OR EXISTS(SELECT 1 FROM jsonb_array_elements(c.changes) x WHERE x->>'asset_id'=$2 OR x->>'replacement_id'=$2 OR x->'relationship'->>'source_asset_id'=$2 OR x->'relationship'->>'target_asset_id'=$2)) ORDER BY c.created_at DESC LIMIT 100", scope.Projects, asset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, project, title, reason, status, schema, requester string
		var reviewer, reviewReason, code *string
		var at time.Time
		var published *time.Time
		var version int64
		var changes []byte
		rows.Scan(&id, &project, &title, &reason, &status, &version, &schema, &changes, &requester, &reviewer, &reviewReason, &code, &at, &published)
		out = append(out, map[string]any{"id": id, "project_id": project, "title": title, "reason": reason, "status": status, "version": version, "schema_version": schema, "changes": json.RawMessage(changes), "requester_id": requester, "reviewer_id": reviewer, "review_reason": reviewReason, "error_code": code, "created_at": at, "published_at": published})
	}
	return out, rows.Err()
}
func (s Service) CreateChangeSet(ctx context.Context, scope Scope, q ChangeSetRequest, key string) (map[string]any, error) {
	if key == "" || q.Title == "" || q.Reason == "" || len(q.Changes) == 0 || len(q.Changes) > 100 {
		return nil, Reject("CHANGESET_INPUT_REQUIRED", 422)
	}
	schema, e := s.Active(ctx)
	if e != nil {
		return nil, e
	}
	if q.SchemaVersion != schema.Version {
		return nil, Reject("SCHEMA_VERSION_CONFLICT", 409)
	}
	hash := Hash(q)
	var existing, previous, previousStatus string
	var previousVersion int64
	e = s.DB.QueryRow(ctx, "SELECT id::text,request_hash,status,version FROM cmdb_changeset WHERE project_id=$1 AND idempotency_key=$2", q.ProjectID, key).Scan(&existing, &previous, &previousStatus, &previousVersion)
	if e == nil {
		if previous != hash {
			return nil, Reject("IDEMPOTENCY_KEY_REUSED", 409)
		}
		return map[string]any{"id": existing, "status": previousStatus, "version": previousVersion}, nil
	}
	if e != pgx.ErrNoRows {
		return nil, e
	}
	planned := map[string]bool{}
	for _, c := range q.Changes {
		if c.Op == "asset.create" {
			planned[c.AssetID] = true
		}
	}
	for i, c := range q.Changes {
		if c.AssetID == "" && c.Op != "relationship.upsert" {
			return nil, Reject("ASSET_ID_REQUIRED", 422)
		}
		if c.AssetID == "" && c.Relationship != nil {
			c.AssetID = c.Relationship.SourceID
			q.Changes[i].AssetID = c.AssetID
		}
		if c.Op == "relationship.upsert" && planned[c.AssetID] {
			continue
		}
		if c.Op != "asset.create" && c.AssetID != "" {
			if _, e = s.Asset(ctx, scope, c.AssetID); e != nil {
				return nil, e
			}
			var current int64
			var payload []byte
			if e = s.DB.QueryRow(ctx, "SELECT version,payload FROM cmdb_entity WHERE item_uid=$1", c.AssetID).Scan(&current, &payload); e != nil {
				return nil, e
			}
			if c.Op != "relationship.upsert" && c.Op != "relationship.end" && current != c.ExpectedRevision {
				return nil, Reject("ASSET_REVISION_CONFLICT", 409)
			}
			q.Changes[i].Before = map[string]any{}
			json.Unmarshal(payload, &q.Changes[i].Before)
		}
	}
	id := uuid.NewString()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, "INSERT INTO cmdb_changeset(id,project_id,title,reason,status,schema_version,changes,requester_id,branch_ref,base_ref,idempotency_key,request_hash) VALUES($1,$2,$3,$4,'Draft',$5,$6,$7,$8,$9,$10,$11)", id, q.ProjectID, q.Title, q.Reason, q.SchemaVersion, Encode(q.Changes), scope.UserID, q.BranchRef, q.BaseRef, key, hash)
	if e != nil {
		return nil, e
	}
	if e = s.Audit(ctx, tx, q.ProjectID, scope.Actor, "changeset.created", id, q); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return map[string]any{"id": id, "status": "Draft", "version": 1}, nil
}
func (s Service) ChangeSetAction(ctx context.Context, scope Scope, id, action, reason string, version int64, canApprove bool) (map[string]any, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	var project, status, schemaVersion, requester string
	var current int64
	var raw []byte
	e = tx.QueryRow(ctx, "SELECT project_id,status,schema_version,requester_id::text,version,changes FROM cmdb_changeset WHERE id::text=$1 FOR UPDATE", id).Scan(&project, &status, &schemaVersion, &requester, &current, &raw)
	if e != nil || !contains(scope.Projects, project) {
		return nil, Reject("CHANGESET_NOT_FOUND", 404)
	}
	if status == "Published" && action == "publish" {
		return map[string]any{"id": id, "status": status, "version": current}, nil
	}
	if current != version {
		return nil, Reject("CHANGESET_REVISION_CONFLICT", 409)
	}
	next := status
	switch action {
	case "submit":
		if status != "Draft" {
			return nil, Reject("CHANGESET_NOT_DRAFT", 409)
		}
		if requester != scope.UserID {
			return nil, Reject("CHANGESET_REQUESTER_REQUIRED", 403)
		}
		schema, e := txSchema(ctx, tx)
		if e != nil {
			return nil, e
		}
		if schema.Version != schemaVersion {
			return nil, Reject("SCHEMA_VERSION_CONFLICT", 409)
		}
		var changes []Change
		json.Unmarshal(raw, &changes)
		if _, e = tx.Exec(ctx, "SAVEPOINT cmdb_validation"); e != nil {
			return nil, e
		}
		validation := s.applyChanges(ctx, tx, schema, scope, id, project, changes)
		if _, e = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT cmdb_validation"); e != nil {
			return nil, e
		}
		if validation != nil {
			return nil, validation
		}
		next = "Review"
	case "approve", "reject":
		if !canApprove {
			return nil, Reject("CHANGESET_APPROVAL_DENIED", 403)
		}
		if requester == scope.UserID {
			return nil, Reject("SELF_APPROVAL_DENIED", 403)
		}
		if status != "Review" || reason == "" {
			return nil, Reject("CHANGESET_REVIEW_REQUIRED", 422)
		}
		next = "Approved"
		if action == "reject" {
			next = "Rejected"
		}
		if _, e = tx.Exec(ctx, "UPDATE cmdb_changeset SET reviewer_id=$2,review_reason=$3 WHERE id=$1", id, scope.UserID, reason); e != nil {
			return nil, e
		}
	case "publish":
		var reviewer string
		if e = tx.QueryRow(ctx, "SELECT COALESCE(reviewer_id::text,'') FROM cmdb_changeset WHERE id::text=$1", id).Scan(&reviewer); e != nil {
			return nil, e
		}
		if reviewer == scope.UserID {
			return nil, Reject("APPROVER_CANNOT_PUBLISH", 403)
		}
		signer, authErr := (identity.Service{DB: s.DB}).Load(ctx, reviewer)
		if authErr != nil || !signer.Can("cmdb:approve", project, "", "") {
			return nil, Reject("REVIEWER_AUTHORIZATION_REVOKED", 422)
		}
		if status != "Approved" {
			return nil, Reject("CHANGESET_APPROVAL_REQUIRED", 422)
		}
		schema, e := txSchema(ctx, tx)
		if e != nil {
			return nil, e
		}
		if schema.Version != schemaVersion {
			return nil, Reject("SCHEMA_VERSION_CONFLICT", 409)
		}
		var changes []Change
		if e = json.Unmarshal(raw, &changes); e != nil {
			return nil, e
		}
		if e = s.applyChanges(ctx, tx, schema, scope, id, project, changes); e != nil {
			tx.Rollback(ctx)
			s.DB.Exec(ctx, "UPDATE cmdb_changeset SET error_code=$2 WHERE id::text=$1", id, e.Error())
			return nil, e
		}
		next = "Published"
		if _, e = tx.Exec(ctx, "UPDATE cmdb_changeset SET published_at=now(),error_code=NULL WHERE id=$1", id); e != nil {
			return nil, e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO outbox_event(id,project_id,event_type,payload) VALUES($1,$2,'cmdb.changeset.published',$3)", uuid.NewString(), project, Encode(map[string]any{"changeset_id": id, "asset_ids": changeAssets(changes)})); e != nil {
			return nil, e
		}
	default:
		return nil, Reject("UNKNOWN_CHANGESET_ACTION", 400)
	}
	if _, e = tx.Exec(ctx, "UPDATE cmdb_changeset SET status=$2,version=version+1,updated_at=now() WHERE id=$1", id, next); e != nil {
		return nil, e
	}
	if e = s.Audit(ctx, tx, project, scope.Actor, "changeset."+action, id, map[string]any{"before": status, "after": next, "reason": reason}); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return map[string]any{"id": id, "status": next, "version": current + 1}, nil
}
func changeAssets(changes []Change) []string {
	out := []string{}
	for _, c := range changes {
		if c.AssetID != "" {
			out = append(out, c.AssetID)
		}
	}
	return out
}
func (s Service) applyChanges(ctx context.Context, tx pgx.Tx, schema Schema, scope Scope, ref, project string, changes []Change) error {
	if e := s.ProjectReference(ctx, tx, schema, project); e != nil {
		return e
	}
	touched := map[string]bool{}
	for _, c := range changes {
		source := Source{Type: "manual", Ref: ref, Layer: "formal", Operator: scope.Actor, Approval: "approved"}
		switch c.Op {
		case "asset.create":
			var exists bool
			tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM cmdb_entity WHERE item_uid=$1)", c.AssetID).Scan(&exists)
			if exists {
				return Reject("ASSET_ALREADY_EXISTS", 409)
			}
			zero := int64(0)
			if _, e := s.UpsertAsset(ctx, tx, schema, AssetInput{ID: c.AssetID, Type: c.AssetType, Project: project, Attributes: c.Attributes, ExpectedVersion: &zero, Source: source}); e != nil {
				return e
			}
			touched[c.AssetID] = true
		case "asset.update", "asset.retire", "asset.replace":
			var raw []byte
			var kind, life string
			var current int64
			e := tx.QueryRow(ctx, "SELECT e.asset_type,e.payload,e.version,e.lifecycle_state FROM cmdb_entity e JOIN cmdb_asset_scope sc ON sc.asset_id=e.item_uid WHERE e.item_uid=$1 AND sc.project_id=$2 FOR UPDATE OF e", c.AssetID, project).Scan(&kind, &raw, &current, &life)
			if e != nil {
				return Reject("ASSET_NOT_FOUND", 404)
			}
			if current != c.ExpectedRevision {
				return Reject("ASSET_REVISION_CONFLICT", 409)
			}
			if life == "retired" {
				return Reject("ASSET_RETIRED", 422)
			}
			var attrs map[string]any
			json.Unmarshal(raw, &attrs)
			if c.Op == "asset.update" {
				if issues := schema.Validate(kind, c.Attributes, "manual", true); len(issues) > 0 {
					return &Error{403, "MANUAL_ATTRIBUTE_DENIED", issues}
				}
				for key, value := range c.Attributes {
					attrs[key] = value
				}
			} else {
				attrs["lifecycle_state"] = "retired"
			}
			if _, e = s.UpsertAsset(ctx, tx, schema, AssetInput{ID: c.AssetID, Type: kind, Project: project, Attributes: attrs, ExpectedVersion: &current, Source: source}); e != nil {
				return e
			}
			touched[c.AssetID] = true
			if c.Op == "asset.replace" {
				if c.ReplacementID == "" || c.ReplacementID == c.AssetID {
					return Reject("REPLACEMENT_ID_REQUIRED", 422)
				}
				zero := int64(0)
				if _, e = s.UpsertAsset(ctx, tx, schema, AssetInput{ID: c.ReplacementID, Type: kind, Project: project, Attributes: c.Attributes, ExpectedVersion: &zero, Source: source}); e != nil {
					return e
				}
				var parent, relationID string
				var relationVersion int64
				e = tx.QueryRow(ctx, "SELECT from_uid,id::text,version FROM cmdb_relationship WHERE to_uid=$1 AND definition_id='contains' AND valid_to IS NULL ORDER BY fact_layer LIMIT 1", c.AssetID).Scan(&parent, &relationID, &relationVersion)
				if e == nil {
					if e = s.EndRelation(ctx, tx, relationID, project, ref, relationVersion); e != nil {
						return e
					}
					if _, e = s.PutRelation(ctx, tx, schema, RelationInput{Definition: "contains", SourceID: parent, TargetID: c.ReplacementID, Project: project, Source: source}); e != nil {
						return e
					}
				} else if e != pgx.ErrNoRows {
					return e
				}
				if _, e = s.PutRelation(ctx, tx, schema, RelationInput{Definition: "replaces", SourceID: c.ReplacementID, TargetID: c.AssetID, Project: project, Source: source}); e != nil {
					return e
				}
				touched[c.ReplacementID] = true
			}
		case "relationship.upsert":
			if c.Relationship == nil {
				return Reject("RELATIONSHIP_REQUIRED", 422)
			}
			r := *c.Relationship
			r.Project = project
			r.Source = source
			if _, e := s.PutRelation(ctx, tx, schema, r); e != nil {
				return e
			}
			touched[r.SourceID] = true
			touched[r.TargetID] = true
		case "relationship.end":
			var from, to string
			if e := tx.QueryRow(ctx, "SELECT from_uid,to_uid FROM cmdb_relationship WHERE id::text=$1 AND project_id=$2", c.RelationshipID, project).Scan(&from, &to); e != nil {
				return Reject("RELATIONSHIP_NOT_FOUND", 404)
			}
			touched[from] = true
			touched[to] = true
			if e := s.EndRelation(ctx, tx, c.RelationshipID, project, ref, c.ExpectedRevision); e != nil {
				return e
			}
		default:
			return Reject("UNKNOWN_CHANGE_OPERATION", 422)
		}
	}
	for asset := range touched {
		var kind string
		tx.QueryRow(ctx, "SELECT asset_type FROM cmdb_entity WHERE item_uid=$1", asset).Scan(&kind)
		if kind == "cable" {
			var incomplete bool
			if e := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM cmdb_relationship WHERE from_uid=$1 AND definition_id IN('cable_endpoint_a','cable_endpoint_b') AND valid_to IS NULL) OR EXISTS(SELECT 1 FROM cmdb_relationship WHERE from_uid=$1 AND definition_id IN('cable_endpoint_a','cable_endpoint_b') AND valid_to IS NULL GROUP BY fact_layer HAVING count(*)<>2 OR count(DISTINCT to_uid)<>2 OR count(DISTINCT definition_id)<>2)`, asset).Scan(&incomplete); e != nil {
				return e
			}
			if incomplete {
				return Reject("CABLE_BOTH_ENDPOINTS_REQUIRED", 422)
			}
		}
		if e := s.AddEvent(ctx, tx, project, "changeset:"+ref+":"+asset, asset, "asset.changed", "资产变更已发布", "published", "manual", ref, scope.Actor, map[string]any{"changeset_id": ref}, []map[string]string{{"label": "查看变更", "href": "/cmdb/history?asset=" + asset + "&changeset=" + ref}}); e != nil {
			return e
		}
	}
	return nil
}
func (s Service) ChangeSetDiff(ctx context.Context, scope Scope, id string) (map[string]any, error) {
	var project string
	var raw []byte
	e := s.DB.QueryRow(ctx, "SELECT project_id,changes FROM cmdb_changeset WHERE id::text=$1", id).Scan(&project, &raw)
	if e != nil || !contains(scope.Projects, project) {
		return nil, Reject("CHANGESET_NOT_FOUND", 404)
	}
	var changes []Change
	json.Unmarshal(raw, &changes)
	out := []map[string]any{}
	for _, c := range changes {
		var before any = c.Before
		if c.AssetID != "" && c.Before == nil {
			var payload []byte
			e = s.DB.QueryRow(ctx, "SELECT payload FROM cmdb_entity WHERE item_uid=$1", c.AssetID).Scan(&payload)
			if e == nil {
				json.Unmarshal(payload, &before)
			}
		}
		out = append(out, map[string]any{"op": c.Op, "asset_id": c.AssetID, "expected_revision": c.ExpectedRevision, "before": before, "after": c.Attributes, "relationship": c.Relationship, "replacement_id": c.ReplacementID})
	}
	return map[string]any{"id": id, "changes": out, "frozen": true}, nil
}
func (s Service) EmitDomainReference(ctx context.Context, tx pgx.Tx, project, resource, id, asset string) error {
	return s.AddEvent(ctx, tx, project, resource+":"+id+":"+asset, asset, resource, "领域记录 "+id, "recorded", resource, id, "domain-adapter", map[string]any{"source_ref": id}, []map[string]string{{"label": "查看原始记录", "href": "/cmdb/history?asset=" + asset + "&resource=" + resource + "&evidence=" + id}})
}
