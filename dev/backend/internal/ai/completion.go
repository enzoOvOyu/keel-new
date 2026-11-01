package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ProviderError struct {
	Status     int
	Reason     string
	RetryAfter time.Duration
}

func (e *ProviderError) Error() string {
	if e.Status == 0 {
		return "PROVIDER_TIMEOUT_OR_UNAVAILABLE"
	}
	if e.Status == 200 {
		return "PROVIDER_RESPONSE_FAILED: " + e.Reason
	}
	return fmt.Sprintf("PROVIDER_ERROR: HTTP %d %s", e.Status, e.Reason)
}
func (e *ProviderError) Temporary() bool {
	return e.Status == 0 || (e.Status == 200 && (e.Reason == "response_timeout" || e.Reason == "response_interrupted")) || e.Status == 429 || e.Status == 502 || e.Status == 503 || e.Status == 504 || (e.Status == 402 && e.Reason == "in_flight_budget_exhausted")
}

func (p *Provider) completeJSON(ctx context.Context, prompt string, input any, format ...map[string]any) ([]byte, error) {
	return p.complete(ctx, prompt, input, nil, format...)
}
func (p *Provider) complete(ctx context.Context, prompt string, input any, functions []any, format ...map[string]any) ([]byte, error) {
	p.Calls++
	p.HTTPStatus = 0
	p.Raw = nil
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
	if len(functions) > 0 {
		// Model tool outputs as actual tool messages; embedding a pseudo transcript
		// in a user JSON payload causes models to repeatedly request the same data.
		if values, ok := input.(map[string]any); ok {
			contextValues := map[string]any{}
			for k, v := range values {
				if k != "history" {
					contextValues[k] = v
				}
			}
			contextJSON, _ := json.Marshal(contextValues)
			messages := []map[string]any{{"role": "system", "content": prompt}, {"role": "user", "content": string(contextJSON)}}
			history, _ := values["history"].([]any)
			toolErrors := []map[string]any{}
			for i, entry := range history {
				encoded, _ := json.Marshal(entry)
				var item struct {
					Action   json.RawMessage `json:"action"`
					Rejected json.RawMessage `json:"rejected_action"`
					Result   any             `json:"result"`
					Error    string          `json:"error"`
				}
				if json.Unmarshal(encoded, &item) != nil {
					continue
				}
				action := item.Action
				if len(action) == 0 {
					action = item.Rejected
				}
				var args map[string]json.RawMessage
				if json.Unmarshal(action, &args) != nil {
					continue
				}
				var name string
				_ = json.Unmarshal(args["action"], &name)
				if name == "" {
					continue
				}
				failure := item.Error
				if result, ok := item.Result.(map[string]any); ok {
					if detail, ok := result["error"].(string); ok {
						failure = detail
					}
				}
				if failure != "" {
					feedback := map[string]any{"tool": name, "error": failure}
					if result, ok := item.Result.(map[string]any); ok {
						for _, key := range []string{"binding_errors", "available_formula_cells", "next"} {
							if value, exists := result[key]; exists {
								feedback[key] = value
							}
						}
					}
					toolErrors = append(toolErrors, feedback)
					continue // Keep rejected large proposals in artifacts, not as examples to imitate.
				}
				delete(args, "action")
				arguments, _ := json.Marshal(args)
				id := fmt.Sprintf("history_%d", i)
				result := item.Result
				resultJSON, _ := json.Marshal(result)
				messages = append(messages, map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": string(arguments)}}}}, map[string]any{"role": "tool", "tool_call_id": id, "content": string(resultJSON)})
			}
			if len(toolErrors) > 0 {
				contextValues["tool_errors"] = toolErrors
				contextJSON, _ = json.Marshal(contextValues)
				messages[1]["content"] = string(contextJSON)
				feedback, _ := json.Marshal(map[string]any{"tool_errors": toolErrors[len(toolErrors)-1:], "instruction": "Correct this latest tool error using active_sheet_preview; rejected proposals are not evidence."})
				messages = append(messages, map[string]any{"role": "user", "content": string(feedback)})
			}
			payload["messages"] = messages
		}
		delete(payload, "response_format")
		payload["tools"] = functions
		payload["tool_choice"] = "required"
		payload["parallel_tool_calls"] = false
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
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &ProviderError{Reason: "request_timeout_or_unavailable"}
	}
	defer res.Body.Close()
	p.HTTPStatus = res.StatusCode
	if res.StatusCode != 200 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		var body struct {
			Error struct {
				Metadata struct {
					Reason  string            `json:"reason"`
					Headers map[string]string `json:"headers"`
				} `json:"metadata"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &body)
		reason := "upstream_rejected"
		// Never surface arbitrary upstream text: it can contain prompts or credentials.
		switch body.Error.Metadata.Reason {
		case "in_flight_budget_exhausted":
			reason = body.Error.Metadata.Reason
		}
		retry := res.Header.Get("Retry-After")
		if retry == "" {
			retry = body.Error.Metadata.Headers["Retry-After"]
		}
		delay := time.Duration(0)
		if seconds, err := strconv.Atoi(retry); err == nil {
			delay = time.Duration(seconds) * time.Second
		} else if at, err := http.ParseTime(retry); err == nil {
			delay = time.Until(at)
		}
		return nil, &ProviderError{Status: res.StatusCode, Reason: reason, RetryAfter: delay}
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if e != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		reason := "response_interrupted"
		var networkError net.Error
		if errors.As(e, &networkError) && networkError.Timeout() {
			reason = "response_timeout"
		}
		return nil, &ProviderError{Status: 200, Reason: reason}
	}
	p.Raw = append([]byte{}, raw...)
	var response struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
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

	if len(functions) > 0 && len(response.Choices[0].Message.ToolCalls) > 0 {
		calls := response.Choices[0].Message.ToolCalls
		actions := []json.RawMessage{}
		for _, call := range calls {
			var args map[string]json.RawMessage
			if json.Unmarshal([]byte(call.Function.Arguments), &args) != nil || args == nil {
				return nil, errors.New("INVALID_JSON")
			}
			args["action"], _ = json.Marshal(call.Function.Name)
			raw, _ := json.Marshal(args)
			actions = append(actions, raw)
		}
		if len(actions) == 1 {
			return actions[0], nil
		}
		return json.Marshal(actions)
	}
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
