package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"keel.local/dev/internal/engineering"
	"strings"
)

func engineeringRule(ctx context.Context, q engineeringQuery, project, id string, version int) (engineering.RuleView, error) {
	v := engineering.RuleView{}
	rule := &v.Rule
	rule.Inputs = []engineering.RuleIO{}
	rule.Outputs = []engineering.RuleIO{}
	rule.Dependencies = []engineering.VersionReference{}
	rule.ValidationCases = []engineering.RuleCase{}
	var raw []byte
	e := q.QueryRow(ctx, `SELECT id::text,version,domain,scope_type,scope_id,expression,approval_status,review_id::text FROM eng_rule WHERE project_id=$1 AND id::text=$2 AND version=$3`, project, id, version).Scan(&rule.ID, &rule.Version, &rule.Domain, &rule.ScopeType, &rule.ScopeID, &raw, &rule.ApprovalStatus, &rule.ReviewDecisionID)
	if e != nil {
		return v, e
	}
	if e = json.Unmarshal(raw, &rule.Expression); e != nil {
		return v, e
	}
	rows, e := q.Query(ctx, `SELECT direction,name,value_type,unit,required FROM eng_rule_io WHERE project_id=$1 AND rule_id=$2 AND version=$3 ORDER BY direction,position`, project, id, version)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		var direction string
		var spec engineering.RuleIO
		if e = rows.Scan(&direction, &spec.Name, &spec.ValueType, &spec.Unit, &spec.Required); e != nil {
			break
		}
		if direction == "input" {
			rule.Inputs = append(rule.Inputs, spec)
		} else {
			rule.Outputs = append(rule.Outputs, spec)
		}
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return v, e
	}
	rows, e = q.Query(ctx, `SELECT dependency_id::text,dependency_version FROM eng_rule_dependency WHERE project_id=$1 AND rule_id=$2 AND version=$3 ORDER BY dependency_id`, project, id, version)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		var ref engineering.VersionReference
		if e = rows.Scan(&ref.ID, &ref.Version); e != nil {
			break
		}
		rule.Dependencies = append(rule.Dependencies, ref)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return v, e
	}
	rows, e = q.Query(ctx, `SELECT id,tolerance::text FROM eng_rule_case WHERE project_id=$1 AND rule_id=$2 AND version=$3 ORDER BY id`, project, id, version)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		c := engineering.RuleCase{Inputs: []engineering.NamedValue{}, Expected: []engineering.NamedValue{}}
		if e = rows.Scan(&c.ID, &c.Tolerance); e != nil {
			break
		}
		rule.ValidationCases = append(rule.ValidationCases, c)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return v, e
	}
	rows, e = q.Query(ctx, `SELECT case_id,direction,name,value FROM eng_rule_case_value WHERE project_id=$1 AND rule_id=$2 AND version=$3 ORDER BY case_id,direction,name`, project, id, version)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		var caseID, direction string
		var n engineering.NamedValue
		if e = rows.Scan(&caseID, &direction, &n.Name, &raw); e != nil {
			break
		}
		if e = json.Unmarshal(raw, &n.Value); e != nil {
			break
		}
		for i := range rule.ValidationCases {
			c := &rule.ValidationCases[i]
			if c.ID == caseID {
				if direction == "input" {
					c.Inputs = append(c.Inputs, n)
				} else {
					c.Expected = append(c.Expected, n)
				}
			}
		}
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return v, e
	}
	content := *rule
	content.ApprovalStatus = "draft"
	content.ReviewDecisionID = nil
	v.Digest = engineering.Digest(content)
	return v, nil
}

