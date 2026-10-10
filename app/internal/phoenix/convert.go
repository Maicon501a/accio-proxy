package phoenix

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BuildBody converts an OpenAI-shaped chat request into the Accio ADK wire
// format (camelCase, verified against the desktop SDK transport).
func BuildBody(in map[string]any, token, requestID string) map[string]any {
	model, _ := in["model"].(string)
	model = strings.TrimPrefix(model, "accio/")
	contents := make([]map[string]any, 0)
	system := ""
	if msgs, ok := in["messages"].([]any); ok {
		for _, raw := range msgs {
			m, _ := raw.(map[string]any)
			role, _ := m["role"].(string)
			parts := messageParts(m)
			if role == "system" {
				text := textContent(m["content"])
				if system != "" && text != "" {
					system += "\n\n"
				}
				system += text
				continue
			}
			if role == "assistant" {
				role = "model"
			}
			if role == "tool" {
				role = "user"
			}
			if len(parts) > 0 {
				contents = append(contents, map[string]any{"role": role, "parts": parts})
			}
		}
	}
	out := map[string]any{
		"model":         model,
		"incremental":   true,
		"requestId":     requestID,
		"userRequestId": requestID,
		"messageId":     "msg-" + randomID(),
		"conversationId": "conv-" + randomID(),
		"sessionKey":     "sess-" + randomID(),
		"contents":       contents,
		"properties":     map[string]any{"normalized_response": "true"},
		"token":          token,
		// Required by the gateway's tenant/ADK routing layer (desktop defaults).
		"empid":  "",
		"tenant": "",
		"iaiTag": "phoenix-desktop",
	}
	if system != "" {
		out["systemInstruction"] = system
	}
	// The ADK wire contract is camelCase; snake_case keys are ignored once
	// the request reaches the ADK layer.
	for src, dst := range map[string]string{
		"temperature":      "temperature",
		"top_p":            "topP",
		"stop_sequences":   "stopSequences",
		"tool_choice":      "toolChoice",
		"response_format":  "responseFormat",
		"thinking_budget":  "thinkingBudget",
		"thinking_level":   "thinkingLevel",
		"reasoning_effort": "reasoningEffort",
	} {
		if v, ok := in[src]; ok {
			out[dst] = v
		}
	}
	// Modelos que não suportam raciocínio (DeepSeek, GLM, Qwen, etc.):
	// remove parâmetros de reasoning para evitar falha no gateway da Alibaba.
	if isNonReasoningModel(model) {
		delete(out, "thinkingBudget")
		delete(out, "thinkingLevel")
		delete(out, "reasoningEffort")
	}
	// Zero is not "no limit" for this gateway: maxOutputTokens=0 completes
	// with MAX_TOKENS before any content. Only forward positive limits.
	if v, ok := in["max_tokens"]; ok && positiveNumber(v) {
		out["maxOutputTokens"] = v
	}
	if v, ok := in["tools"]; ok {
		out["tools"] = convertTools(v)
	}
	return out
}

// isNonReasoningModel checks if the model code/id is known to not support reasoning efforts.
func isNonReasoningModel(model string) bool {
	m := strings.ToLower(model)
	// DeepSeek (1Nova-), GLM (1Chronos-), Qwen (1Drift-, 2Drift-), Moonshot (1Synapse-), MiniMax (1Zenith-), Grok (1Helix-G46)
	if strings.Contains(m, "nova") || strings.Contains(m, "deepseek") ||
		strings.Contains(m, "chronos") || strings.Contains(m, "glm") ||
		strings.Contains(m, "drift") || strings.Contains(m, "qwen") ||
		strings.Contains(m, "synapse") || strings.Contains(m, "kimi") ||
		strings.Contains(m, "zenith") || strings.Contains(m, "minimax") ||
		strings.Contains(m, "g46") || strings.Contains(m, "grok") {
		return true
	}
	return false
}


// messageParts converts one OpenAI message into phoenix content parts.
func messageParts(m map[string]any) []any {
	parts := make([]any, 0)
	if reasoning, ok := m["reasoning_content"].(string); ok && strings.TrimSpace(reasoning) != "" {
		parts = append(parts, map[string]any{"text": reasoning, "thought": true})
	}
	content := m["content"]
	if list, ok := content.([]any); ok {
		for _, raw := range list {
			p, _ := raw.(map[string]any)
			switch p["type"] {
			case "text":
				if text, _ := p["text"].(string); text != "" {
					parts = append(parts, map[string]any{"text": text, "thought": false})
				}
			case "image_url":
				if image, ok := p["image_url"].(map[string]any); ok {
					if u, _ := image["url"].(string); strings.HasPrefix(u, "data:") {
						if mime, data, ok := decodeDataURL(u); ok {
							parts = append(parts, map[string]any{"inlineData": map[string]any{"mimeType": mime, "data": data}, "thought": false})
						}
					}
				}
			}
		}
	} else if text := textContent(content); text != "" {
		parts = append(parts, map[string]any{"text": text, "thought": false})
	}
	if calls, ok := m["tool_calls"].([]any); ok {
		for _, raw := range calls {
			call, _ := raw.(map[string]any)
			fn, _ := call["function"].(map[string]any)
			name, _ := fn["name"].(string)
			args, _ := fn["arguments"].(string)
			parts = append(parts, map[string]any{"functionCall": map[string]any{"id": call["id"], "name": name, "argsJson": args}})
		}
	}
	if m["role"] == "tool" {
		parts = append(parts, map[string]any{"functionResponse": map[string]any{
			"id": m["tool_call_id"], "name": m["name"],
			"responseJson": fmt.Sprintf(`{"result":%q}`, textContent(content)),
		}})
	}
	return parts
}

// convertTools maps OpenAI tools (type:function) to the ADK flat format.
func convertTools(v any) any {
	list, ok := v.([]any)
	if !ok {
		return v
	}
	out := make([]any, 0, len(list))
	for _, raw := range list {
		t, _ := raw.(map[string]any)
		fn, _ := t["function"].(map[string]any)
		if len(fn) == 0 {
			out = append(out, raw)
			continue
		}
		params, _ := json.Marshal(fn["parameters"])
		out = append(out, map[string]any{
			"name": fn["name"], "description": fn["description"],
			"parameters_json": string(params),
		})
	}
	return out
}

func textContent(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if list, ok := v.([]any); ok {
		var b strings.Builder
		for _, x := range list {
			if m, ok := x.(map[string]any); ok {
				if s, ok := m["text"].(string); ok {
					b.WriteString(s)
				}
			}
		}
		return b.String()
	}
	return ""
}

func decodeDataURL(value string) (string, string, bool) {
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "data:") || !strings.Contains(parts[0], ";base64") {
		return "", "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64"), parts[1], true
}

func positiveNumber(v any) bool {
	switch n := v.(type) {
	case int:
		return n > 0
	case int64:
		return n > 0
	case float64:
		return n > 0
	case json.Number:
		value, err := n.Int64()
		return err == nil && value > 0
	default:
		return false
	}
}
