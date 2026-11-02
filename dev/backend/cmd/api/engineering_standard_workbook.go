package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/engineering"
)

func (s *Server) standardWorkbook(ctx context.Context, q engineering.StandardWorkbookRequest) ([]engineering.InputPreviewRequest, standardInputAttachment, error) {
	a := standardInputAttachment{ID: uuid.NewString()}
	if q.Step < 1 || q.Step > 5 || q.ImportID == "" {
		return nil, a, errors.New("STEP_INPUT_INVALID")
	}
	if e := s.db.QueryRow(ctx, `SELECT object_key,content_sha256 FROM bom_import_batch WHERE id::text=$1 AND project_id=$2`, q.ImportID, s.project).Scan(&a.ObjectKey, &a.SHA256); e != nil {
		return nil, a, e
	}
	if s.objects == nil {
		return nil, a, errors.New("STEP_SOURCE_STORAGE_UNAVAILABLE")
	}
	object, e := s.objects.GetObject(ctx, s.bucket, a.ObjectKey, minio.GetObjectOptions{})
	if e != nil {
		return nil, a, errors.New("STEP_SOURCE_STORAGE_UNAVAILABLE")
	}
	defer object.Close()
	const limit = 20 << 20
	raw, e := io.ReadAll(io.LimitReader(object, limit+1))
	if e != nil {
		return nil, a, errors.New("STEP_SOURCE_STORAGE_UNAVAILABLE")
	}
	if len(raw) > limit {
		return nil, a, errors.New("STEP_WORKBOOK_TOO_LARGE")
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != a.SHA256 {
		return nil, a, errors.New("STEP_SOURCE_INTEGRITY_FAILED")
	}
	inputs, e := engineering.ReadStepWorkbookCandidates(bytes.NewReader(raw), q.Step)
	a.TableID = "step-workbook-" + string(rune('0'+q.Step))
	return inputs, a, e
}
func workbookPreview(q engineering.StandardWorkbookRequest, inputs []engineering.InputPreviewRequest) engineering.StandardWorkbookPreview {
	out := engineering.StandardWorkbookPreview{ImportID: q.ImportID, Step: q.Step, Tables: []engineering.InputPreview{}, Valid: true, SupportsConfirmation: true}
	catalog := map[string]engineering.InputTable{}
	for _, t := range engineering.StepInputCatalog().Tables {
		catalog[t.ID] = t
	}
	for _, input := range inputs {
		v := engineering.PreviewStepInput(input)
		out.Tables = append(out.Tables, v)
		out.Valid = out.Valid && v.Valid
		out.SupportsConfirmation = out.SupportsConfirmation && (catalog[input.TableID].SupportsConfirmation || q.Step == 3)
	}
	return out
}
func (s *Server) importStandardWorkbook(ctx context.Context, w engineering.Workspace, q engineering.StandardWorkbookRequest, actor string) error {
	if w.HeadRevisionID == nil || strings.TrimSpace(q.Reason) == "" {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	if q.ExpectedRevision != w.Revision {
		return errors.New("REVISION_CONFLICT")
	}
	if w.LatestJob != nil && (w.LatestJob.State == "pending" || w.LatestJob.State == "running") {
		return errors.New("ENGINEERING_JOB_ACTIVE")
	}
	inputs, attachment, e := s.standardWorkbook(ctx, q)
	if e != nil {
		return e
	}
	preview := workbookPreview(q, inputs)
	if !preview.Valid {
		return errors.New("STEP_INPUT_INVALID")
	}
	if !preview.SupportsConfirmation {
		return errors.New("STEP_TABLE_NOT_SUPPORTED")
	}
	if e = s.prepareStandardPowerRules(ctx, inputs, actor); e != nil {
		return e
	}
	m, evidence, e := engineeringModel(ctx, s.db, *w.HeadRevisionID, s.project)
	if e != nil {
		return e
	}
	for _, input := range inputs {
		if q.Step == 3 && hasCablingInputs(inputs) {
			continue
		}
		if input.TableID == "terminations" {
			m, e = s.resolveStandardInputIdentities(ctx, m, input)
			if e != nil {
				return e
			}
		}
		m, e = engineering.MergeStandardInput(m, input)
		if e != nil {
			return e
		}
	}
	if q.Step == 3 && hasCablingInputs(inputs) {
		ids := []engineering.ID{}
		for _, v := range evidence {
			ids = append(ids, v.ID)
		}
		m, e = engineering.MergeCablingWorkbook(m, inputs, ids, s.standardCableRuleRunner(ctx, w, attachment, actor))
		if e != nil {
			return e
		}
	}
	if e = s.bindStandardNetworkContext(ctx, w.ID, &m); e != nil {
		return e
	}
	var raw []byte
	var proposal engineering.EngineeringProposal
	if e = s.db.QueryRow(ctx, `SELECT proposal FROM eng_revision WHERE id=$1`, *w.HeadRevisionID).Scan(&raw); e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &proposal); e != nil {
		return e
	}
	// Reuse the already immutable upload. Unlike temporary CSV input objects,
	// failure must never delete this original workbook.
	_, e = s.saveEngineeringDraft(ctx, w.ID, q.ExpectedRevision, "human", actor, q.Reason, engineering.Extraction{Model: m, Evidence: evidence, TotalCells: w.TotalCells, MappedCells: w.MappedCells}, proposal, attachment)
	return e
}

func hasCablingInputs(inputs []engineering.InputPreviewRequest) bool {
	for _, input := range inputs {
		if input.TableID != "port_specs" && input.TableID != "patch_paths" {
			return true
		}
	}
	return false
}
