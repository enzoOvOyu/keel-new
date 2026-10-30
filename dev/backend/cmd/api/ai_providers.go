package main

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/ai"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type aiConfig struct {
	TokenParameter string     `json:"token_parameter"`
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Protocol       string     `json:"protocol"`
	Endpoint       string     `json:"endpoint"`
	Model          string     `json:"model"`
	CredentialRef  string     `json:"credential_ref"`
	HasCredential  bool       `json:"has_credential"`
	Timeout        int        `json:"timeout_seconds"`
	Budget         int        `json:"token_budget"`
	JSONMode       bool       `json:"json_mode"`
	Enabled        bool       `json:"enabled"`
	Default        bool       `json:"is_default"`
	Revision       int64      `json:"revision"`
	TestStatus     string     `json:"last_test_status"`
	TestCode       *string    `json:"last_test_code"`
	HTTPStatus     *int       `json:"last_test_http_status"`
	TestAt         *time.Time `json:"last_test_at"`
	TestMS         *int64     `json:"last_test_ms"`
	APIKey         string     `json:"api_key,omitempty"`
}

const aiConfigSelect = "SELECT id::text,name,protocol,endpoint,model,credential_ref,timeout_seconds,token_budget,json_mode,enabled,is_default,revision,last_test_status,last_test_code,last_test_http_status,last_test_at,last_test_ms,token_parameter FROM ai_provider_config "

