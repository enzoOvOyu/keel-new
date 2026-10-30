package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Candidate struct {
	SourceFile string  `json:"source_file"`
	Sheet      string  `json:"sheet_name"`
	Row        int     `json:"source_row"`
	Column     int     `json:"source_column"`
	Cell       string  `json:"source_cell"`
	Original   string  `json:"original_value"`
	Field      string  `json:"candidate_field"`
	Normalized string  `json:"normalized_value"`
	Reason     string  `json:"reasoning"`
	Confidence float64 `json:"confidence"`
	Ambiguity  bool    `json:"ambiguity"`
	Model      string  `json:"model"`
	Prompt     string  `json:"prompt_version"`
	Schema     string  `json:"schema_version"`
}
type Request struct {
	Task          string   `json:"task"`
	Headers       []string `json:"headers"`
	PromptVersion string   `json:"prompt_version"`
	SchemaVersion string   `json:"schema_version"`
}
type Adapter interface {
	Suggest(context.Context, Request) ([]Candidate, error)
}
type Rules struct{}

func (Rules) Suggest(context.Context, Request) ([]Candidate, error) { return []Candidate{}, nil }

type Mock struct{ Scenario string }

func (m Mock) Suggest(ctx context.Context, r Request) ([]Candidate, error) {
	switch m.Scenario {
	case "timeout":
		return nil, context.DeadlineExceeded
	case "provider-error":
		return nil, errors.New("PROVIDER_ERROR")
	case "invalid-json":
		return Decode([]byte("invalid"))
	case "schema-failure":
		return Decode([]byte(`[{"confidence":2}]`))
	}
	c := Candidate{SourceFile: "synthetic", Sheet: "synthetic", Row: 1, Column: 1, Cell: "A1", Original: "SN", Field: "serial", Reason: "SIMULATION_ONLY", Confidence: 0.95, Model: "mock", Prompt: r.PromptVersion, Schema: r.SchemaVersion}
	if m.Scenario == "low-confidence" {
		c.Confidence = .3
	}
	if m.Scenario == "ambiguity" || m.Scenario == "conflict" {
		c.Ambiguity = true
	}
	if m.Scenario == "unknown-field" {
		c.Field = "unknown"
	}
	return []Candidate{c}, nil
}
func Decode(b []byte) ([]Candidate, error) {
	var out []Candidate
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(&out); e != nil {
		return nil, errors.New("INVALID_JSON")
	}
	for _, c := range out {
		if c.SourceFile == "" || c.Sheet == "" || c.Cell == "" || c.Row < 1 || c.Column < 1 || c.Field == "" || c.Model == "" || c.Prompt == "" || c.Schema == "" || c.Confidence < 0 || c.Confidence > 1 {
			return nil, errors.New("SCHEMA_VALIDATION_FAILED")
		}
	}
	return out, nil
}

type Provider struct {
	Endpoint, Model, Key string
	Timeout              time.Duration
}

func (p Provider) Suggest(ctx context.Context, r Request) ([]Candidate, error) {
	u, e := url.Parse(p.Endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, errors.New("INVALID_PROVIDER_ENDPOINT")
	}
	b, _ := json.Marshal(map[string]any{"model": p.Model, "input": r, "response_format": "mapping/v1"})
	req, e := http.NewRequestWithContext(ctx, "POST", p.Endpoint, bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+p.Key)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: p.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("REDIRECT_DENIED") }}
	res, e := client.Do(req)
	if e != nil {
		return nil, errors.New("PROVIDER_UNAVAILABLE")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("PROVIDER_ERROR")
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if e != nil {
		return nil, e
	}
	return Decode(data)
}
