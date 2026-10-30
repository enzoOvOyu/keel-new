package main

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
)

type identityTx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func decodeIdentity(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		fail(w, 400, "INVALID_REQUEST")
		return false
	}
	return true
}
func (s *Server) identityMutation(w http.ResponseWriter, r *http.Request, action, target string, fn func(identityTx) error) {
	tx, e := s.db.Begin(r.Context())
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	defer tx.Rollback(r.Context())
	before := identityAuditSnapshot(r.Context(), tx, target)
	if e = fn(tx); e != nil {
		fail(w, 409, "IDENTITY_STATE_CONFLICT")
		return
	}
	details, _ := json.Marshal(map[string]any{"method": r.Method, "path": r.URL.Path, "before": json.RawMessage(before), "after": json.RawMessage(identityAuditSnapshot(r.Context(), tx, target))})
	_, e = tx.Exec(r.Context(), "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,$4,$5,$6)", uuid.NewString(), "identity", subjectOf(r.Context()).Username, "identity."+action, target, details)
	if e != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "IDENTITY_MUTATION_FAILED")
		return
	}
	respond(w, 200, map[string]string{"id": target, "status": "updated"})
}

func identityAuditSnapshot(ctx context.Context, tx pgx.Tx, id string) []byte {
	var raw []byte
	e := tx.QueryRow(ctx, `SELECT jsonb_build_object('user',(SELECT jsonb_build_object('id',id,'username',username,'display_name',display_name,'status',status,'permission_version',permission_version) FROM app_user WHERE id::text=$1),'global_roles',(SELECT COALESCE(jsonb_agg(jsonb_build_object('role_id',role_id,'expires_at',expires_at,'revoked_at',revoked_at)),'[]') FROM user_global_role WHERE user_id::text=$1),'memberships',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'project_id',project_id,'role_id',role_id,'resource_type',resource_type,'resource_id',resource_id,'expires_at',expires_at,'revoked_at',revoked_at)),'[]') FROM project_membership WHERE user_id::text=$1),'active_sessions',(SELECT count(*) FROM api_session WHERE user_id::text=$1 AND revoked_at IS NULL AND expires_at>now()))`, id).Scan(&raw)
	if e != nil {
		return []byte("null")
	}
	return raw
}
