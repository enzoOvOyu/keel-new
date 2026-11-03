package main

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
)

const kickoffAgentKind = "kickoff-domain-agent/v1"

// This metadata uses the existing durable eng_job row, not another queue.
// ExpectedRevision on eng_job is the ENGINEERING version. WorkspaceRevision and
// BaseVersions are separate snapshots; HTTP expectedRevision is always Kickoff.
type kickoffAgentRequest struct {
	Kind                   string         `json:"kind"`
	SourceID               string         `json:"sourceId"`
	Domains                []string       `json:"domains"`
	WorkspaceRevision      int            `json:"workspaceRevision"`
	BaseVersions           map[string]int `json:"baseVersions"`
	Actor                  string         `json:"actor"`
	AdoptionSourceRevision int            `json:"adoptionSourceRevision,omitempty"`
	AdoptedTableIDs        []string       `json:"adoptedTableIds,omitempty"`
}
type kickoffAgentAnalysisResponse struct {
	Adoption          *kickoffAdoptionStatus     `json:"adoption,omitempty"`
	CoverageTotal     int                        `json:"coverageTotal"`
	MissingTotal      int                        `json:"missingTotal"`
	CoverageTruncated bool                       `json:"coverageTruncated"`
	MissingTruncated  bool                       `json:"missingTruncated"`
	JobID             *string                    `json:"jobId"`
	State             string                     `json:"state"`
	ErrorCode         *string                    `json:"errorCode,omitempty"`
	Domains           []string                   `json:"domains"`
	Coverage          []ai.KickoffCoverage       `json:"coverage"`
	Missing           []string                   `json:"missing"`
	Drafts            []kickoffAgentDraftPreview `json:"drafts"`
}
type kickoffAgentDraftPreview struct {
	ai.KickoffAgentCandidate
	RowCount     int `json:"rowCount"`
	PreviewLimit int `json:"previewLimit"`
}

