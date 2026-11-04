package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEngineeringJSONCorrectionArchivesBothFailures(t *testing.T) {
	for _, recoverOutput := range []bool{true, false} {
		t.Run(fmt.Sprint(recoverOutput), func(t *testing.T) {
			calls, archived, failed := 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if calls == 2 && (!strings.Contains(body.Messages[0].Content, "Return exactly one complete JSON object") || strings.Contains(body.Messages[0].Content, "supplied function")) {
					t.Error("JSON correction must match the actual non-function protocol")
				}
				content := "invalid output"
				if calls == 2 && recoverOutput {
					content = `{"action":"inspect_sheet","sheet":"synthetic"}`
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}, "finish_reason": "stop"}}})
			}))
			defer server.Close()
			p := Provider{Endpoint: server.URL, Timeout: time.Second, JSONMode: true}
			raw, err := p.engineeringCompletion(context.Background(), "Return JSON", map[string]any{}, nil, func(tool, state, detail string) error {
				if tool == "provider_request" && state == "failed" {
					failed++
				}
				return nil
			}, func(_ context.Context, tool, state string, v any) error {
				archived++
				if tool != "provider_output" || state != "rejected" {
					t.Error("missing rejected response")
				}
				return nil
			})
			if calls != 2 {
				t.Fatalf("unbounded retry: %d", calls)
			}
			if recoverOutput {
				if err != nil || !json.Valid(raw) || archived != 1 || failed != 0 {
					t.Fatalf("correction failed: %v archived=%d failed=%d", err, archived, failed)
				}
			} else if err == nil || archived != 2 || failed != 1 {
				t.Fatalf("failure evidence missing: %v archived=%d failed=%d", err, archived, failed)
			}
		})
	}
}
