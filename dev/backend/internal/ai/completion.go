package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (p *Provider) completeJSON(ctx context.Context, prompt string, input any, format ...map[string]any) ([]byte, error) {
	p.Calls++
	encoded, _ := json.Marshal(input)
	payload := map[string]any{"model": p.Model, "messages": []map[string]string{{"role": "system", "content": prompt}, {"role": "user", "content": string(encoded)}}, "stream": false}
	token := "max_tokens"
	if p.TokenParameter == "max_completion_tokens" {
		token = p.TokenParameter
	}
	payload[token] = p.TokenBudget
	if endpoint, err := url.Parse(p.Endpoint); err == nil && endpoint.Hostname() == "openrouter.ai" {
		payload["reasoning"] = map[string]bool{"enabled": false}
	}
	if p.JSONMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	if len(format) > 0 {
		payload["response_format"] = format[0]
	}
	b, _ := json.Marshal(payload)
	req, e := http.NewRequestWithContext(ctx, "POST", p.Endpoint, bytes.NewReader(b))
	if e != nil {
		return nil, errors.New("PROVIDER_RESPONSE_FAILED")
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
		return nil, errors.New("PROVIDER_TIMEOUT_OR_UNAVAILABLE")
	}
	defer res.Body.Close()
	p.HTTPStatus = res.StatusCode
	if res.StatusCode != 200 {
		return nil, errors.New("PROVIDER_ERROR")
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if e != nil {
		return nil, errors.New("PROVIDER_RESPONSE_FAILED")
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Finish string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			Input  *int `json:"prompt_tokens"`
			Output *int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &response) != nil || len(response.Choices) != 1 {
		return nil, errors.New("SCHEMA_VALIDATION_FAILED")
	}
	if response.Choices[0].Finish == "length" {
		return nil, errors.New("PROVIDER_OUTPUT_TRUNCATED")
	}
	if response.Usage.Input != nil {
		p.TotalInputTokens += *response.Usage.Input
	}
	if response.Usage.Output != nil {
		p.TotalOutputTokens += *response.Usage.Output
	}
	p.InputTokens = response.Usage.Input
	p.OutputTokens = response.Usage.Output

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}
	if !json.Valid([]byte(content)) {
		return nil, errors.New("INVALID_JSON")
	}
	return []byte(content), nil
}