func kickoffAgentPermission(w http.ResponseWriter, r *http.Request, project, permission string) bool {
	if !subjectOf(r.Context()).Can(permission, project, "", "") {
		fail(w, 403, "FORBIDDEN")
		return false
	}
	return true
}
func kickoffAgentDomains(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{"equipment", "facility", "cabling", "network", "power", "provisioning"}, nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		if ai.KickoffAgentStep(v) == 0 {
			return nil, errors.New("KICKOFF_AGENT_DOMAINS_INVALID")
		}
		if !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	return out, nil
}
func (s *Server) kickoffAgentAnalyze(w http.ResponseWriter, r *http.Request, source string) {
	if !kickoffAgentPermission(w, r, s.project, "bom:import") {
		return
	}
	var body struct {
		ExpectedRevision int      `json:"expectedRevision"`
		Domains          []string `json:"domains"`
	}
	if !engineeringDecode(w, r, &body) {
		return
	}
	domains, err := kickoffAgentDomains(body.Domains)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if s.objects == nil {
		fail(w, 503, "ANALYSIS_STORAGE_UNAVAILABLE")
		return
	}
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if err = kickoffLock(ctx, tx, s.project, body.ExpectedRevision); err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	original, version, err := s.kickoffAgentSource(ctx, tx, source, subjectOf(ctx).ID)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	var active bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_job WHERE source_id=$1 AND state IN('pending','running'))`, original).Scan(&active); err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if active {
		fail(w, 409, "ENGINEERING_JOB_ACTIVE")
		return
	}
	inputs, err := loadKickoffInputs(ctx, tx, s.project)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	request := kickoffAgentRequest{Kind: kickoffAgentKind, SourceID: source, Domains: domains, WorkspaceRevision: body.ExpectedRevision, BaseVersions: map[string]int{}, Actor: subjectOf(ctx).ID}
	for _, input := range inputs {
		request.BaseVersions[input.TableID] = input.Version
	}
	job := uuid.NewString()
	if _, err = tx.Exec(ctx, `INSERT INTO eng_job(id,source_id,project_id,expected_revision,state,proposal) VALUES($1,$2,$3,$4,'pending',$5)`, job, original, s.project, version, jsonBytes(request)); err == nil {
		err = s.kickoffAudit(ctx, tx, "kickoff.analysis.queued", source, map[string]any{"jobId": job, "domains": domains, "sourceRevision": version, "workspaceRevision": body.ExpectedRevision})
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	respond(w, 202, map[string]string{"jobId": job, "state": "pending"})
}

// Resolve an existing same-project source by immutable hash/history before
// creating an anchor. Reusing history never copies identities or reimports it.
func (s *Server) kickoffAgentSource(ctx context.Context, tx pgx.Tx, source, actor string) (string, int, error) {
	var key, name, digest string
	err := tx.QueryRow(ctx, `SELECT object_key,filename,sha256 FROM kickoff_source WHERE id::text=$1 AND project_id=$2 AND detached_at IS NULL FOR UPDATE`, source, s.project).Scan(&key, &name, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, errors.New("KICKOFF_SOURCE_NOT_FOUND")
	}
	if err != nil {
		return "", 0, err
	}
	var original string
	var revision int
	err = tx.QueryRow(ctx, `SELECT e.id::text,e.revision FROM eng_source e JOIN bom_import_batch b ON b.id=e.import_id WHERE e.project_id=$1 AND b.project_id=$1 AND b.content_sha256=$2 ORDER BY (e.id::text=$3) DESC,e.created_at LIMIT 1 FOR UPDATE OF e`, s.project, digest, source).Scan(&original, &revision)
	if err == nil {
		return original, revision, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", 0, err
	}
	// Uploaded sources already use a shared engineering anchor in most cases.
	// The additional anchor does not become the current workspace model here.
	var importID string
	err = tx.QueryRow(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,import_mode) VALUES($1,$2,$1,$3,$4,$5,'uploaded','engineering') ON CONFLICT(object_key) DO UPDATE SET object_key=excluded.object_key WHERE bom_import_batch.project_id=excluded.project_id RETURNING id::text`, source, s.project, key, name, digest).Scan(&importID)
	if err != nil {
		return "", 0, err
	}
	original = source
	if err = tx.QueryRow(ctx, `INSERT INTO eng_source(id,project_id,import_id) VALUES($1,$2,$3) ON CONFLICT(import_id) DO UPDATE SET import_id=excluded.import_id WHERE eng_source.project_id=excluded.project_id RETURNING id::text,revision`, original, s.project, importID).Scan(&original, &revision); err != nil {
		return "", 0, err
	}
	var head *string
	if err = tx.QueryRow(ctx, `SELECT head_revision_id::text FROM eng_source WHERE id=$1`, original).Scan(&head); err != nil {
		return "", 0, err
	}
	if head == nil {
		id := uuid.NewString()
		proposal := engineering.EngineeringProposal{Summary: "Kickoff immutable source anchor", Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
		if _, err = tx.Exec(ctx, `INSERT INTO eng_revision(id,source_id,sequence,actor,origin,reason,total_cells,mapped_cells,proposal) VALUES($1,$2,1,$3,'agent','immutable source anchor',0,0,$4)`, id, original, actor, jsonBytes(proposal)); err != nil {
			return "", 0, err
		}
		if err = saveEngineeringIntelligence(ctx, tx, s.project, original, id, 1, proposal, engineering.Extraction{Model: engineering.EmptyPhysical(s.project)}, nil); err != nil {
			return "", 0, err
		}
		if _, err = tx.Exec(ctx, `UPDATE eng_source SET head_revision_id=$2 WHERE id=$1`, original, id); err != nil {
			return "", 0, err
		}
	}
	return original, revision, nil
}
func (s *Server) kickoffAgentJob(ctx context.Context, source, requested string) (string, string, *string, kickoffAgentRequest, error) {
	var job, state string
	var code *string
	var raw []byte
	var request kickoffAgentRequest
	err := s.db.QueryRow(ctx, `SELECT j.id::text,j.state,j.error_code,j.proposal FROM eng_job j JOIN kickoff_source k ON k.id::text=j.proposal->>'sourceId' AND k.project_id=j.project_id WHERE j.project_id=$1 AND k.id::text=$2 AND k.detached_at IS NULL AND j.proposal->>'kind'=$3 AND ($4='' OR j.id::text=$4) ORDER BY j.created_at DESC,j.id DESC LIMIT 1`, s.project, source, kickoffAgentKind, requested).Scan(&job, &state, &code, &raw)
	if err == nil {
		err = json.Unmarshal(raw, &request)
	}
	return job, state, code, request, err
}
func (s *Server) kickoffAgentAnalysis(w http.ResponseWriter, r *http.Request, source string) {
	if !kickoffAgentPermission(w, r, s.project, "bom:read") {
		return
	}
	out := kickoffAgentAnalysisResponse{State: "idle", Domains: []string{}, Coverage: []ai.KickoffCoverage{}, Missing: []string{}, Drafts: []kickoffAgentDraftPreview{}}
	job, state, code, request, err := s.kickoffAgentJob(r.Context(), source, r.URL.Query().Get("jobId"))
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err = s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM kickoff_source WHERE id::text=$1 AND project_id=$2 AND detached_at IS NULL)`, source, s.project).Scan(&exists); err != nil {
			kickoffAgentFailure(w, err)
			return
		}
		if !exists {
			fail(w, 404, "KICKOFF_SOURCE_NOT_FOUND")
			return
		}
		if r.URL.Query().Get("jobId") != "" {
			fail(w, 404, "KICKOFF_ANALYSIS_NOT_FOUND")
			return
		}
		respond(w, 200, out)
		return
	}
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	out.Adoption, err = s.adoptionStatus(r.Context(), source)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	out.JobID = &job
	out.State = state
	out.ErrorCode = code
	out.Domains = request.Domains
	var result ai.KickoffResult
	err = s.kickoffAgentArtifact(r.Context(), job, "kickoff_domain_result", &result)
	if errors.Is(err, pgx.ErrNoRows) && state != "succeeded" {
		respond(w, 200, out)
		return
	}
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	out.Coverage = result.Coverage
	out.Missing = result.Missing
	out.CoverageTotal = len(result.Coverage)
	out.MissingTotal = len(result.Missing)
	out.CoverageTruncated = len(result.Coverage) > 200
	out.MissingTruncated = len(result.Missing) > 200
	// Bound every potentially large public collection. Full data stays in MinIO.
	if len(out.Coverage) > 200 {
		out.Coverage = out.Coverage[:200]
	}
	if len(out.Missing) > 200 {
		out.Missing = out.Missing[:200]
	}
	adopted := map[string]bool{}
	for _, id := range request.AdoptedTableIDs {
		adopted[id] = true
	}
	for _, candidate := range result.Candidates {
		if adopted[candidate.TableID] {
			continue
		}
		count := len(candidate.Rows)
		candidate.Rows = candidate.Rows[:min(count, 20)]
		refs := []ai.KickoffAgentEvidence{}
		for _, e := range candidate.Evidence {
			if e.Row > 0 && e.Row <= 20 && len(refs) < 100 {
				refs = append(refs, e)
			}
		}
		candidate.Evidence = refs
		if len(candidate.Issues) > 20 {
			candidate.Issues = candidate.Issues[:20]
		}
		out.Drafts = append(out.Drafts, kickoffAgentDraftPreview{KickoffAgentCandidate: candidate, RowCount: count, PreviewLimit: 20})
	}
	respond(w, 200, out)
}

// Reads the same hashed/gzipped artifact format as the existing checkpoint path.
func (s *Server) kickoffAgentArtifact(ctx context.Context, job, kind string, value any) error {
	if s.objects == nil {
		return errors.New("ANALYSIS_STORAGE_UNAVAILABLE")
	}
	var key, digest string
	err := s.db.QueryRow(ctx, `SELECT a.object_key,a.content_sha256 FROM eng_analysis_artifact a JOIN eng_job j ON j.id=a.job_id WHERE j.id::text=$1 AND j.project_id=$2 AND a.kind=$3 AND a.state='succeeded' ORDER BY a.created_at DESC,a.id DESC LIMIT 1`, job, s.project, kind).Scan(&key, &digest)
	if err != nil {
		return err
	}
	obj, err := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return errors.New("ANALYSIS_STORAGE_UNAVAILABLE")
	}
	defer obj.Close()
	hash := sha256.New()
	reader := io.TeeReader(obj, hash)
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return errors.New("ANALYSIS_STORAGE_UNAVAILABLE")
	}
	defer gz.Close()
	if err = json.NewDecoder(gz).Decode(value); err != nil {
		return err
	}
	if _, err = io.Copy(io.Discard, gz); err != nil {
		return err
	}
	if _, err = io.Copy(io.Discard, reader); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return errors.New("ANALYSIS_ARTIFACT_HASH_MISMATCH")
	}
	return nil
}

func (s *Server) runKickoffAgentJob(ctx context.Context, job, claim, source string, expected int, trace func(string, string, string) error) (bool, error) {
	var raw []byte
	if err := s.db.QueryRow(ctx, `SELECT proposal FROM eng_job WHERE id=$1 AND project_id=$2 AND claim_id=$3 AND state='running'`, job, s.project, claim).Scan(&raw); err != nil {
		return true, err
	}
	var request kickoffAgentRequest
	if len(raw) == 0 || string(raw) == "null" {
		return false, nil
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return true, err
	}
	if request.Kind == kickoffAdoptionKind {
		return true, s.runKickoffAdoption(ctx, job, claim, raw)
	}
	if request.Kind != kickoffAgentKind {
		return false, nil
	}
	wb, importID, digest, err := s.kickoffAgentWorkbook(ctx, source, expected)
	if err != nil {
		return true, err
	}
	if err = s.saveAnalysisArtifact(ctx, job, claim, "kickoff_domain_structure", "succeeded", kickoffAgentWorkbookStructure(wb)); err != nil {
		return true, err
	}
	provider, _, err := s.workbenchProvider(ctx, importID)
	if err != nil {
		return true, err
	}
	hooks := s.kickoffToolHooks(ctx, source, expected, wb)
	hooks.SourceHash = digest
	hooks.Resume, err = s.kickoffAgentCheckpoint(ctx, job, source, expected, request.Domains)
	if err != nil {
		return true, err
	}
	hooks.Artifact = func(ctx context.Context, tool, state string, value any) error {
		return s.saveAnalysisArtifact(ctx, job, claim, tool, state, value)
	}
	hooks.Checkpoint = func(ctx context.Context, cp ai.EngineeringCheckpoint) error {
		return s.saveAnalysisArtifact(ctx, job, claim, "checkpoint", "succeeded", cp)
	}
	if err = trace("kickoff_domains_selected", "succeeded", strings.Join(request.Domains, ",")); err != nil {
		return true, err
	}
	result, err := provider.ExtractKickoff(ctx, engineeringSafeWorkbook(wb), wb, trace, hooks, ai.KickoffAgentOptions{Domains: request.Domains})
	if err != nil {
		return true, err
	}
	result.Candidates = kickoffAgentProjectEndpoints(kickoffAgentCombine(result.Candidates))
	for i := range result.Candidates {
		result.Candidates[i].BaseVersion = request.BaseVersions[result.Candidates[i].TableID]
	}
	report := kickoffAgentUnderstandingReport(request.SourceID, job, wb, result)
	if err = s.saveAnalysisArtifact(ctx, job, claim, "kickoff_domain_understanding", "succeeded", report); err != nil {
		return true, err
	}
	// No staging write or model revision is made by the worker.
	if err = s.saveAnalysisArtifact(ctx, job, claim, "kickoff_domain_result", "succeeded", result); err != nil {
		return true, err
	}
	// Source status is attachment metadata only; it is not confirmation or release.
	_, err = s.db.Exec(ctx, `UPDATE kickoff_source SET state='needs_input' WHERE id::text=$1 AND project_id=$2 AND detached_at IS NULL AND EXISTS(SELECT 1 FROM eng_job WHERE id=$3 AND claim_id=$4 AND state='running')`, request.SourceID, s.project, job, claim)
	return true, err
}
func (s *Server) kickoffAgentWorkbook(ctx context.Context, source string, expected int) (bom.Workbook, string, string, error) {
	var raw []byte
	var importID, key, digest string
	var revision int
	err := s.db.QueryRow(ctx, `SELECT b.id::text,b.workbook,b.object_key,b.content_sha256,e.revision FROM eng_source e JOIN bom_import_batch b ON b.id=e.import_id WHERE e.id=$1 AND e.project_id=$2 AND b.project_id=$2`, source, s.project).Scan(&importID, &raw, &key, &digest, &revision)
	if err != nil {
		return bom.Workbook{}, "", "", err
	}
	if revision != expected {
		return bom.Workbook{}, "", "", errors.New("REVISION_CONFLICT")
	}
	var wb bom.Workbook
	if err = json.Unmarshal(raw, &wb); err == nil && len(wb.Sheets) > 0 && wb.Sheets[0].Parser == "excelize-ir/v4" {
		return wb, importID, digest, nil
	}
	if s.objects == nil {
		return wb, "", "", errors.New("ANALYSIS_STORAGE_UNAVAILABLE")
	}
	obj, err := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return wb, "", "", errors.New("ANALYSIS_STORAGE_UNAVAILABLE")
	}
	defer obj.Close()
	data, err := io.ReadAll(io.LimitReader(obj, (64<<20)+1))
	if err != nil {
		return wb, "", "", err
	}
	if len(data) > 64<<20 {
		return wb, "", "", errors.New("KICKOFF_SOURCE_TOO_LARGE")
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != digest {
		return wb, "", "", errors.New("KICKOFF_SOURCE_HASH_MISMATCH")
	}
	wb, err = bom.Parse(data)
	if err != nil {
		return wb, "", "", err
	}
	_, err = s.db.Exec(ctx, `UPDATE bom_import_batch SET workbook=$3 WHERE id=$1 AND project_id=$2`, importID, s.project, jsonBytes(wb))
	return wb, importID, digest, err
}
func (s *Server) kickoffAgentCheckpoint(ctx context.Context, job, source string, expected int, domains []string) (*ai.EngineeringCheckpoint, error) {
	var cp ai.EngineeringCheckpoint
	err := s.kickoffAgentArtifact(ctx, job, "checkpoint", &cp)
	if err == nil {
		return &cp, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var previous string
	err = s.db.QueryRow(ctx, `SELECT j.id::text FROM eng_job j WHERE j.source_id=$1 AND j.project_id=$2 AND j.expected_revision=$3 AND j.id<>$4 AND j.proposal->>'kind'=$5 AND EXISTS(SELECT 1 FROM eng_analysis_artifact a WHERE a.job_id=j.id AND a.kind='checkpoint') ORDER BY j.created_at DESC LIMIT 1`, source, s.project, expected, job, kickoffAgentKind).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = s.kickoffAgentArtifact(ctx, previous, "checkpoint", &cp); err != nil {
		return nil, err
	}
	// Keep the source route and other domain plans. A fresh request invalidates
	// only its selected domain tasks; a reclaimed job resumes its own checkpoints.
	if cp.Kickoff != nil {
		for _, domain := range domains {
			delete(cp.Kickoff.Tasks, ai.KickoffAgentStep(domain))
		}
	}
	return &cp, nil
}
func kickoffAgentCombine(values []ai.KickoffAgentCandidate) []ai.KickoffAgentCandidate {
	out := []ai.KickoffAgentCandidate{}
	index := map[string]int{}
	for _, v := range values {
		n, ok := index[v.TableID]
		if !ok {
			index[v.TableID] = len(out)
			out = append(out, v)
			continue
		}
		c := &out[n]
		offset := len(c.Rows)
		for i := range v.Evidence {
			if v.Evidence[i].Row > 0 {
				v.Evidence[i].Row += offset
			}
		}
		for i := range v.Issues {
			if v.Issues[i].Row > 0 {
				v.Issues[i].Row += offset
			}
		}
		c.Rows = append(c.Rows, v.Rows...)
		c.Evidence = append(c.Evidence, v.Evidence...)
		c.Issues = append(c.Issues, v.Issues...)
	}
	// Repeated rack-grid rows and identical identities across sheets describe
	// one candidate, with all contributing source references retained.
	for i := range out {
		c := &out[i]
		seen := map[string]int{}
		numbers := map[int]int{}
		rows := []map[string]string{}
		for j, row := range c.Rows {
			key := engineering.Digest(row)
			n, ok := seen[key]
			if !ok {
				rows = append(rows, row)
				n = len(rows)
				seen[key] = n
			}
			numbers[j+1] = n
		}
		c.Rows = rows
		for j := range c.Evidence {
			if c.Evidence[j].Row > 0 {
				c.Evidence[j].Row = numbers[c.Evidence[j].Row]
			}
		}
		for j := range c.Issues {
			if c.Issues[j].Row > 0 {
				c.Issues[j].Row = numbers[c.Issues[j].Row]
			}
		}
	}
	return out
}
func kickoffAgentFailure(w http.ResponseWriter, err error) {
	code := err.Error()
	status := 500
	switch code {
	case "KICKOFF_AGENT_DOMAINS_INVALID", "KICKOFF_AGENT_JOB_REQUIRED", "KICKOFF_AGENT_TABLE_INVALID":
		status = 400
	case "KICKOFF_SOURCE_NOT_FOUND", "KICKOFF_ANALYSIS_NOT_FOUND":
		status = 404
	case "REVISION_CONFLICT", "ENGINEERING_JOB_ACTIVE", "KICKOFF_ANALYSIS_NOT_READY":
		status = 409
	case "KICKOFF_AGENT_NO_CANDIDATES", "KICKOFF_CANDIDATE_INVALID":
		status = 422
	case "ANALYSIS_STORAGE_UNAVAILABLE", "ANALYSIS_ARTIFACT_HASH_MISMATCH":
		status = 503
	default:
		code = "KICKOFF_ANALYSIS_UNAVAILABLE"
	}
	fail(w, status, code)
}

// Like the existing termination-to-port projection, this reuses explicit
// endpoint identities without asking the model to identify them again. No
// specification, MAC, role, or component association is inferred.
func kickoffAgentProjectEndpoints(values []ai.KickoffAgentCandidate) []ai.KickoffAgentCandidate {
	devices := ai.KickoffAgentCandidate{TableID: "devices", Domain: "equipment", Rows: []map[string]string{}, Issues: []engineering.InputProblem{}, Evidence: []ai.KickoffAgentEvidence{}}
	ports := ai.KickoffAgentCandidate{TableID: "ports", Domain: "equipment", Rows: []map[string]string{}, Issues: []engineering.InputProblem{}, Evidence: []ai.KickoffAgentEvidence{}}
	seenDevice, seenPort := map[string]bool{}, map[string]bool{}
	for _, c := range values {
		for _, row := range c.Rows {
			if c.TableID == "devices" {
				seenDevice[row["serial"]] = true
			}
			if c.TableID == "ports" {
				seenPort[row["serial"]+"\x00"+row["port"]] = true
			}
		}
	}
	for _, c := range values {
		if c.TableID != "connections" && c.TableID != "ports" {
			continue
		}
		evidence := map[int]ai.KickoffAgentEvidence{}
		for _, e := range c.Evidence {
			evidence[e.Row] = e
		}
		for i, row := range c.Rows {
			sides := []string{""}
			if c.TableID == "connections" {
				sides = []string{"a_", "b_"}
			}
			for _, side := range sides {
				serial, name := row[side+"serial"], row[side+"port"]
				ref, exists := evidence[i+1]
				if serial == "" || !exists {
					continue
				}
				serialCell, hasSerial := ref.Fields[side+"serial"]
				if !hasSerial || serialCell.Cell == "" {
					continue
				}
				if !seenDevice[serial] {
					seenDevice[serial] = true
					devices.Rows = append(devices.Rows, map[string]string{"serial": serial})
					devices.Evidence = append(devices.Evidence, ai.KickoffAgentEvidence{Row: len(devices.Rows), Region: ref.Region, Sheet: ref.Sheet, SourceRow: ref.SourceRow, SourceColumn: ref.SourceColumn, Fields: map[string]ai.KickoffAgentCell{"serial": serialCell}, Missing: []string{"仅复用端口/连接中的明确设备身份，类型和型号仍待补充"}})
				}
				if name != "" && !seenPort[serial+"\x00"+name] {
					portCell, hasPort := ref.Fields[side+"port"]
					if !hasPort || portCell.Cell == "" {
						continue
					}
					seenPort[serial+"\x00"+name] = true
					ports.Rows = append(ports.Rows, map[string]string{"serial": serial, "port": name})
					ports.Evidence = append(ports.Evidence, ai.KickoffAgentEvidence{Row: len(ports.Rows), Region: ref.Region, Sheet: ref.Sheet, SourceRow: ref.SourceRow, SourceColumn: ref.SourceColumn, Fields: map[string]ai.KickoffAgentCell{"serial": serialCell, "port": portCell}, Missing: []string{"仅复用明确端点，端口规格仍待补充"}})
				}
			}
		}
	}
	if len(devices.Rows) > 0 {
		values = append(values, devices)
	}
	if len(ports.Rows) > 0 {
		values = append(values, ports)
	}
	return kickoffAgentCombine(values)
}
