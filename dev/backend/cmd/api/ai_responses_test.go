package main

import "testing"

func TestAIResponsesEndpoint(t *testing.T) {
	for _, url := range []string{"https://example.test/v1", "https://example.test/v1/responses", "https://example.test/v1/chat/completions"} {
		got, err := aiEndpoint(url, "openai-responses")
		if err != nil || got != "https://example.test/v1/responses" {
			t.Fatalf("%s: %s %v", url, got, err)
		}
	}
}
