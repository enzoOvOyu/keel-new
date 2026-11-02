package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"io"
	"keel.local/dev/internal/kickoff"
	"net/http"
	"strings"
	"time"
)

type kickoffInput = kickoff.ModelInput
type kickoffSource struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Size              string          `json:"size"`
	Version           int             `json:"version"`
	State             string          `json:"state"`
	Hash              string          `json:"hash"`
	CreatedAt         time.Time       `json:"createdAt"`
	Regions           []kickoffRegion `json:"regions"`
	OriginalAvailable bool            `json:"originalAvailable"`
}

func (s *Server) kickoffRoutes(w http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) < 6 || p[0] != "api" || p[1] != "v1" || p[2] != "projects" || p[4] != "kickoff" {
		return false
	}
	permission := "bom:read"
	if r.Method != "GET" {
		permission = "bom:edit"
		if p[5] == "sources" || p[5] == "history" {
			permission = "bom:import"
		}
		if p[5] == "publications" {
			permission = "bom:execute"
		}
	}
	if !subjectOf(r.Context()).Can(permission, p[3], "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	scoped := *s
	scoped.project = p[3]
	s = &scoped
	if _, err := s.db.Exec(r.Context(), `INSERT INTO kickoff_workspace(project_id) VALUES($1) ON CONFLICT DO NOTHING`, s.project); err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return true
	}
	switch {
	case len(p) == 6 && p[5] == "workspace" && r.Method == "GET":
		s.kickoffWorkspace(w, r)
	case len(p) == 7 && p[5] == "models" && r.Method == "PUT":
		s.kickoffSaveInput(w, r, p[6])
	case len(p) == 6 && p[5] == "sources" && r.Method == "POST":
		s.kickoffUpload(w, r)
	case len(p) == 7 && p[5] == "sources" && r.Method == "DELETE":
		s.kickoffRemoveSource(w, r, p[6])
	case len(p) == 6 && p[5] == "calculations" && r.Method == "POST":
		s.kickoffCalculate(w, r)
	case len(p) == 8 && p[5] == "issues" && p[7] == "resolve" && r.Method == "POST":
		s.kickoffResolve(w, r, p[6])
	case len(p) == 6 && p[5] == "publications" && r.Method == "POST":
		s.kickoffPublish(w, r)
	case len(p) == 7 && p[5] == "publications" && r.Method == "GET":
		s.kickoffPublication(w, r, p[6])
	case len(p) == 8 && p[5] == "sources" && p[7] == "analyze" && r.Method == "POST":
		fail(w, 503, "KICKOFF_AGENT_NOT_CONFIGURED")
	case len(p) == 6 && p[5] == "history" && r.Method == "POST":
		s.kickoffHistory(w, r)
	case len(p) == 8 && p[5] == "publications" && p[7] == "status" && r.Method == "GET":
		s.kickoffPublicationStatus(w, r, p[6])
	default:
		fail(w, 404, "KICKOFF_OPERATION_UNAVAILABLE")
	}
	return true
}
func (s *Server) kickoffWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	var revision int
	var name string
	if err := tx.QueryRow(ctx, `SELECT k.revision,p.name FROM kickoff_workspace k JOIN project p ON p.id=k.project_id WHERE k.project_id=$1`, s.project).Scan(&revision, &name); err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	inputs, err := loadKickoffInputs(ctx, tx, s.project)
	if err != nil {
		fail(w, 500, "KICKOFF_INPUT_UNAVAILABLE")
		return
	}
	model, evidence, sourceID, err := s.kickoffModel(ctx, tx)
	if err != nil {
		fail(w, 500, "KICKOFF_MODEL_UNAVAILABLE")
		return
	}
	inputs = kickoff.ResolveReferences(model, inputs)
	view := kickoff.Project(model, evidence, sourceID, inputs)
	sources := []kickoffSource{}
	rows, err := tx.Query(ctx, `SELECT id::text,filename,byte_size,state,sha256,created_at FROM kickoff_source WHERE project_id=$1 AND detached_at IS NULL ORDER BY created_at`, s.project)
	if err != nil {
		fail(w, 500, "KICKOFF_SOURCE_UNAVAILABLE")
		return
	}
	for rows.Next() {
		v := kickoffSource{Version: 1, Regions: []kickoffRegion{}, OriginalAvailable: true}
		var size int64
		if err = rows.Scan(&v.ID, &v.Name, &size, &v.State, &v.Hash, &v.CreatedAt); err != nil {
			break
		}
		v.Size = fmt.Sprintf("%.1f KB", float64(size)/1024)
		sources = append(sources, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, 500, "KICKOFF_SOURCE_UNAVAILABLE")
		return
	}
	for i := range sources {
		sources[i].Regions, err = loadKickoffRegions(ctx, tx, sources[i].ID)
		if err != nil {
			fail(w, 500, "KICKOFF_REGIONS_UNAVAILABLE")
			return
		}
	}
	calculations, err := loadKickoffCalculations(ctx, tx, s.project)
	if err != nil {
		fail(w, 500, "KICKOFF_CALCULATION_UNAVAILABLE")
		return
	}
	publications, err := s.loadKickoffPublications(ctx, tx)
	if err != nil {
		fail(w, 500, "KICKOFF_PUBLICATION_UNAVAILABLE")
		return
	}
	activity, err := loadKickoffActivity(ctx, tx, s.project)
	if err != nil {
		fail(w, 500, "KICKOFF_ACTIVITY_UNAVAILABLE")
		return
	}
	if err = tx.Commit(ctx); err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	respond(w, 200, map[string]any{"schemaVersion": 1, "projectId": s.project, "projectName": name, "revision": revision, "models": inputs, "sources": sources, "devices": view.Devices, "ports": view.Ports, "connections": view.Connections, "components": view.Components, "racks": view.Racks, "issues": view.Issues, "calculations": calculations, "publications": publications, "activity": activity})
}
func (s *Server) kickoffSaveInput(w http.ResponseWriter, r *http.Request, table string) {
	var body struct {
		Rows             []map[string]string `json:"rows"`
		Source           string              `json:"source"`
		ExpectedRevision int                 `json:"expectedRevision"`
	}
	if !engineeringDecode(w, r, &body) {
		return
	}
	if body.Rows == nil {
		fail(w, 400, "KICKOFF_ROWS_REQUIRED")
		return
	}
	if err := kickoff.ValidateInput(table, body.Rows); err != nil {
		fail(w, 422, err.Error())
		return
	}
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		fail(w, 500, "KICKOFF_INPUT_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	var revision int
	if err = tx.QueryRow(ctx, `UPDATE kickoff_workspace SET revision=revision+1 WHERE project_id=$1 AND revision=$2 RETURNING revision`, s.project, body.ExpectedRevision).Scan(&revision); errors.Is(err, pgx.ErrNoRows) {
		fail(w, 409, "REVISION_CONFLICT")
		return
	} else if err != nil {
		fail(w, 500, "KICKOFF_INPUT_UNAVAILABLE")
		return
	}
	var version int
	err = tx.QueryRow(ctx, `INSERT INTO kickoff_input_revision(project_id,table_id,version,workspace_revision,rows,source,actor) SELECT $1,$2,COALESCE(max(version),0)+1,$3,$4,$5,$6 FROM kickoff_input_revision WHERE project_id=$1 AND table_id=$2 RETURNING version`, s.project, table, revision, jsonBytes(body.Rows), body.Source, subjectOf(ctx).ID).Scan(&version)
	if err == nil {
		err = s.applyKickoffInputs(ctx, tx, subjectOf(ctx).ID, body.Source)
	}
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	if err == nil {
		err = s.kickoffAudit(ctx, tx, "kickoff.input.saved", table, map[string]any{"version": version, "rows": len(body.Rows)})
	}
	if err != nil || tx.Commit(ctx) != nil {
		fail(w, 500, "KICKOFF_INPUT_UNAVAILABLE")
		return
	}
	respond(w, 200, map[string]int{"version": version, "revision": revision})
}
func (s *Server) kickoffUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		fail(w, 400, "KICKOFF_FILE_INVALID")
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, header, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "KICKOFF_FILE_REQUIRED")
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil || len(raw) == 0 {
		fail(w, 400, "KICKOFF_FILE_INVALID")
		return
	}
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	key := s.project + "/kickoff/sources/" + digest
	if _, err = s.objects.PutObject(r.Context(), s.bucket, key, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{ContentType: header.Header.Get("Content-Type")}); err != nil {
		fail(w, 503, "OBJECT_STORAGE_UNAVAILABLE")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "KICKOFF_SOURCE_UNAVAILABLE")
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	err = tx.QueryRow(r.Context(), `INSERT INTO kickoff_source(id,project_id,filename,object_key,sha256,byte_size) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(project_id,sha256) DO UPDATE SET detached_at=NULL RETURNING id::text`, uuid.NewString(), s.project, header.Filename, key, digest, len(raw)).Scan(&id)
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE kickoff_workspace SET revision=revision+1 WHERE project_id=$1`, s.project)
	}
	if err == nil {
		err = s.applyKickoffInputs(r.Context(), tx, subjectOf(r.Context()).ID, "资料接入")
	}
	if err == nil {
		err = s.kickoffAudit(r.Context(), tx, "kickoff.source.attached", id, map[string]any{"bytes": len(raw)})
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "KICKOFF_SOURCE_UNAVAILABLE")
		return
	}
	respond(w, 201, map[string]string{"id": id})
}
func (s *Server) kickoffRemoveSource(w http.ResponseWriter, r *http.Request, id string) {
	// Source objects remain immutable and retained; deletion removes only the unused attachment.
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		fail(w, 500, "KICKOFF_SOURCE_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE kickoff_source SET detached_at=now() WHERE id::text=$1 AND project_id=$2 AND state='awaiting_analysis' AND detached_at IS NULL`, id, s.project)
	if err != nil {
		fail(w, 500, "KICKOFF_SOURCE_UNAVAILABLE")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "KICKOFF_SOURCE_IMMUTABLE")
		return
	}
	if _, err = tx.Exec(ctx, `UPDATE kickoff_workspace SET revision=revision+1 WHERE project_id=$1`, s.project); err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	if err = s.kickoffAudit(ctx, tx, "kickoff.source.detached", id, map[string]bool{"retained": true}); err != nil {
		kickoffFailure(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		fail(w, 500, "KICKOFF_WORKSPACE_UNAVAILABLE")
		return
	}
	respond(w, 200, map[string]bool{"removed": true})
}
