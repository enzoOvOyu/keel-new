package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
)

func (s *Server) mutate(ctx context.Context, id, action, reason, sql string, args ...any) error {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, sql, args...)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return errors.New("REVISION_CONFLICT")
	}
	actor := "local-developer"
	if p, ok := ctx.Value(principalKey{}).(principal); ok {
		actor = p.Actor
	}
	details, _ := json.Marshal(map[string]string{"reason": reason})
	_, e = tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,$4,$5,$6)", uuid.NewString(), s.project, actor, action, id, details)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func actorName(ctx context.Context) string {
	if p, ok := ctx.Value(principalKey{}).(principal); ok {
		return p.Actor
	}
	return "local-developer"
}
