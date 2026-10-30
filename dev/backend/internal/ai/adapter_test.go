package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMockErrorsAreNotCandidates(t *testing.T) {
	for _, s := range []string{"invalid-json", "schema-failure", "timeout", "provider-error"} {
		if _, e := (Mock{s}).Suggest(context.Background(), Request{}); e == nil {
			t.Fatal(s)
		}
	}
	c, e := (Mock{"low-confidence"}).Suggest(context.Background(), Request{PromptVersion: "field/v1", SchemaVersion: "mapping/v1"})
	if e != nil || c[0].Confidence >= .5 {
		t.Fatal("low confidence lost")
	}
}
func TestProviderHTTPSContractWithSyntheticService(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer synthetic-test-key" {
			t.Error("credential missing")
		}
		var body map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&body) != nil || len(body["system"]) == 0 || len(body["max_output_tokens"]) == 0 {
			t.Error("versioned prompt or token budget missing")
		}
		json.NewEncoder(w).Encode(map[string]any{"candidates": []Candidate{{SourceFile: "synthetic", Sheet: "synthetic", Row: 1, Column: 1, Cell: "A1", Original: "SN", Field: "serial", Reason: "synthetic protocol response", Confidence: 1, Model: "synthetic-provider", Prompt: "field/v1", Schema: "mapping/v1"}}, "usage": map[string]int{"input_tokens": 10, "output_tokens": 20}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Model: "synthetic-provider", Key: "synthetic-test-key", Timeout: time.Second, Client: server.Client()}
	out, e := p.Suggest(context.Background(), Request{Task: "field", Headers: []string{"SN"}, PromptVersion: "field/v1", SchemaVersion: "mapping/v1"})
	if e != nil || len(out) != 1 || p.InputTokens == nil || *p.OutputTokens != 20 {
		t.Fatal("synthetic HTTPS protocol failed", e)
	}
}
func TestRulesGoldenIsExplicitlySynthetic(t *testing.T) {
	e := Evaluate(context.Background(), Rules{}, "rules")
	if e.FieldAccuracy == nil || *e.FieldAccuracy != 1 || e.SheetAccuracy == nil || *e.SheetAccuracy != 1 || e.RelationAccuracy == nil || *e.RelationAccuracy != 1 || e.TokenCost != nil {
		t.Fatal("golden results or unknown token cost incorrect")
	}
}
