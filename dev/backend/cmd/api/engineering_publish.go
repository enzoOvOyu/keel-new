package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
)

func (s *Server) publishEngineering(ctx context.Context, tx pgx.Tx, event, project string, raw []byte) error {
	var payload struct {
		PublicationID string `json:"publication_id"`
		SourceID      string `json:"source_id"`
		Target        string `json:"target"`
	}
	if e := json.Unmarshal(raw, &payload); e != nil {
		return e
	}
	var scopedSource string
	if e := tx.QueryRow(ctx, `SELECT id::text FROM eng_source WHERE id=$1 AND project_id=$2 FOR UPDATE`, payload.SourceID, project).Scan(&scopedSource); e != nil {
		return e
	}
	var revision, digest, state, actor, importID string
	e := tx.QueryRow(ctx, `SELECT p.revision_id::text,p.digest,p.state,r.actor,s.import_id::text FROM eng_publication p JOIN eng_review r ON r.id=p.review_id JOIN eng_source s ON s.id=p.source_id WHERE p.id=$1 AND p.project_id=$2 FOR UPDATE OF p`, payload.PublicationID, project).Scan(&revision, &digest, &state, &actor, &importID)
	if e != nil {
		return e
	}
	if state == "published" {
		return nil
	}
	var current bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_target WHERE source_id=$1 AND target=$2 AND revision_id=$3 AND digest=$4 AND publication_id=$5 AND state='APPROVED')`, payload.SourceID, payload.Target, revision, digest, payload.PublicationID).Scan(&current); e != nil {
		return e
	}
	if !current {
		_, e = tx.Exec(ctx, `UPDATE eng_publication SET state='failed',error_code='ENGINEERING_PUBLICATION_STALE' WHERE id=$1`, payload.PublicationID)
		return e
	}
	model, _, e := engineeringModel(ctx, tx, revision, project)
	if e != nil {
		return e
	}
	for _, dependency := range engineering.TargetDependencies(payload.Target) {
		var ready bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_target WHERE source_id=$1 AND target=$2 AND state='PUBLISHED')`, payload.SourceID, dependency).Scan(&ready); e != nil {
			return e
		}
		if !ready {
			return errors.New("ENGINEERING_DEPENDENCY_UNPUBLISHED")
		}
	}
	if payload.Target == engineering.NetworkTarget {
		for _, p := range model.NetworkPlans {
			if !networkPublicationReady(ctx, tx, project, p) {
				return errors.New("NETWORK_PHYSICAL_REVISION_STALE")
			}
		}
	}
	if payload.Target != engineering.CablingTarget && payload.Target != engineering.PowerTarget && payload.Target != engineering.MaterialTarget && payload.Target != engineering.WorkloadTarget && payload.Target != engineering.NetworkTarget {
		if _, e = tx.Exec(ctx, `INSERT INTO bom_revision(id,import_id,project_id,digest) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, payload.PublicationID, importID, project, digest); e != nil {
			return e
		}
		if e = cmdbService(s).ProjectEngineering(ctx, tx, event, project, payload.PublicationID, payload.Target, actor, model); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, `UPDATE eng_publication SET state='published',error_code=NULL,receipt=$2,published_at=now() WHERE id=$1`, payload.PublicationID, event); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `UPDATE eng_target SET state='PUBLISHED' WHERE source_id=$1 AND target=$2 AND publication_id=$3`, payload.SourceID, payload.Target, payload.PublicationID)
	return e
}
