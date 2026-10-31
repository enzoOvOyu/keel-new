package flows

import (
	"context"
	"encoding/json"
	"keel.local/dev/internal/identity"
	"sort"
	"time"
)

func ApprovalDigest(r RunRequest) string {
	r.ApprovalID = ""
	r.Targets = append([]string{}, r.Targets...)
	sort.Strings(r.Targets)
	r.Source = ""
	r.Parent = ""
	return hash(string(raw(r)))
}
func NeedsApproval(r RunRequest) bool {
	if highRisk.MatchString(r.Command) {
		return true
	}
	for _, step := range r.Steps {
		if highRisk.MatchString(step.Command) {
			return true
		}
	}
	return r.ApprovalAction != ""
}
func (s *Service) CheckJobApproval(ctx context.Context, p Scope, r RunRequest) error {
	if !NeedsApproval(r) {
		return nil
	}
	if len(r.TargetRevisions) != len(r.Targets) {
		return problem(409, "APPROVAL_REQUIRES_FROZEN_TARGET_VERSIONS")
	}
	permission := map[string]string{"bios": "provision:execute", "firmware": "firmware:execute", "bmc_power": "bmc:execute", "reinstall": "provision:execute", "cleanup": "cleanup:execute"}[r.ApprovalAction]
	if permission == "" || r.ApprovalID == "" {
		return problem(422, "HIGH_RISK_APPROVAL_REQUIRED")
	}
	subject, e := (identity.Service{DB: s.DB}).Load(ctx, p.User)
	if e != nil || !subject.Can(permission, r.Project, "flow_job", r.Flow) {
		return problem(403, "APPROVED_DOMAIN_PERMISSION_REQUIRED")
	}
	var requester, action, kind, resource, version, state string
	var targets, groups []byte
	var expires time.Time
	e = s.DB.QueryRow(ctx, `SELECT requester_id::text,action,resource_type,resource_id,snapshot_version,state,targets,required_groups,expires_at FROM approval_record WHERE id::text=$1 AND project_id=$2`, r.ApprovalID, r.Project).Scan(&requester, &action, &kind, &resource, &version, &state, &targets, &groups, &expires)
	expected := r.Flow
	if expected == "" {
		expected = "quick-run"
	}
	if e != nil || requester != p.User || action != r.ApprovalAction || kind != "flow_job" || resource != expected || version != ApprovalDigest(r) || state != "approved" || time.Now().After(expires) {
		return problem(422, "APPROVAL_MISSING_EXPIRED_OR_SCOPE_CHANGED")
	}
	var required, approvedTargets []string
	json.Unmarshal(groups, &required)
	json.Unmarshal(targets, &approvedTargets)
	sort.Strings(approvedTargets)
	want := append([]string{}, r.Targets...)
	sort.Strings(want)
	if string(raw(want)) != string(raw(approvedTargets)) {
		return problem(409, "APPROVAL_TARGETS_CHANGED")
	}
	rows, e := s.DB.Query(ctx, `SELECT user_id::text,group_id FROM approval_signature WHERE approval_id=$1 AND decision='approve'`, r.ApprovalID)
	if e != nil {
		return e
	}
	type signature struct{ user, group string }
	signs := []signature{}
	for rows.Next() {
		var x signature
		rows.Scan(&x.user, &x.group)
		signs = append(signs, x)
	}
	rows.Close()
	valid := map[string]bool{}
	for _, x := range signs {
		signer, e := (identity.Service{DB: s.DB}).Load(ctx, x.user)
		role, known := identity.SignatureRoles[x.group]
		if e != nil || !known || x.user == p.User || !signer.HasRole(role, r.Project, kind, resource) {
			return problem(422, "APPROVAL_SIGNER_REVOKED_OR_SELF")
		}
		valid[x.group] = true
	}
	for _, group := range required {
		if !valid[group] {
			return problem(422, "APPROVAL_SIGNATURE_INCOMPLETE")
		}
	}
	return nil
}
