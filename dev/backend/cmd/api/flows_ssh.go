package main

import (
	"context"
	"github.com/google/uuid"
	"keel.local/dev/internal/flows"
	"net/http"
)

func (s *Server) flowSSH(w http.ResponseWriter, r *http.Request, p flows.Scope) (any, error) {
	ctx := r.Context()
	if r.Method == "GET" {
		rows, e := s.db.Query(ctx, `SELECT id::text,project_id,asset_id,host,port,username,secret_ref,known_hosts_ref,status,revision FROM flow_ssh_trust WHERE project_id=ANY($1) ORDER BY created_at DESC`, p.Projects)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, project, asset, host, user, key, known, state string
			var port, revision int
			rows.Scan(&id, &project, &asset, &host, &port, &user, &key, &known, &state, &revision)
			out = append(out, map[string]any{"id": id, "project_id": project, "asset_id": asset, "host": host, "port": port, "username": user, "secret_ref": key, "known_hosts_ref": known, "status": state, "revision": revision})
		}
		return out, rows.Err()
	}
	if r.Method != "POST" {
		fail(w, 405, "METHOD_NOT_ALLOWED")
		return nil, nil
	}
	var input struct {
		Project  string `json:"project_id"`
		Asset    string `json:"asset_id"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"username"`
		Key      string `json:"secret_ref"`
		Known    string `json:"known_hosts_ref"`
		Revision int    `json:"expected_revision"`
		Status   string `json:"status"`
	}
	if !readFlowInput(w, r, &input) {
		return nil, nil
	}
	u := subjectOf(ctx)
	if !u.Can("admin:edit", input.Project, "", "") {
		fail(w, 403, "SSH_TRUST_ADMIN_REQUIRED")
		return nil, nil
	}
	if _, e := flows.ResolveSecret(input.Key); e != nil {
		return nil, e
	}
	if _, e := flows.ResolveSecret(input.Known); e != nil {
		return nil, e
	}
	targets, e := s.flows.Targets(ctx, p, "")
	if e != nil {
		return nil, e
	}
	valid := false
	for _, t := range targets {
		if t.ID == input.Asset && t.Project == input.Project && !flows.Maintained(t) {
			for _, k := range []string{"management_ip", "mgmt_ip", "ip", "bmc_ip"} {
				if k != "bmc_ip" && t.Fields[k] == input.Host {
					valid = true
				}
			}
		}
	}
	if !valid || input.Port < 1 || input.Port > 65535 || input.User == "" || (input.Status != "approved" && input.Status != "revoked") {
		fail(w, 422, "SSH_TRUST_FACT_INVALID")
		return nil, nil
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	id := uuid.NewString()
	tag, e := tx.Exec(ctx, `INSERT INTO flow_ssh_trust(id,project_id,asset_id,host,port,username,secret_ref,known_hosts_ref,approved_by,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(project_id,asset_id) DO UPDATE SET host=EXCLUDED.host,port=EXCLUDED.port,username=EXCLUDED.username,secret_ref=EXCLUDED.secret_ref,known_hosts_ref=EXCLUDED.known_hosts_ref,approved_by=EXCLUDED.approved_by,status=EXCLUDED.status,revision=flow_ssh_trust.revision+1 WHERE flow_ssh_trust.revision=$11`, id, input.Project, input.Asset, input.Host, input.Port, input.User, input.Key, input.Known, p.User, input.Status, input.Revision)
	if e != nil {
		return nil, e
	}
	if tag.RowsAffected() != 1 {
		fail(w, 409, "SSH_TRUST_REVISION_CONFLICT")
		return nil, nil
	}
	_, e = tx.Exec(ctx, `INSERT INTO flow_audit(id,project_id,actor,action,resource_id,details) VALUES(gen_random_uuid(),$1,$2,'ssh.trust',$3,'{}')`, input.Project, p.Actor, input.Asset)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(context.WithoutCancel(ctx)); e != nil {
		return nil, e
	}
	return map[string]string{"asset_id": input.Asset, "status": input.Status}, nil
}
