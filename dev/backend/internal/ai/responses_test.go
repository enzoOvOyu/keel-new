package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResponsesCompletion(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["max_output_tokens"] != float64(512) || body["messages"] != nil || body["store"] != false {
			t.Errorf("unexpected request: %v", body)
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]string{"type": "output_text", "text": "{\"ok\":true}"}}}}, "usage": map[string]int{"input_tokens": 3, "output_tokens": 4}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Protocol: "openai-responses", Model: "synthetic", TokenBudget: 512, JSONMode: true, Timeout: time.Second, Client: server.Client()}
	raw, err := p.completeJSON(context.Background(), "Return JSON", map[string]string{"input": "synthetic"})
	if err != nil || string(raw) != "{\"ok\":true}" || p.TotalOutputTokens != 4 {
		t.Fatalf("%s %v usage=%d", raw, err, p.TotalOutputTokens)
	}
}
func TestResponsesToolHistory(t *testing.T) {
	chat := map[string]any{"model": "test", "max_tokens": 128, "messages": []any{map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "call-1", "function": map[string]any{"name": "read_range", "arguments": "{}"}}}}, map[string]any{"role": "tool", "tool_call_id": "call-1", "content": "{}"}}, "tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": "read_range", "parameters": map[string]any{"type": "object"}}}}, "tool_choice": "required"}
	payload := responsesPayload(chat)
	input := payload["input"].([]any)
	if input[0].(map[string]any)["call_id"] != "call-1" || input[1].(map[string]any)["type"] != "function_call_output" {
		t.Fatal(input)
	}
	if _, err := responsesCompletion([]byte(`{"status":"incomplete"}`)); err == nil {
		t.Fatal("truncated response accepted")
	}
}
