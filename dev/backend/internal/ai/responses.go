package ai

import (
	"encoding/json"
	"errors"
)

// Translate the shared conversation into stateless Responses input items.
func responsesPayload(chat map[string]any) map[string]any {
	out := map[string]any{"model": chat["model"], "stream": false, "store": false}
	if n, ok := chat["max_tokens"]; ok {
		out["max_output_tokens"] = n
	} else {
		out["max_output_tokens"] = chat["max_completion_tokens"]
	}
	raw, _ := json.Marshal(chat["messages"])
	var messages []map[string]any
	_ = json.Unmarshal(raw, &messages)
	items := []any{}
	for _, m := range messages {
		if m["role"] == "tool" {
			items = append(items, map[string]any{"type": "function_call_output", "call_id": m["tool_call_id"], "output": m["content"]})
			continue
		}
		if calls, ok := m["tool_calls"].([]any); ok {
			for _, v := range calls {
				c := v.(map[string]any)
				f := c["function"].(map[string]any)
				items = append(items, map[string]any{"type": "function_call", "call_id": c["id"], "name": f["name"], "arguments": f["arguments"]})
			}
			continue
		}
		items = append(items, m)
	}
	out["input"] = items
	if tools, ok := chat["tools"].([]any); ok {
		converted := []any{}
		for _, v := range tools {
			raw, _ := json.Marshal(v)
			var t struct {
				Function map[string]any `json:"function"`
			}
			_ = json.Unmarshal(raw, &t)
			t.Function["type"] = "function"
			if _, ok := t.Function["strict"]; !ok {
				t.Function["strict"] = false
			}
			converted = append(converted, t.Function)
		}
		out["tools"] = converted
		out["tool_choice"] = chat["tool_choice"]
		out["parallel_tool_calls"] = false
	}
	if value, ok := chat["response_format"]; ok {
		raw, _ := json.Marshal(value)
		var f map[string]any
		_ = json.Unmarshal(raw, &f)
		if f["type"] == "json_schema" {
			schema, _ := f["json_schema"].(map[string]any)
			f = schema
			f["type"] = "json_schema"
		}
		out["text"] = map[string]any{"format": f}
	}
	return out
}

func responsesCompletion(raw []byte) ([]byte, error) {
	var r struct {
		Status string `json:"status"`
		Output []struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			Input  *int `json:"input_tokens"`
			Output *int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return nil, errors.New("SCHEMA_VALIDATION_FAILED")
	}
	if r.Status == "incomplete" {
		return nil, errors.New("PROVIDER_OUTPUT_TRUNCATED")
	}
	if r.Status != "completed" {
		return nil, errors.New("PROVIDER_RESPONSE_FAILED")
	}
	content := ""
	calls := []any{}
	for _, item := range r.Output {
		if item.Type == "function_call" {
			calls = append(calls, map[string]any{"function": map[string]string{"name": item.Name, "arguments": item.Arguments}})
		}
		for _, part := range item.Content {
			if part.Type == "refusal" {
				return nil, errors.New("PROVIDER_RESPONSE_FAILED")
			}
			if part.Type == "output_text" {
				content += part.Text
			}
		}
	}
	return json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content, "tool_calls": calls}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": r.Usage.Input, "completion_tokens": r.Usage.Output}})
}
