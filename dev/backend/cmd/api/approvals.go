package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"keel.local/dev/internal/identity"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"
)

type approvalPolicy struct {
	Permission string
	Groups     []string
}

var approvalPolicies = map[string]approvalPolicy{"network_config": {"fabric:execute", []string{"NE", "DE"}}, "bios": {"provision:execute", []string{"DL"}}, "firmware": {"firmware:execute", []string{"DL"}}, "bmc_power": {"bmc:execute", []string{"DL"}}, "reinstall": {"provision:execute", []string{"DL"}}, "cleanup": {"cleanup:execute", []string{"DL"}}}

func (s *Server) approvalRoutes(w http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/"), "/")
	if len(p) < 3 || p[0] != "projects" || p[2] != "approvals" {
		return false
	}
	ctx := r.Context()
	actor := subjectOf(ctx)
	project := p[1]
	if !actor.Can("approval:read", project, "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	if len(p) == 3 && r.Method == "GET" {
		rows, e := s.db.Query(ctx, "SELECT id,requester_id,action,resource_type,resource_id,snapshot_version,risk,state,required_groups,targets,reason,expires_at FROM approval_record WHERE project_id=$1 ORDER BY created_at DESC LIMIT 200", project)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, requester, action, kind, res, version, risk, state, reason string
			var groups, targets []byte
			var expires time.Time
			rows.Scan(&id, &requester, &action, &kind, &res, &version, &risk, &state, &groups, &targets, &reason, &expires)
			out = append(out, map[string]any{"id": id, "requester_id": requester, "action": action, "resource_type": kind, "resource_id": res, "snapshot_version": version, "risk": risk, "state": state, "required_groups": json.RawMessage(groups), "target_ids": json.RawMessage(targets), "reason": reason, "expires_at": expires})
		}
		respond(w, 200, out)
		return true
	}
	if len(p) == 3 && r.Method == "POST" {
		var q struct {
			Action   string     `json:"action"`
			Kind     string     `json:"resource_type"`
			Resource string     `json:"resource_id"`
			Version  string     `json:"snapshot_version"`
			Risk     string     `json:"risk"`
			Targets  []string   `json:"target_ids"`
			Reason   string     `json:"reason"`
			Expires  *time.Time `json:"expires_at"`
		}
		if !decodeIdentity(w, r, &q) {
			return true
		}
		policy, ok := approvalPolicies[q.Action]
		if !ok || q.Resource == "" || q.Version == "" || q.Reason == "" || q.Kind == "" || !identity.ValidExpiry(q.Expires) {
			fail(w, 422, "INVALID_APPROVAL_REQUEST")
			return true
		}
		if !actor.Can(policy.Permission, project, q.Kind, q.Resource) {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		groups := append([]string{}, policy.Groups...)
		state := "pending"
		if q.Action == "network_config" && q.Risk == "critical" {
			groups = append(groups, "NH")
			state = "blocked_policy"
		}
		if q.Risk != "normal" && q.Risk != "critical" {
			fail(w, 422, "INVALID_RISK")
			return true
		}
		sort.Strings(q.Targets)
		g, _ := json.Marshal(groups)
		targets, _ := json.Marshal(q.Targets)
		expires := time.Now().Add(24 * time.Hour)
		if q.Expires != nil {
			expires = *q.Expires
		}
		id := uuid.NewString()
		s.identityMutation(w, r, "approval.requested", id, func(tx identityTx) error {
			_, e := tx.Exec(ctx, "INSERT INTO approval_record(id,project_id,requester_id,action,resource_type,resource_id,snapshot_version,risk,state,required_groups,targets,reason,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)", id, project, actor.ID, q.Action, q.Kind, q.Resource, q.Version, q.Risk, state, g, targets, q.Reason, expires)
			return e
		})
		return true
	}
	if len(p) != 5 || r.Method != "POST" {
		return false
	}
	var requester, action, kind, resource, version, state string
	var groupsRaw, targetsRaw []byte
	var expires time.Time
	tx, e := s.db.Begin(ctx)
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, "SELECT requester_id,action,resource_type,resource_id,snapshot_version,state,required_groups,targets,expires_at FROM approval_record WHERE id::text=$1 AND project_id=$2 FOR UPDATE", p[3], project).Scan(&requester, &action, &kind, &resource, &version, &state, &groupsRaw, &targetsRaw, &expires)
	if e != nil {
		fail(w, 404, "NOT_FOUND")
		return true
	}
	var groups []string
	json.Unmarshal(groupsRaw, &groups)
	if time.Now().After(expires) {
		fail(w, 422, "APPROVAL_EXPIRED")
		return true
	}
	if p[4] == "sign" {
		var q struct {
			Group    string `json:"group"`
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		if !decodeIdentity(w, r, &q) {
			return true
		}
		if requester == actor.ID {
			fail(w, 403, "SELF_APPROVAL_DENIED")
			return true
		}
		if state != "pending" {
			fail(w, 422, "APPROVAL_NOT_PENDING")
			return true
		}
		role, known := identity.SignatureRoles[q.Group]
		if !known {
			fail(w, 422, "NH_POLICY_UNRESOLVED")
			return true
		}
		present := false
		for _, g := range groups {
			if g == q.Group {
				present = true
			}
		}
		if !present || !actor.HasRole(role, project, kind, resource) {
			fail(w, 403, "APPROVAL_QUALIFICATION_DENIED")
			return true
		}
		if q.Reason == "" || (q.Decision != "approve" && q.Decision != "reject") {
			fail(w, 422, "DECISION_REQUIRED")
			return true
		}
		_, e = tx.Exec(ctx, "INSERT INTO approval_signature(approval_id,user_id,group_id,decision,reason) VALUES($1,$2,$3,$4,$5)", p[3], actor.ID, q.Group, q.Decision, q.Reason)
		if e != nil {
			fail(w, 409, "DUPLICATE_SIGNER_OR_GROUP")
			return true
		}
		next := "pending"
		if q.Decision == "reject" {
			next = "rejected"
		} else {
			var count int
			tx.QueryRow(ctx, "SELECT count(*) FROM approval_signature WHERE approval_id=$1 AND decision='approve'", p[3]).Scan(&count)
			if count == len(groups) {
				next = "approved"
			}
		}
		_, e = tx.Exec(ctx, "UPDATE approval_record SET state=$2 WHERE id=$1", p[3], next)
		if e == nil {
			details, _ := json.Marshal(map[string]string{"group": q.Group, "decision": q.Decision, "reason": q.Reason})
			_, e = tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,'identity.approval.signed',$4,$5)", uuid.NewString(), project, actor.Username, p[3], details)
		}
		if e != nil || tx.Commit(ctx) != nil {
			fail(w, 500, "APPROVAL_SAVE_FAILED")
			return true
		}
		respond(w, 200, map[string]string{"id": p[3], "state": next})
		return true
	}
	if p[4] == "check" {
		var q struct {
			Resource string   `json:"resource_id"`
			Version  string   `json:"snapshot_version"`
			Targets  []string `json:"target_ids"`
		}
		if !decodeIdentity(w, r, &q) {
			return true
		}
		if state == "blocked_policy" {
			fail(w, 422, "NH_POLICY_UNRESOLVED")
			return true
		}
		if !actor.Can(approvalPolicies[action].Permission, project, kind, resource) || requester != actor.ID {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		var targets []string
		json.Unmarshal(targetsRaw, &targets)
		sort.Strings(q.Targets)
		if q.Resource != resource || q.Version != version || !reflect.DeepEqual(q.Targets, targets) {
			fail(w, 409, "APPROVAL_SNAPSHOT_MISMATCH")
			return true
		}
		if state != "approved" {
			fail(w, 422, "APPROVAL_REQUIRED")
			return true
		}
		rows, e := tx.Query(ctx, "SELECT user_id,group_id FROM approval_signature WHERE approval_id=$1 AND decision='approve'", p[3])
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		type sig struct{ User, Group string }
		signs := []sig{}
		for rows.Next() {
			var x sig
			rows.Scan(&x.User, &x.Group)
			signs = append(signs, x)
		}
		rows.Close()
		for _, sign := range signs {
			p, e := identityService(s).Load(ctx, sign.User)
			if e != nil || !p.HasRole(identity.SignatureRoles[sign.Group], project, kind, resource) {
				fail(w, 422, "SIGNER_AUTHORIZATION_REVOKED")
				return true
			}
		}
		respond(w, 200, map[string]any{"allowed": true, "permission_allowed": true, "approval_satisfied": true, "device_execution": "not_performed"})
		return true
	}
	return false
}
