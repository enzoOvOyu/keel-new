package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"keel.local/dev/internal/bom"
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
type HeaderCell struct {
	SourceFile string `json:"source_file"`
	Sheet      string `json:"sheet_name"`
	Row        int    `json:"source_row"`
	Column     int    `json:"source_column"`
	Cell       string `json:"source_cell"`
	Original   string `json:"original_value"`
}
type Request struct {
	Cells         []HeaderCell  `json:"source_cells,omitempty"`
	SourceFile    string        `json:"source_file"`
	SheetName     string        `json:"sheet_name"`
	ExplicitRefs  []ExplicitRef `json:"explicit_refs,omitempty"`
	Task          string        `json:"task"`
	Headers       []string      `json:"headers"`
	PromptVersion string        `json:"prompt_version"`
	SchemaVersion string        `json:"schema_version"`
}
type ExplicitRef struct {
	Reference  string   `json:"reference"`
	Candidates []string `json:"candidates"`
}
type Adapter interface {
	Suggest(context.Context, Request) ([]Candidate, error)
}
type Rules struct{}

func (Rules) Suggest(ctx context.Context, r Request) ([]Candidate, error) {
	out := []Candidate{}
	base := Candidate{SourceFile: r.SourceFile, Sheet: r.SheetName, Row: 1, Column: 1, Cell: "A1", Reason: "Deterministic candidate only", Confidence: 1, Model: "rules/v1", Prompt: r.PromptVersion, Schema: r.SchemaVersion}
	if base.SourceFile == "" {
		base.SourceFile = "structural-input"
	}
	if base.Sheet == "" {
		base.Sheet = "headers"
	}
	if len(r.Cells) > 0 && r.Task == "field" {
		for _, h := range r.Cells {
			if field := bom.Field(h.Original); field != "" {
				c := base
				c.SourceFile = h.SourceFile
				c.Sheet = h.Sheet
				c.Row = h.Row
				c.Column = h.Column
				c.Cell = h.Cell
				c.Original = h.Original
				c.Field = field
				out = append(out, c)
			}
		}
		return out, nil
	}
	if r.Task == "sheet" && len(r.Cells) > 0 {
		groups := map[string][]HeaderCell{}
		order := []string{}
		for _, c := range r.Cells {
			if _, ok := groups[c.Sheet]; !ok {
				order = append(order, c.Sheet)
			}
			groups[c.Sheet] = append(groups[c.Sheet], c)
		}
		for _, name := range order {
			cells := groups[name]
			sub := r
			sub.Cells = nil
			sub.Headers = nil
			sub.SheetName = name
			sub.SourceFile = cells[0].SourceFile
			for _, c := range cells {
				sub.Headers = append(sub.Headers, c.Original)
			}
			result, e := (Rules{}).Suggest(ctx, sub)
			if e != nil {
				return nil, e
			}
			for j := range result {
				result[j].Cell = cells[0].Cell
				result[j].Row = cells[0].Row
				result[j].Column = cells[0].Column
				result[j].Original = cells[0].Original
			}
			out = append(out, result...)
		}
		return out, nil
	}
	if r.Task == "sheet" {
		purpose := "unknown"
		fields := map[string]bool{}
		for _, h := range r.Headers {
			fields[bom.Field(h)] = true
		}
		if fields["serial"] || fields["hostname"] {
			purpose = "assets"
		}
		if fields["endpoint_a_device"] && fields["endpoint_b_device"] {
			purpose = "cabling"
		}
		base.Field = "sheet_purpose"
		base.Normalized = purpose
		base.Ambiguity = purpose == "unknown"
		return []Candidate{base}, nil
	}
	if r.Task == "relation" {
		for _, ref := range r.ExplicitRefs {
			c := base
			c.Field = "relation_ref"
			c.Original = ref.Reference
			if len(ref.Candidates) == 1 {
				c.Normalized = ref.Candidates[0]
			} else {
				c.Ambiguity = true
			}
			out = append(out, c)
		}
		return out, nil
	}
	for i, h := range r.Headers {
		if field := bom.Field(h); field != "" {
			out = append(out, Candidate{SourceFile: "header-vocabulary", Sheet: "headers", Row: 1, Column: i + 1, Cell: fmt.Sprintf("header-%d", i+1), Original: h, Field: field, Normalized: "", Reason: "Exact approved alias dictionary; candidate only", Confidence: 1, Model: "rules/v1", Prompt: r.PromptVersion, Schema: r.SchemaVersion})
		}
	}
	return out, nil
}

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
	if len(r.Cells) > 0 {
		h := r.Cells[0]
		c.SourceFile = h.SourceFile
		c.Sheet = h.Sheet
		c.Row = h.Row
		c.Column = h.Column
		c.Cell = h.Cell
		c.Original = h.Original
		if field := bom.Field(h.Original); field != "" {
			c.Field = field
		}
	}
	if r.Task == "sheet" || r.Task == "relation" {
		results, _ := (Rules{}).Suggest(ctx, r)
		if len(results) > 0 {
			c = results[0]
			c.Model = "mock"
			c.Reason = "SIMULATION_ONLY"
			c.Confidence = .95
		}
	}
	if m.Scenario == "low-confidence" {
		c.Confidence = .3
	}
	if m.Scenario == "ambiguity" || m.Scenario == "conflict" {
		c.Ambiguity = true
	}
	if m.Scenario == "unknown-field" {
		c.Field = "unknown"
	}
	if m.Scenario == "conflict" {
		other := c
		other.Field = "hostname"
		if c.Field == "hostname" {
			other.Field = "serial"
		}
		other.Ambiguity = true
		return []Candidate{c, other}, nil
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
	if d.Decode(new(any)) != io.EOF {
		return nil, errors.New("INVALID_JSON_TRAILING_DATA")
	}
	for _, c := range out {
		if c.SourceFile == "" || c.Sheet == "" || c.Cell == "" || c.Row < 1 || c.Column < 1 || c.Field == "" || c.Model == "" || c.Prompt == "" || c.Schema == "" || c.Confidence < 0 || c.Confidence > 1 {
			return nil, errors.New("SCHEMA_VALIDATION_FAILED")
		}
	}
	return out, nil
}

