package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (s *Server) aiAPI(w http.ResponseWriter, r *http.Request, id string, rev int, status string, workbook []byte) bool {
	evaluate := strings.HasSuffix(r.URL.Path, "/evaluate")
	if !strings.HasSuffix(r.URL.Path, "/ai-tasks") && !evaluate {
		return false
	}
	ctx := r.Context()
	if r.Method == "GET" {
		rows, e := s.db.Query(ctx, "SELECT id,mode,task,status,diagnostic,created_at FROM ai_task WHERE import_id=$1 ORDER BY created_at DESC LIMIT 100", id)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var aid, mode, task, status string
			var diag []byte
			var at time.Time
			if rows.Scan(&aid, &mode, &task, &status, &diag, &at) != nil {
				fail(w, 500, "DATABASE_ERROR")
				return true
			}
			out = append(out, map[string]any{"id": aid, "mode": mode, "task": task, "status": status, "diagnostic": json.RawMessage(diag), "created_at": at})
		}
		respond(w, 200, out)
		return true
	}
	if r.Method != "POST" {
		return false
	}
	var req struct {
		Mode     string `json:"mode"`
		Task     string `json:"task"`
		Scenario string `json:"scenario"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		fail(w, 400, "INVALID_REQUEST")
		return true
	}
	if req.Task != "sheet" && req.Task != "field" && req.Task != "relation" {
		fail(w, 422, "UNKNOWN_PROMPT")
		return true
	}
	if status == "COMMITTED" {
		fail(w, 409, "IMPORT_IMMUTABLE")
		return true
	}
	var adapter ai.Adapter
	switch req.Mode {
	case "rules":
		adapter = ai.Rules{}
	case "mock":
		adapter = ai.Mock{Scenario: req.Scenario}
	case "provider":
		endpoint := os.Getenv("AI_ENDPOINT")
		key := os.Getenv("AI_API_KEY")
		if endpoint == "" || key == "" {
			fail(w, 422, "AI_PROVIDER_NOT_CONFIGURED")
			return true
		}
		seconds, _ := strconv.Atoi(env("AI_TIMEOUT_SECONDS", "30"))
		budget, _ := strconv.Atoi(env("AI_TOKEN_BUDGET", "2048"))
		if seconds < 1 || seconds > 120 || budget < 1 || budget > 32768 {
			fail(w, 422, "INVALID_AI_CONFIGURATION")
			return true
		}
		adapter = &ai.Provider{Endpoint: endpoint, Model: os.Getenv("AI_MODEL"), Key: key, Timeout: time.Duration(seconds) * time.Second, TokenBudget: budget}
	default:
		fail(w, 400, "INVALID_AI_MODE")
		return true
	}
	if evaluate {
		evaluation := ai.Evaluate(ctx, adapter, req.Mode)
		b, _ := json.Marshal(evaluation)
		aid := uuid.NewString()
		_, e := s.db.Exec(ctx, "INSERT INTO ai_task(id,import_id,mode,task,status,diagnostic) VALUES($1,$2,$3,'evaluation','SUCCEEDED',$4)", aid, id, req.Mode, b)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		respond(w, 200, evaluation)
		return true
	}
	// No raw workbook values are ever sent. Header vocabulary is explicitly allowlisted.
	var wb bom.Workbook
	json.Unmarshal(workbook, &wb)
	headers := []string{}
	for _, m := range bom.Rules(wb) {
		if !bom.Sensitive(m.Header) {
			headers = append(headers, m.Header)
		}
	}
	input := ai.Request{Task: req.Task, Headers: headers, PromptVersion: req.Task + "/v1", SchemaVersion: "mapping/v1"}
	var profileRaw []byte
	s.db.QueryRow(ctx, "SELECT profiles FROM bom_import_batch WHERE id=$1", id).Scan(&profileRaw)
	var profiles []bom.Profile
	json.Unmarshal(profileRaw, &profiles)
	for _, p := range profiles {
		for _, sh := range wb.Sheets {
			if sh.Name != p.Sheet {
				continue
			}
			for _, c := range sh.Cells {
				header := false
				for _, r := range p.HeaderRows {
					if c.Row == r {
						header = true
					}
				}
				if !header || c.Sensitive || c.Formula != "" || bom.Sensitive(c.Value) || len(c.Value) > 120 {
					continue
				}
				if bom.Field(c.Value) == "" {
					sensitiveLiteral := net.ParseIP(c.Value) != nil || strings.Contains(c.Value, "@") || strings.Contains(c.Value, ":")
					for _, r := range c.Value {
						if unicode.IsDigit(r) {
							sensitiveLiteral = true
						}
					}
					if sensitiveLiteral {
						continue
					}
				}
				if bom.Field(c.Value) == "" && !(p.Confirmed && os.Getenv("AI_ALLOW_UNKNOWN_HEADERS") == "true") {
					continue
				}
				input.Cells = append(input.Cells, ai.HeaderCell{SourceFile: id, Sheet: sh.Name, Row: c.Row, Column: c.Column, Cell: c.Ref, Original: c.Value})
			}
		}
	}
	if req.Task == "relation" {
		var raw []byte
		s.db.QueryRow(ctx, "SELECT draft FROM bom_import_batch WHERE id=$1", id).Scan(&raw)
		var items []bom.Item
		json.Unmarshal(raw, &items)
		for _, relation := range bom.Relations(items) {
			input.ExplicitRefs = append(input.ExplicitRefs, ai.ExplicitRef{Reference: relation.ID, Candidates: relation.Candidates})
		}
		input.SourceFile = id
		input.SheetName = "explicit-relation-candidates"
	}
	if len(input.Cells) > 512 {
		fail(w, 422, "AI_CONTEXT_LIMIT")
		return true
	}

	hash := sha256.Sum256([]byte(strings.Join(headers, "\n")))
	start := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	candidates, e := adapter.Suggest(runCtx, input)
	state := "SUCCEEDED"
	errCode := ""
	if e != nil {
		state = "FAILED"
		errCode = e.Error()
	}
	rawRef := ""
	if provider, ok := adapter.(*ai.Provider); ok && len(provider.Raw) > 0 {
		digest := sha256.Sum256(provider.Raw)
		rawRef = s.project + "/ai-output/" + hex.EncodeToString(digest[:]) + ".json"
		if _, saveErr := s.objects.PutObject(ctx, s.bucket, rawRef, bytes.NewReader(provider.Raw), int64(len(provider.Raw)), minio.PutObjectOptions{ContentType: "application/json"}); saveErr != nil {
			state = "FAILED"
			errCode = "AI_EVIDENCE_STORAGE_FAILED"
			rawRef = ""
		}
	}
	schemaStatus := "valid"
	var schemaValid any = true
	if e != nil {
		schemaValid = nil
		schemaStatus = "not_run"
		if strings.HasPrefix(errCode, "INVALID_JSON") {
			schemaStatus = "invalid_json"
			schemaValid = false
		}
		if strings.Contains(errCode, "SCHEMA_VALIDATION") {
			schemaStatus = "invalid_schema"
			schemaValid = false
		}
	}
	diagnostic := map[string]any{"schema_status": schemaStatus, "raw_response_ref": rawRef, "prompt_version": input.PromptVersion, "schema_version": input.SchemaVersion, "request_sha256": hex.EncodeToString(hash[:]), "duration_ms": time.Since(start).Milliseconds(), "schema_valid": schemaValid, "error_code": errCode, "model": os.Getenv("AI_MODEL"), "candidates": candidates, "simulation_only": req.Mode == "mock", "token_input": nil, "token_output": nil, "token_cost": nil, "token_support": "unknown", "merged_into_bom": false}
	if provider, ok := adapter.(*ai.Provider); ok {
		diagnostic["token_input"] = provider.InputTokens
		diagnostic["token_output"] = provider.OutputTokens
		if provider.InputTokens != nil && provider.OutputTokens != nil {
			diagnostic["token_support"] = "supported"
		}
	}
	b, _ := json.Marshal(diagnostic)
	aid := uuid.NewString()
	_, dbErr := s.db.Exec(ctx, "INSERT INTO ai_task(id,import_id,mode,task,status,diagnostic) VALUES($1,$2,$3,$4,$5,$6)", aid, id, req.Mode, req.Task, state, b)
	if dbErr != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	s.audit(ctx, id, "ai."+state, req.Mode+":"+req.Task)
	respond(w, 200, map[string]any{"id": aid, "status": state, "diagnostic": diagnostic})
	return true
}