func (s *Server) saveEngineeringRule(ctx context.Context, draft engineering.RuleDraft, actor string) (engineering.RuleView, error) {
	rule := draft.Rule
	if rule.ID == "" {
		rule.ID = uuid.NewString()
	}
	if _, e := uuid.Parse(rule.ID); e != nil {
		return engineering.RuleView{}, fmt.Errorf("ENGINEERING_SCHEMA_INVALID")
	}
	if strings.TrimSpace(draft.Reason) == "" || draft.ExpectedVersion < 0 {
		return engineering.RuleView{}, fmt.Errorf("ENGINEERING_SCHEMA_INVALID")
	}
	// Registration is project-authorized; a declared customer/domain scope never
	// grants a different project's caller access to this rule version.
	if rule.ScopeType == "project" && rule.ScopeID != s.project {
		return engineering.RuleView{}, fmt.Errorf("PROJECT_SCOPE_MISMATCH")
	}
	switch rule.ScopeType {
	case "project", "customer", "domain":
	default:
		return engineering.RuleView{}, fmt.Errorf("RULE_SCOPE_INVALID")
	}
	switch rule.Domain {
	case engineering.SourceDomain, engineering.EquipmentDomain, engineering.FacilityDomain, engineering.PowerDomain, engineering.ConnectivityDomain, engineering.MaterialDomain, engineering.NetworkDomain:
	default:
		return engineering.RuleView{}, fmt.Errorf("RULE_DOMAIN_INVALID")
	}
	rule.Version = draft.ExpectedVersion + 1
	rule.ApprovalStatus = "validated"
	rule.ReviewDecisionID = nil
	if e := engineering.ValidateRule(rule); e != nil {
		return engineering.RuleView{}, e
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return engineering.RuleView{}, e
	}
	defer tx.Rollback(ctx)
	var lock string
	if e = tx.QueryRow(ctx, `SELECT id FROM project WHERE id=$1 FOR UPDATE`, s.project).Scan(&lock); e != nil {
		return engineering.RuleView{}, e
	}
	var current int
	if e = tx.QueryRow(ctx, `SELECT COALESCE(max(version),0) FROM eng_rule WHERE project_id=$1 AND id=$2`, s.project, rule.ID).Scan(&current); e != nil {
		return engineering.RuleView{}, e
	}
	if current != draft.ExpectedVersion {
		return engineering.RuleView{}, fmt.Errorf("REVISION_CONFLICT")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO eng_rule(project_id,id,version,domain,scope_type,scope_id,expression,approval_status,actor,reason) VALUES($1,$2,$3,$4,$5,$6,$7,'validated',$8,$9)`, s.project, rule.ID, rule.Version, rule.Domain, rule.ScopeType, rule.ScopeID, jsonBytes(rule.Expression), actor, draft.Reason); e != nil {
		return engineering.RuleView{}, e
	}
	for direction, specs := range map[string][]engineering.RuleIO{"input": rule.Inputs, "output": rule.Outputs} {
		for i, v := range specs {
			if _, e = tx.Exec(ctx, `INSERT INTO eng_rule_io VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, s.project, rule.ID, rule.Version, direction, i, v.Name, v.ValueType, v.Unit, v.Required); e != nil {
				return engineering.RuleView{}, e
			}
		}
	}
	for _, ref := range rule.Dependencies {
		dep, err := engineeringRule(ctx, tx, s.project, ref.ID, ref.Version)
		if err != nil {
			return engineering.RuleView{}, err
		}
		if dep.Rule.ApprovalStatus != "approved" {
			return engineering.RuleView{}, fmt.Errorf("RULE_DEPENDENCY_APPROVAL_REQUIRED")
		}
		if _, e = tx.Exec(ctx, `INSERT INTO eng_rule_dependency VALUES($1,$2,$3,$4,$5,$6)`, s.project, rule.ID, rule.Version, ref.ID, ref.Version, dep.Digest); e != nil {
			return engineering.RuleView{}, e
		}
	}
	for _, c := range rule.ValidationCases {
		if _, e = tx.Exec(ctx, `INSERT INTO eng_rule_case VALUES($1,$2,$3,$4,$5)`, s.project, rule.ID, rule.Version, c.ID, c.Tolerance); e != nil {
			return engineering.RuleView{}, e
		}
		for direction, values := range map[string][]engineering.NamedValue{"input": c.Inputs, "expected": c.Expected} {
			for _, v := range values {
				if _, e = tx.Exec(ctx, `INSERT INTO eng_rule_case_value VALUES($1,$2,$3,$4,$5,$6,$7)`, s.project, rule.ID, rule.Version, c.ID, direction, v.Name, jsonBytes(v.Value)); e != nil {
					return engineering.RuleView{}, e
				}
			}
		}
	}
	view, e := engineeringRule(ctx, tx, s.project, rule.ID, rule.Version)
	if e != nil {
		return view, e
	}
	if e = tx.Commit(ctx); e != nil {
		return view, e
	}
	return view, nil
}

func (s *Server) reviewEngineeringRule(ctx context.Context, id string, version int, decision engineering.RuleReview, actor string) (engineering.RuleView, error) {
	if strings.TrimSpace(decision.Reason) == "" {
		return engineering.RuleView{}, fmt.Errorf("ENGINEERING_SCHEMA_INVALID")
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return engineering.RuleView{}, e
	}
	defer tx.Rollback(ctx)
	var locked string
	if e = tx.QueryRow(ctx, `SELECT id::text FROM eng_rule WHERE project_id=$1 AND id::text=$2 AND version=$3 FOR UPDATE`, s.project, id, version).Scan(&locked); e != nil {
		return engineering.RuleView{}, e
	}
	view, e := engineeringRule(ctx, tx, s.project, id, version)
	if e != nil {
		return view, e
	}
	if view.Digest != decision.ExpectedDigest {
		return view, fmt.Errorf("REVISION_CONFLICT")
	}
	next := ""
	switch decision.Decision {
	case "approve":
		if e = engineering.ValidateRule(view.Rule); e != nil {
			return view, e
		}
		next = "approved"
	case "reject":
		next = "draft"
	case "retire":
		next = "retired"
	default:
		return view, fmt.Errorf("ENGINEERING_SCHEMA_INVALID")
	}
	review := uuid.NewString()
	if _, e = tx.Exec(ctx, `INSERT INTO eng_rule_review(id,project_id,rule_id,version,digest,decision,actor,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, review, s.project, id, version, view.Digest, decision.Decision, actor, decision.Reason); e != nil {
		return view, e
	}
	if _, e = tx.Exec(ctx, `UPDATE eng_rule SET approval_status=$4,review_id=$5 WHERE project_id=$1 AND id=$2 AND version=$3`, s.project, id, version, next, review); e != nil {
		return view, e
	}
	view.Rule.ApprovalStatus = next
	view.Rule.ReviewDecisionID = &review
	return view, tx.Commit(ctx)
}