func scanAI(row pgx.Row) (aiConfig, error) {
	var c aiConfig
	e := row.Scan(&c.ID, &c.Name, &c.Protocol, &c.Endpoint, &c.Model, &c.CredentialRef, &c.Timeout, &c.Budget, &c.JSONMode, &c.Enabled, &c.Default, &c.Revision, &c.TestStatus, &c.TestCode, &c.HTTPStatus, &c.TestAt, &c.TestMS, &c.TokenParameter)
	c.HasCredential = c.CredentialRef != ""
	return c, e
}
func aiSecretPath(id string) string {
	return filepath.Join(env("KEEL_SECRET_DIR", ".secrets"), "ai-providers", id+".key")
}
func aiEndpoint(endpoint, protocol string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(endpoint))
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(env("APP_ENV", "development") != "production" && u.Scheme == "http")) {
		return "", errors.New("INVALID_PROVIDER_ENDPOINT")
	}
	if protocol == "openai-chat" {
		u.Path = strings.TrimRight(u.Path, "/")
		if !strings.HasSuffix(u.Path, "/chat/completions") {
			u.Path += "/chat/completions"
		}
	}
	return u.String(), nil
}
func (s *Server) defaultAI(ctx context.Context) (*ai.Provider, string, error) {
	c, e := scanAI(s.db.QueryRow(ctx, aiConfigSelect+"WHERE enabled AND is_default"))
	if e != nil {
		return nil, "", errors.New("AI_PROVIDER_NOT_CONFIGURED")
	}
	return s.aiAdapter(c)
}
func (s *Server) aiAdapter(c aiConfig) (*ai.Provider, string, error) {
	raw, e := os.ReadFile(aiSecretPath(strings.TrimPrefix(c.CredentialRef, "local-secret://ai-provider/")))
	if e != nil || len(raw) == 0 {
		return nil, c.ID, errors.New("AI_CREDENTIAL_UNAVAILABLE")
	}
	return &ai.Provider{ConfigID: c.ID, ConfigRevision: c.Revision, Endpoint: c.Endpoint, TokenParameter: c.TokenParameter, Model: c.Model, Key: string(raw), Protocol: c.Protocol, Timeout: time.Duration(c.Timeout) * time.Second, TokenBudget: c.Budget, JSONMode: c.JSONMode, AllowHTTP: env("APP_ENV", "development") != "production"}, c.ID, nil
}
func (s *Server) aiConfigured(ctx context.Context) bool {
	var ready bool
	s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM ai_provider_config WHERE enabled AND is_default AND credential_ref<>'')").Scan(&ready)
	return ready
}
func (s *Server) aiProviderRoutes(w http.ResponseWriter, r *http.Request) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	if path != "ai/provider-options" && !strings.HasPrefix(path, "admin/ai-providers") {
		return false
	}
	u := subjectOf(r.Context())
	ctx := r.Context()
	if path == "ai/provider-options" && r.Method == "GET" {
		project := r.URL.Query().Get("project_id")
		if !u.Can("ai:read", project, "", "") {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		rows, e := s.db.Query(ctx, "SELECT id::text,name,model,protocol,is_default FROM ai_provider_config WHERE enabled ORDER BY is_default DESC,name")
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, name, model, protocol string
			var def bool
			rows.Scan(&id, &name, &model, &protocol, &def)
			out = append(out, map[string]any{"id": id, "name": name, "model": model, "protocol": protocol, "is_default": def})
		}
		respond(w, 200, out)
		return true
	}
	permission := "admin:read"
	if r.Method != "GET" {
		permission = "admin:edit"
	}
	if !u.Can(permission, "", "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	p := strings.Split(path, "/")
	if len(p) == 2 && r.Method == "GET" {
		rows, e := s.db.Query(ctx, aiConfigSelect+"ORDER BY created_at")
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []aiConfig{}
		for rows.Next() {
			c, e := scanAI(rows)
			if e != nil {
				fail(w, 500, "DATABASE_ERROR")
				return true
			}
			out = append(out, c)
		}
		respond(w, 200, out)
		return true
	}
	if len(p) == 4 && p[3] == "test" && r.Method == "POST" {
		c, e := scanAI(s.db.QueryRow(ctx, aiConfigSelect+"WHERE id::text=$1", p[2]))
		if e != nil {
			fail(w, 404, "PROVIDER_NOT_FOUND")
			return true
		}
		var q struct {
			Revision int64 `json:"revision"`
		}
		if !decodeIdentity(w, r, &q) {
			return true
		}
		if q.Revision != c.Revision {
			fail(w, 409, "PROVIDER_REVISION_CONFLICT")
			return true
		}
		adapter, _, e := s.aiAdapter(c)
		start := time.Now()
		if e == nil {
			_, e = adapter.Suggest(ctx, ai.Request{Task: "field", Headers: []string{"SN"}, SourceFile: "keel-connectivity-test", SheetName: "synthetic", PromptVersion: "field/v1", SchemaVersion: "mapping/v1"})
		}
		status, code := "passed", ""
		if e != nil {
			status = "failed"
			code = e.Error()
		}
		httpStatus := 0
		if adapter != nil {
			httpStatus = adapter.HTTPStatus
		}
		ms := time.Since(start).Milliseconds()
		_, e = s.db.Exec(ctx, "UPDATE ai_provider_config SET last_test_status=$2,last_test_code=NULLIF($3,''),last_test_http_status=$4,last_test_at=now(),last_test_ms=$5 WHERE id::text=$1 AND revision=$6", c.ID, status, code, httpStatus, ms, c.Revision)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		s.audit(ctx, c.ID, "ai.provider.test", status+":"+code)
		respond(w, 200, map[string]any{"provider_id": c.ID, "revision": c.Revision, "status": status, "error_code": code, "http_status": httpStatus, "duration_ms": ms, "synthetic": true, "observed_at": time.Now().UTC()})
		return true
	}
	if (len(p) == 2 && r.Method == "POST") || (len(p) == 3 && r.Method == "PUT") {
		var c aiConfig
		if !decodeIdentity(w, r, &c) {
			return true
		}
		if c.TokenParameter == "" {
			c.TokenParameter = "max_tokens"
		}
		if c.TokenParameter != "max_tokens" && c.TokenParameter != "max_completion_tokens" {
			fail(w, 422, "INVALID_TOKEN_PARAMETER")
			return true
		}
		c.Name = strings.TrimSpace(c.Name)
		c.Model = strings.TrimSpace(c.Model)
		if len([]rune(c.Name)) < 2 || len([]rune(c.Name)) > 120 || c.Model == "" || len(c.Model) > 200 || (c.Protocol != "openai-chat" && c.Protocol != "keel-mapping") || c.Timeout < 1 || c.Timeout > 120 || c.Budget < 128 || c.Budget > 32768 {
			fail(w, 422, "INVALID_PROVIDER_CONFIGURATION")
			return true
		}
		endpoint, e := aiEndpoint(c.Endpoint, c.Protocol)
		if e != nil {
			fail(w, 422, e.Error())
			return true
		}
		c.Endpoint = endpoint
		creating := len(p) == 2
		if creating {
			c.ID = uuid.NewString()
		} else {
			c.ID = p[2]
			if _, e = uuid.Parse(c.ID); e != nil {
				fail(w, 404, "PROVIDER_NOT_FOUND")
				return true
			}
		}
		if c.Default && !c.Enabled {
			fail(w, 422, "DEFAULT_PROVIDER_MUST_BE_ENABLED")
			return true
		}
		tx, e := s.db.Begin(ctx)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1123199020)"); e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		ref := ""
		if !creating {
			var version int64
			if e = tx.QueryRow(ctx, "SELECT revision,credential_ref FROM ai_provider_config WHERE id::text=$1 FOR UPDATE", c.ID).Scan(&version, &ref); e != nil {
				fail(w, 404, "PROVIDER_NOT_FOUND")
				return true
			}
			if version != c.Revision {
				fail(w, 409, "PROVIDER_REVISION_CONFLICT")
				return true
			}
		}
		secret := strings.TrimSpace(c.APIKey)
		if secret != "" {
			if len(secret) > 8192 {
				fail(w, 422, "INVALID_API_KEY")
				return true
			}
			secretID := c.ID + "-" + uuid.NewString()
			file := aiSecretPath(secretID)
			if e = os.MkdirAll(filepath.Dir(file), 0700); e != nil {
				fail(w, 500, "SECRET_STORE_UNAVAILABLE")
				return true
			}
			if e = os.WriteFile(file, []byte(secret), 0600); e != nil {
				fail(w, 500, "SECRET_STORE_UNAVAILABLE")
				return true
			}
			ref = "local-secret://ai-provider/" + secretID
		}
		if c.Enabled && ref == "" {
			fail(w, 422, "AI_CREDENTIAL_REQUIRED")
			return true
		}
		if c.Default {
			if _, e = tx.Exec(ctx, "UPDATE ai_provider_config SET is_default=false,revision=revision+1 WHERE is_default AND id::text<>$1", c.ID); e != nil {
				fail(w, 500, "DATABASE_ERROR")
				return true
			}
		}
		if creating {
			_, e = tx.Exec(ctx, "INSERT INTO ai_provider_config(id,name,protocol,endpoint,model,credential_ref,timeout_seconds,token_budget,json_mode,enabled,is_default,token_parameter) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)", c.ID, c.Name, c.Protocol, c.Endpoint, c.Model, ref, c.Timeout, c.Budget, c.JSONMode, c.Enabled, c.Default, c.TokenParameter)
		} else {
			_, e = tx.Exec(ctx, "UPDATE ai_provider_config SET name=$2,protocol=$3,endpoint=$4,model=$5,credential_ref=$6,timeout_seconds=$7,token_budget=$8,json_mode=$9,enabled=$10,is_default=$11,revision=revision+1,updated_at=now(),last_test_status='not_tested',last_test_code=NULL,last_test_at=NULL,last_test_ms=NULL,last_test_http_status=NULL,token_parameter=$12 WHERE id::text=$1", c.ID, c.Name, c.Protocol, c.Endpoint, c.Model, ref, c.Timeout, c.Budget, c.JSONMode, c.Enabled, c.Default, c.TokenParameter)
		}
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		if _, e = tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,'system',$2,'ai.provider.saved',$3,$4)", uuid.NewString(), u.Username, c.ID, []byte(`{"secret_values_omitted":true}`)); e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		if e = tx.Commit(ctx); e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		respond(w, 200, map[string]any{"id": c.ID, "status": "saved"})
		return true
	}
	fail(w, 404, "PROVIDER_ENDPOINT_NOT_FOUND")
	return true
}
