package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"net/http"
	"strings"
)

func engineeringAttributeDefinitions(ctx context.Context, q engineeringQuery, project string) ([]engineering.AttributeDefinition, error) {
	var raw []byte
	e := q.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_strip_nulls(to_jsonb(d)-'project_id'-'created_by'-'reviewed_by'-'reason'-'created_at') ORDER BY entity_kind,key,version),'[]'::jsonb) FROM eng_attribute_definition d WHERE project_id=$1`, project).Scan(&raw)
	values := []engineering.AttributeDefinition{}
	if e == nil {
		e = json.Unmarshal(raw, &values)
	}
	return values, e
}
func (s *Server) saveAttributeDefinition(ctx context.Context, q engineering.AttributeDraft, actor string) (engineering.AttributeDefinition, error) {
	d := q.Definition
	if strings.TrimSpace(q.Reason) == "" {
		return d, errors.New("ATTRIBUTE_REASON_REQUIRED")
	}
	if e := engineering.ValidateAttributeDefinition(d); e != nil {
		return d, e
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return d, e
	}
	defer tx.Rollback(ctx)
	// Serialize definition versions in this project; no schema migration is performed by a definition change.
	var project string
	if e = tx.QueryRow(ctx, `SELECT id FROM project WHERE id=$1 FOR UPDATE`, s.project).Scan(&project); e != nil {
		return d, e
	}
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	var latest int
	if e = tx.QueryRow(ctx, `SELECT COALESCE(max(version),0) FROM eng_attribute_definition WHERE project_id=$1 AND id::text=$2`, s.project, d.ID).Scan(&latest); e != nil {
		return d, e
	}
	if latest != q.ExpectedVersion {
		return d, errors.New("REVISION_CONFLICT")
	}
	d.Version = latest + 1
	d.ApprovalStatus = "draft"
	if d.EnumValues == nil {
		d.EnumValues = []string{}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO eng_attribute_definition(project_id,id,version,entity_kind,key,value_type,unit,enum_values,required,approval_status,created_by,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft',$10,$11)`, s.project, d.ID, d.Version, d.EntityKind, d.Key, d.ValueType, d.Unit, d.EnumValues, d.Required, actor, q.Reason); e != nil {
		return d, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,'engineering.attribute.proposed',$4,$5)`, uuid.NewString(), s.project, actor, d.ID, jsonBytes(map[string]any{"version": d.Version, "reason": q.Reason})); e != nil {
		return d, e
	}
	return d, tx.Commit(ctx)
}
func (s *Server) reviewAttributeDefinition(ctx context.Context, id string, q engineering.AttributeReview, actor string) (engineering.AttributeDefinition, error) {
	if q.Version < 1 || strings.TrimSpace(q.Reason) == "" || (q.Decision != "approve" && q.Decision != "reject") {
		return engineering.AttributeDefinition{}, errors.New("ATTRIBUTE_REVIEW_INVALID")
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return engineering.AttributeDefinition{}, e
	}
	defer tx.Rollback(ctx)
	status := "approved"
	if q.Decision == "reject" {
		status = "retired"
	}
	tag, e := tx.Exec(ctx, `UPDATE eng_attribute_definition SET approval_status=$4,reviewed_by=$5,reason=$6 WHERE project_id=$1 AND id::text=$2 AND version=$3 AND approval_status='draft'`, s.project, id, q.Version, status, actor, q.Reason)
	if e != nil {
		return engineering.AttributeDefinition{}, e
	}
	if tag.RowsAffected() != 1 {
		return engineering.AttributeDefinition{}, errors.New("REVISION_CONFLICT")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,'engineering.attribute.reviewed',$4,$5)`, uuid.NewString(), s.project, actor, id, jsonBytes(q)); e != nil {
		return engineering.AttributeDefinition{}, e
	}
	definitions, e := engineeringAttributeDefinitions(ctx, tx, s.project)
	if e != nil {
		return engineering.AttributeDefinition{}, e
	}
	for _, d := range definitions {
		if d.ID == id && d.Version == q.Version {
			return d, tx.Commit(ctx)
		}
	}
	return engineering.AttributeDefinition{}, pgx.ErrNoRows
}
func saveEngineeringAttributes(ctx context.Context, tx pgx.Tx, project, revision, kind string, v engineering.EntityVersion, definitions []engineering.AttributeDefinition) error {
	seen := map[string]bool{}
	for _, value := range v.Attributes {
		if seen[value.DefinitionID] {
			return errors.New("ATTRIBUTE_DUPLICATE")
		}
		seen[value.DefinitionID] = true
		found := false
		for _, d := range definitions {
			if d.ID == value.DefinitionID && d.Version == value.DefinitionVersion {
				found = true
				if e := engineering.ValidateAttribute(value, d, kind); e != nil {
					return e
				}
			}
		}
		if !found {
			return errors.New("ATTRIBUTE_DEFINITION_UNAVAILABLE")
		}

		t := value.Value
		if _, e := tx.Exec(ctx, `INSERT INTO eng_attribute_value VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, revision, v.EntityID, project, value.DefinitionID, value.DefinitionVersion, t.Kind, t.Text, t.Decimal, t.Boolean, t.Reference, t.Unit); e != nil {
			return e
		}
	}
	return nil
}
func loadEngineeringAttributes(ctx context.Context, q engineeringQuery, revision string, m *engineering.PhysicalModel) error {
	objects := map[string]*engineering.EntityVersion{}
	for _, v := range engineering.ModelObjects(m) {
		v.Version.Attributes = []engineering.AttributeValue{}
		objects[v.Version.EntityID] = v.Version
	}
	rows, e := q.Query(ctx, `SELECT entity_id::text,definition_id::text,definition_version,value_kind,text_value,decimal_value::text,boolean_value,reference_value,unit FROM eng_attribute_value WHERE revision_id=$1 ORDER BY entity_id,definition_id`, revision)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var v engineering.AttributeValue
		if e = rows.Scan(&id, &v.DefinitionID, &v.DefinitionVersion, &v.Value.Kind, &v.Value.Text, &v.Value.Decimal, &v.Value.Boolean, &v.Value.Reference, &v.Value.Unit); e != nil {
			return e
		}
		if object := objects[id]; object != nil {
			object.Attributes = append(object.Attributes, v)
		}
	}
	return rows.Err()
}
func (s *Server) engineeringAttributeRoutes(w http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) < 5 || p[0] != "api" || p[1] != "v1" || p[2] != "projects" || p[4] != "engineering-attributes" {
		return false
	}
	permission := "bom:read"
	if r.Method != "GET" {
		permission = "bom:edit"
		if len(p) == 7 && p[6] == "review" {
			permission = "bom:review"
		}
	}
	if !subjectOf(r.Context()).Can(permission, p[3], "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	scoped := *s
	scoped.project = p[3]
	s = &scoped
	ctx := r.Context()
	switch {
	case len(p) == 5 && r.Method == "GET":
		values, e := engineeringAttributeDefinitions(ctx, s.db, s.project)
		if e != nil {
			engineeringFailure(w, e)
		} else {
			respond(w, 200, values)
		}
	case len(p) == 5 && r.Method == "POST":
		var q engineering.AttributeDraft
		if !engineeringDecode(w, r, &q) {
			return true
		}
		value, e := s.saveAttributeDefinition(ctx, q, subjectOf(ctx).Username)
		if e != nil {
			engineeringFailure(w, e)
		} else {
			respond(w, 200, value)
		}
	case len(p) == 7 && p[6] == "review" && r.Method == "POST":
		var q engineering.AttributeReview
		if !engineeringDecode(w, r, &q) {
			return true
		}
		value, e := s.reviewAttributeDefinition(ctx, p[5], q, subjectOf(ctx).Username)
		if e != nil {
			engineeringFailure(w, e)
		} else {
			respond(w, 200, value)
		}
	default:
		fail(w, 405, "METHOD_NOT_ALLOWED")
	}
	return true
}

func attributeReferenceIssues(ctx context.Context, q engineeringQuery, project string, m engineering.PhysicalModel) ([]engineering.ValidationIssue, error) {
	issues := []engineering.ValidationIssue{}
	for _, object := range engineering.ModelObjects(&m) {
		for _, v := range object.Version.Attributes {
			if v.Value.Reference == nil {
				continue
			}
			var exists bool
			if e := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_entity WHERE project_id=$1 AND id::text=$2)`, project, *v.Value.Reference).Scan(&exists); e != nil {
				return issues, e
			}
			if !exists {
				issues = append(issues, engineering.ValidationIssue{ID: engineering.StableID(project, "attribute-reference", object.Version.EntityID+v.DefinitionID), EntityID: object.Version.EntityID, Code: "ATTRIBUTE_REFERENCE_UNRESOLVED", ModuleID: "validation_release", Field: v.DefinitionID, Message: "扩展引用必须关联当前项目的明确工程对象", PublishTarget: object.Target, Severity: "BLOCK", EvidenceIDs: object.Version.EvidenceIDs})
			}
		}
	}
	return issues, nil
}