type Provider struct {
	Client               *http.Client
	Endpoint, Model, Key string
	Raw                  []byte
	TokenBudget          int
	InputTokens          *int
	OutputTokens         *int
	Timeout              time.Duration
}

func (p *Provider) Suggest(ctx context.Context, r Request) ([]Candidate, error) {
	u, e := url.Parse(p.Endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, errors.New("INVALID_PROVIDER_ENDPOINT")
	}
	prompt, e := Prompt(r.Task)
	if e != nil {
		return nil, e
	}
	budget := p.TokenBudget
	if budget < 1 {
		budget = 2048
	}
	b, _ := json.Marshal(map[string]any{"model": p.Model, "input": r, "response_format": "mapping/v1", "system": prompt, "max_output_tokens": budget})
	req, e := http.NewRequestWithContext(ctx, "POST", p.Endpoint, bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+p.Key)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: p.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("REDIRECT_DENIED") }}
	if p.Client != nil {
		client = *p.Client
		client.Timeout = p.Timeout
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("REDIRECT_DENIED") }
	}
	res, e := client.Do(req)
	if e != nil {
		return nil, errors.New("PROVIDER_UNAVAILABLE")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("PROVIDER_ERROR")
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(data) > 1<<20 {
		return nil, errors.New("PROVIDER_RESPONSE_TOO_LARGE")
	}
	p.Raw = append([]byte{}, data...)
	if len(data) > 0 && data[0] == '{' {
		var envelope struct {
			Candidates json.RawMessage `json:"candidates"`
			Usage      struct {
				Input  *int `json:"input_tokens"`
				Output *int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(data, &envelope) != nil || len(envelope.Candidates) == 0 {
			return nil, errors.New("SCHEMA_VALIDATION_FAILED")
		}
		p.InputTokens = envelope.Usage.Input
		p.OutputTokens = envelope.Usage.Output
		return Decode(envelope.Candidates)
	}
	return Decode(data)
}
