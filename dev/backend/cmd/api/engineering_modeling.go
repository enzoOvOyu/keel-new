package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/engineering"
)

type modelingStored struct {
	engineering.ModelingInput
	Key, Digest string
	Actor       string
}

func (s *Server) modelingObject(ctx context.Context, key, digest string) ([]byte, error) {
	if s.objects == nil {
		return nil, errors.New("STEP_SOURCE_STORAGE_UNAVAILABLE")
	}
	o, err := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer o.Close()
	raw, err := io.ReadAll(io.LimitReader(o, (20<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 20<<20 {
		return nil, errors.New("STEP_WORKBOOK_TOO_LARGE")
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, errors.New("STEP_SOURCE_INTEGRITY_FAILED")
	}
	return raw, nil
}

func (s *Server) modelingInputs(ctx context.Context, w engineering.Workspace, withCSV bool) ([]modelingStored, error) {
	rows, err := s.db.Query(ctx, `SELECT table_id,version,base_revision,adopted_revision,object_key,content_sha256,row_count,issues,reason,actor FROM eng_modeling_input WHERE source_id=$1 ORDER BY table_id`, w.ID)
	if err != nil {
		return nil, err
	}
	out := []modelingStored{}
	for rows.Next() {
		var v modelingStored
		var adopted *int
		var issues []byte
		if err = rows.Scan(&v.TableID, &v.Version, &v.BaseRevision, &adopted, &v.Key, &v.Digest, &v.Rows, &issues, &v.Reason, &v.Actor); err != nil {
			break
		}
		if err = json.Unmarshal(issues, &v.Issues); err != nil {
			break
		}
		{
			required := map[string]bool{}
			for _, table := range engineering.StepInputCatalog().Tables {
				if table.ID == v.TableID {
					for _, f := range table.Fields {
						required[f.Code] = f.Required
					}
					break
				}
			}
			current := v.Issues[:0]
			for _, issue := range v.Issues {
				if !(v.TableID == "terminations" && issue.Field == "termination_type") && !(issue.Code == "REQUIRED" && !required[issue.Field]) {
					current = append(current, issue)
				}
			}
			v.Issues = current
		}
		v.State = "draft"
		if adopted != nil {
			v.State = "adopted"
		}
		out = append(out, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Reopen existing adopted standard inputs through their immutable source
	// records. They are version 0 until explicitly saved in the modeling editor.
	known := map[string]bool{}
	for _, v := range out {
		known[v.TableID] = true
	}
	legacy, e := s.db.Query(ctx, `SELECT i.table_id,i.object_key,i.content_sha256,r.sequence,r.reason FROM eng_standard_input_source i JOIN eng_revision r ON r.id=i.revision_id WHERE r.source_id=$1 ORDER BY r.sequence DESC,i.created_at DESC`, w.ID)
	if e != nil {
		return nil, e
	}
	type sourceInput struct {
		table, key, digest, reason string
		revision                   int
	}
	sources := []sourceInput{}
	for legacy.Next() {
		var src sourceInput
		if e = legacy.Scan(&src.table, &src.key, &src.digest, &src.revision, &src.reason); e != nil {
			break
		}
		sources = append(sources, src)
	}
	if e == nil {
		e = legacy.Err()
	}
	legacy.Close()
	if e != nil {
		return nil, e
	}
	for _, src := range sources {
		if known[src.table] {
			continue
		}
		raw, e := s.modelingObject(ctx, src.key, src.digest)
		if e != nil {
			return nil, e
		}
		parts := []engineering.InputPreviewRequest{{TableID: src.table, CSV: string(raw)}}
		if strings.HasPrefix(src.table, "step-workbook-") {
			step, _ := strconv.Atoi(strings.TrimPrefix(src.table, "step-workbook-"))
			parts, e = engineering.ReadStepWorkbookCandidates(bytes.NewReader(raw), step)
			if e != nil {
				return nil, e
			}
		}
		for _, part := range parts {
			if known[part.TableID] {
				continue
			}
			known[part.TableID] = true
			part.CSV = currentModelingCSV(part.TableID, part.CSV)
			preview := engineering.PreviewStepInput(part)
			v := modelingStored{ModelingInput: engineering.ModelingInput{TableID: part.TableID, BaseRevision: src.revision, State: "adopted", Rows: len(preview.Rows), Issues: preview.Issues, Reason: src.reason}}
			if withCSV {
				v.CSV = canonicalModelingCSV(part.TableID, preview.Rows)
			}
			out = append(out, v)
		}
	}
	if withCSV {
		for i := range out {
			if out[i].Key == "" {
				continue
			}
			raw, e := s.modelingObject(ctx, out[i].Key, out[i].Digest)
			if e != nil {
				return nil, e
			}
			out[i].CSV = currentModelingCSV(out[i].TableID, string(raw))
			preview := engineering.PreviewStepInput(engineering.InputPreviewRequest{TableID: out[i].TableID, CSV: out[i].CSV})
			out[i].Issues = preview.Issues
		}
	}
	return out, nil
}

// Project the retired column out of previously stored inputs. New uploads
// still have to match the current catalog. Immutable source objects stay intact.
func currentModelingCSV(table, raw string) string {
	if table != "terminations" {
		return raw
	}
	reader := csv.NewReader(strings.NewReader(raw))
	rows, err := reader.ReadAll()
	if err != nil || len(rows) == 0 || len(rows[0]) != 8 || rows[0][7] != "端接类型" {
		return raw
	}
	fields := engineering.StepInputCatalog().Tables[0].Fields
	for i, field := range fields {
		if rows[0][i] != field.Label {
			return raw
		}
	}
	for i := range rows {
		rows[i] = rows[i][:len(fields)]
	}
	return canonicalModelingCSV(table, rows[1:])
}

// Endpoint location references need only identities and placements, not all
// ports, connections, calculations and evidence in the full model.
func (s *Server) endpointPlacementModel(ctx context.Context, revision string) (engineering.PhysicalModel, error) {
	m := engineering.EmptyPhysical(s.project)
	for _, entry := range []struct {
		table, fields string
		dest          any
	}{
		{"eng_equipment", "'entity_id',t.entity_id,'serial',t.serial", &m.Equipment},
		{"eng_facility", "'entity_id',t.entity_id,'kind',t.kind,'code',t.code,'label',t.label,'parent_id',t.parent_id", &m.Facilities},
		{"eng_placement", "'entity_id',t.entity_id,'equipment_id',t.equipment_id,'rack_id',t.rack_id,'start_u',t.start_u,'position_code',t.position_code", &m.Placements},
	} {
		var raw []byte
		query := `SELECT COALESCE(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(` + entry.fields + `)) ORDER BY t.entity_id),'[]'::jsonb) FROM ` + entry.table + ` t JOIN eng_revision r ON r.id=t.revision_id JOIN eng_source src ON src.id=r.source_id WHERE t.revision_id=$1 AND src.project_id=$2`
		if err := s.db.QueryRow(ctx, query, revision, s.project).Scan(&raw); err != nil {
			return m, err
		}
		if err := json.Unmarshal(raw, entry.dest); err != nil {
			return m, err
		}
	}
	return m, nil
}

func canonicalModelingCSV(table string, rows [][]string) string {
	var b strings.Builder
	writer := csv.NewWriter(&b)
	for _, t := range engineering.StepInputCatalog().Tables {
		if t.ID == table {
			headers := []string{}
			for _, f := range t.Fields {
				headers = append(headers, f.Label)
			}
			_ = writer.Write(headers)
			break
		}
	}
	_ = writer.WriteAll(rows)
	writer.Flush()
	return b.String()
}

func modelingResponse(w engineering.Workspace, values []modelingStored, table string) engineering.ModelingInputs {
	out := engineering.ModelingInputs{Revision: w.Revision, Items: []engineering.ModelingInput{}}
	for _, v := range values {
		if table == "" {
			v.CSV = ""
		}
		if table == "" || table == v.TableID {
			out.Items = append(out.Items, v.ModelingInput)
		}
	}
	return out
}

func (s *Server) saveModelingInputs(ctx context.Context, w engineering.Workspace, edits []engineering.ModelingInputEdit, actor string) error {
	if s.objects == nil {
		return errors.New("STEP_SOURCE_STORAGE_UNAVAILABLE")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var revision int
	if err = tx.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1 AND project_id=$2 FOR UPDATE`, w.ID, s.project).Scan(&revision); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, q := range edits {
		if q.ExpectedRevision != revision {
			return errors.New("REVISION_CONFLICT")
		}
		if strings.TrimSpace(q.Reason) == "" || seen[q.TableID] {
			return errors.New("ENGINEERING_SCHEMA_INVALID")
		}
		seen[q.TableID] = true
		preview := engineering.PreviewStepInput(q.InputPreviewRequest)
		for _, issue := range preview.Issues {
			if issue.Code == "SENSITIVE_VALUE" || issue.Code == "COLUMN_COUNT" || issue.Code == "INVALID_CSV" || (issue.Row <= 1 && issue.Code != "EMPTY_INPUT") {
				return errors.New("STEP_INPUT_INVALID")
			}
		}
		var version int
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT version FROM eng_modeling_input WHERE source_id=$1 AND table_id=$2),0)`, w.ID, q.TableID).Scan(&version); err != nil {
			return err
		}
		if version != q.InputVersion {
			return errors.New("REVISION_CONFLICT")
		}
		q.CSV = canonicalModelingCSV(q.TableID, preview.Rows)
		sum := sha256.Sum256([]byte(q.CSV))
		digest := hex.EncodeToString(sum[:])
		key := s.project + "/engineering-inputs/" + w.ID + "/draft/" + uuid.NewString() + ".csv"
		if _, err = s.objects.PutObject(ctx, s.bucket, key, strings.NewReader(q.CSV), int64(len(q.CSV)), minio.PutObjectOptions{ContentType: "text/csv; charset=utf-8"}); err != nil {
			return err
		}
		// Old objects remain immutable evidence. Project deletion removes the prefix.
		_, err = tx.Exec(ctx, `INSERT INTO eng_modeling_input(source_id,table_id,version,base_revision,object_key,content_sha256,row_count,issues,reason,actor) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(source_id,table_id) DO UPDATE SET version=excluded.version,base_revision=excluded.base_revision,adopted_revision=NULL,object_key=excluded.object_key,content_sha256=excluded.content_sha256,row_count=excluded.row_count,issues=excluded.issues,reason=excluded.reason,actor=excluded.actor,updated_at=now()`, w.ID, q.TableID, version+1, revision, key, digest, len(preview.Rows), jsonBytes(preview.Issues), q.Reason, actor)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Server) adoptModelingInputs(ctx context.Context, w engineering.Workspace, q engineering.ModelingAdoption, actor string) error {
	if q.ExpectedRevision != w.Revision {
		return errors.New("REVISION_CONFLICT")
	}
	if q.Step < 1 || q.Step > 5 || len(q.Inputs) == 0 || strings.TrimSpace(q.Reason) == "" {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	if w.LatestJob != nil && (w.LatestJob.State == "running" || w.LatestJob.State == "pending") {
		return errors.New("ENGINEERING_JOB_ACTIVE")
	}
	values, err := s.modelingInputs(ctx, w, true)
	if err != nil {
		return err
	}
	selected := map[string]int{}
	for _, v := range q.Inputs {
		if _, ok := selected[v.TableID]; ok {
			return errors.New("STEP_TABLE_DUPLICATE")
		}
		selected[v.TableID] = v.Version
	}
	inputs := []engineering.InputPreviewRequest{}
	attachments := []standardInputAttachment{}
	// Catalog order is the existing standard workbook dependency order.
	for _, table := range engineering.StepInputCatalog().Tables {
		version, ok := selected[table.ID]
		if !ok {
			continue
		}
		if table.Step != q.Step {
			return errors.New("STEP_TABLE_INVALID")
		}
		for _, v := range values {
			if v.TableID == table.ID {
				if v.Version != version {
					return errors.New("REVISION_CONFLICT")
				}
				if v.Key == "" {
					if s.objects == nil {
						return errors.New("STEP_SOURCE_STORAGE_UNAVAILABLE")
					}
					sum := sha256.Sum256([]byte(v.CSV))
					v.Digest = hex.EncodeToString(sum[:])
					v.Key = s.project + "/engineering-inputs/" + w.ID + "/" + uuid.NewString() + ".csv"
					if _, err := s.objects.PutObject(ctx, s.bucket, v.Key, strings.NewReader(v.CSV), int64(len(v.CSV)), minio.PutObjectOptions{ContentType: "text/csv; charset=utf-8"}); err != nil {
						return err
					}
				}
				inputs = append(inputs, engineering.InputPreviewRequest{TableID: v.TableID, CSV: v.CSV})
				attachments = append(attachments, standardInputAttachment{ID: uuid.NewString(), TableID: v.TableID, ObjectKey: v.Key, SHA256: v.Digest, InputVersion: v.Version})
				delete(selected, table.ID)
			}
		}
	}
	if len(selected) > 0 {
		return errors.New("STEP_INPUT_REQUIRED")
	}
	return s.mergeStandardInputs(ctx, w, engineering.StandardWorkbookRequest{Step: q.Step, ExpectedRevision: q.ExpectedRevision, Reason: q.Reason}, inputs, attachments, actor)
}

func (s *Server) modelingRoutes(out http.ResponseWriter, r *http.Request, w engineering.Workspace, action string) bool {
	if action != "modeling-inputs" && action != "modeling-package" && action != "modeling-adopt" && action != "kickoff-handoff" {
		return false
	}
	ctx := r.Context()
	actor := subjectOf(ctx).Username
	reply := func(v any, err error) {
		if err != nil {
			engineeringFailure(out, err)
		} else {
			respond(out, 200, v)
		}
	}
	if action == "kickoff-handoff" {
		s.kickoffHandoffRoute(out, r, w)
		return true
	}
	if action == "modeling-inputs" && r.Method == "GET" {
		table := r.URL.Query().Get("table_id")
		values, err := s.modelingInputs(ctx, w, table != "")
		response := modelingResponse(w, values, table)
		if err == nil && table == "terminations" && w.HeadRevisionID != nil {
			model, e := s.endpointPlacementModel(ctx, *w.HeadRevisionID)
			if e != nil {
				reply(nil, e)
				return true
			}
			inputs := []engineering.ModelingInput{}
			for _, v := range values {
				inputs = append(inputs, v.ModelingInput)
			}
			response.PlacementReferences = engineering.ResolveEndpointPlacements(model, inputs)
		}
		reply(response, err)
		return true
	}
	if action == "modeling-inputs" && r.Method == "PUT" {
		var q engineering.ModelingInputEdit
		if !engineeringDecode(out, r, &q) {
			return true
		}
		if err := s.saveModelingInputs(ctx, w, []engineering.ModelingInputEdit{q}, actor); err != nil {
			reply(nil, err)
			return true
		}
		values, err := s.modelingInputs(ctx, w, true)
		if err != nil {
			reply(nil, err)
			return true
		}
		for _, v := range values {
			if v.TableID == q.TableID {
				reply(v.ModelingInput, nil)
				return true
			}
		}
	}
	if action == "modeling-adopt" && r.Method == "POST" {
		// Large source-backed plans can outlive the default HTTP write deadline.
		// Keep the bounded synchronous adoption response alive through commit.
		if err := http.NewResponseController(out).SetWriteDeadline(time.Now().Add(5 * time.Minute)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			reply(nil, err)
			return true
		}
		var q engineering.ModelingAdoption
		if !engineeringDecode(out, r, &q) {
			return true
		}
		if err := s.adoptModelingInputs(ctx, w, q, actor); err != nil {
			reply(nil, err)
		} else {
			v, e := s.engineeringWorkspace(ctx, w.ID)
			reply(v, e)
		}
		return true
	}
	if action == "modeling-package" && r.Method == "GET" {
		step, _ := strconv.Atoi(r.URL.Query().Get("step"))
		values, err := s.modelingInputs(ctx, w, true)
		if err != nil {
			reply(nil, err)
			return true
		}
		tables := []engineering.StandardTableData{}
		basis := engineering.ModelingPackageBasis{SourceID: w.ID, Revision: w.Revision, Inputs: []engineering.ModelingInputVersion{}}
		for _, t := range engineering.StepInputCatalog().Tables {
			if t.Step != step {
				continue
			}
			data := engineering.StandardTableData{TableID: t.ID, Rows: [][]string{}}
			version := 0
			for _, v := range values {
				if v.TableID == t.ID {
					data.Rows = engineering.PreviewStepInput(engineering.InputPreviewRequest{TableID: t.ID, CSV: v.CSV}).Rows
					version = v.Version
				}
			}
			tables = append(tables, data)
			basis.Inputs = append(basis.Inputs, engineering.ModelingInputVersion{TableID: t.ID, Version: version})
		}
		raw, err := engineering.WriteModelingPackage(step, tables, basis)
		if err != nil {
			reply(nil, err)
			return true
		}
		out.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		out.Header().Set("Content-Disposition", `attachment; filename="kickoff-step-`+strconv.Itoa(step)+`.xlsx"`)
		out.Write(raw)
		return true
	}
	if action == "modeling-package" && r.Method == "POST" {
		var q engineering.StandardWorkbookRequest
		if !engineeringDecode(out, r, &q) {
			return true
		}
		if q.ExpectedRevision != w.Revision {
			reply(nil, errors.New("REVISION_CONFLICT"))
			return true
		}
		var key, digest string
		err := s.db.QueryRow(ctx, `SELECT object_key,content_sha256 FROM bom_import_batch WHERE id::text=$1 AND project_id=$2`, q.ImportID, s.project).Scan(&key, &digest)
		if err != nil {
			reply(nil, err)
			return true
		}
		raw, err := s.modelingObject(ctx, key, digest)
		if err != nil {
			reply(nil, err)
			return true
		}
		inputs, basis, err := engineering.ReadModelingPackage(bytes.NewReader(raw), q.Step)
		if err != nil {
			reply(nil, err)
			return true
		}
		values, err := s.modelingInputs(ctx, w, false)
		if err != nil {
			reply(nil, err)
			return true
		}
		versions := map[string]int{}
		for _, v := range values {
			versions[v.TableID] = v.Version
		}
		if basis != nil {
			if basis.SourceID != w.ID || basis.Revision != w.Revision {
				reply(nil, errors.New("REVISION_CONFLICT"))
				return true
			}
			for _, v := range basis.Inputs {
				if versions[v.TableID] != v.Version {
					reply(nil, errors.New("REVISION_CONFLICT"))
					return true
				}
			}
		}
		edits := []engineering.ModelingInputEdit{}
		for _, input := range inputs {
			if len(engineering.PreviewStepInput(input).Rows) == 0 {
				continue
			}
			edits = append(edits, engineering.ModelingInputEdit{ExpectedRevision: q.ExpectedRevision, InputVersion: versions[input.TableID], InputPreviewRequest: input, Reason: q.Reason})
		}
		if len(edits) == 0 {
			reply(nil, errors.New("STEP_EMPTY_WORKBOOK"))
			return true
		}
		if err = s.saveModelingInputs(ctx, w, edits, actor); err != nil {
			reply(nil, err)
			return true
		}
		values, err = s.modelingInputs(ctx, w, false)
		reply(modelingResponse(w, values, ""), err)
		return true
	}
	fail(out, 405, "METHOD_NOT_ALLOWED")
	return true
}
