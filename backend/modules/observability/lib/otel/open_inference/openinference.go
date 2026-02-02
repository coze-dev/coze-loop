// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package open_inference

import (
	"encoding/json"
	"fmt"
)

type Literal string

const (
	TextLiteral     Literal = "text"
	ImageLiteral    Literal = "image"
	ImageUrlLiteral Literal = "image_url"
)

type ModelMessagePartType string

var (
	ModelMessagePartTypeText  ModelMessagePartType = "text"
	ModelMessagePartTypeImage ModelMessagePartType = "image_url"
)

func ConvertToModelInput(input interface{}) (interface{}, error) {
	// check slice
	inputSlice, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("input is not a slice")
	}

	messages := make([]interface{}, 0, len(inputSlice))
	for _, item := range inputSlice {
		// check map
		msgSurface, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		msg, ok := msgSurface["message"].(map[string]interface{})
		if !ok {
			msg = msgSurface // maybe no message key, it has been a raw message
		}

		modelMsg := convertModelMsg(msg)
		messages = append(messages, modelMsg)
	}

	modelInput := map[string]interface{}{
		"messages": messages,
	}

	return modelInput, nil
}

func convertModelMsg(msg map[string]interface{}) map[string]interface{} {
	modelMsg := map[string]interface{}{
		"role": msg["role"],
	}

	// content
	if content, ok := msg["content"].(string); ok {
		modelMsg["content"] = content
	}
	if c, ok := msg["reasoning_content"].(string); ok {
		modelMsg["reasoning_content"] = c
	}

	// contents or parts
	var contents []interface{}
	partsKey := []string{"contents", "content", "parts"}
	for _, key := range partsKey {
		if parts, ok := msg[key].([]interface{}); ok && len(parts) > 0 {
			contents = parts
			break
		}
	}
	if len(contents) > 0 {
		parts := make([]interface{}, 0, len(contents))
		for _, content := range contents {
			if mc, ok := content.(map[string]interface{}); ok {
				mcContent, ok := mc["message_content"].(map[string]interface{})
				if !ok {
					mcContent = mc // maybe no message_content key, it has been a raw message
				}
				typ, _ := mcContent["type"]
				text, _ := mcContent["text"]
				image, _ := mcContent["image_url"]
				part := map[string]interface{}{}
				switch typ {
				case string(TextLiteral):
					part["type"] = string(ModelMessagePartTypeText)
					part["text"] = text
				case string(ImageLiteral), string(ImageUrlLiteral):
					part["type"] = string(ModelMessagePartTypeImage)
					imageMap, ok := image.(map[string]interface{})
					if ok {
						url, _ := imageMap["url"]
						part["image_url"] = map[string]interface{}{"url": url}
					}
				default:
				}
				parts = append(parts, part)
			}
		}
		if len(parts) > 0 {
			modelMsg["parts"] = parts
		}
	}

	// tool_calls
	if toolCalls, ok := msg["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
		calls := make([]interface{}, 0, len(toolCalls))
		for _, call := range toolCalls {
			if tc, ok := call.(map[string]interface{}); ok {
				// get tool_call
				toolCall, ok := tc["tool_call"].(map[string]interface{})
				if !ok {
					toolCall = tc // maybe no tool_call key, it has been a raw tool_call
				}
				// get function from tool_call
				function, ok := toolCall["function"].(map[string]interface{})
				if !ok {
					continue
				}
				id, _ := toolCall["id"]
				modelCall := map[string]interface{}{
					"type": "function",
					"id":   id,
					"function": map[string]interface{}{
						"name": function["name"],
					},
				}
				if args, ok := function["arguments"].(string); ok {
					modelCall["function"].(map[string]interface{})["arguments"] = args
				}
				calls = append(calls, modelCall)
			}
		}
		if len(calls) > 0 {
			modelMsg["tool_calls"] = calls
		}
	}

	return modelMsg
}

func ConvertToModelOutput(input interface{}) (interface{}, error) {
	// check slice
	surfaces, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("input is not a slice")
	}

	choices := make([]interface{}, 0, len(surfaces))
	for _, item := range surfaces {
		// check map
		surface, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		msg, ok := surface["message"].(map[string]interface{})
		if !ok {
			msg = surface // maybe no message key, it has been a raw message
		}

		modelMsg := convertModelMsg(msg)
		choice := map[string]interface{}{
			"message": modelMsg,
		}

		choices = append(choices, choice)
	}

	modelOutput := map[string]interface{}{
		"choices": choices,
	}

	return modelOutput, nil
}

func AddTools2ModelInput(input interface{}, tools interface{}) (interface{}, error) {
	modelInput, ok := input.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("input is not a map")
	}

	toolsSlice, ok := tools.([]interface{})
	if !ok {
		return modelInput, nil
	}

	modelTools := make([]interface{}, 0, len(toolsSlice))

	for _, tool := range toolsSlice {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		toolData, ok := toolMap["tool"].(map[string]interface{})
		if !ok {
			continue
		}

		schemaStr, ok := toolData["json_schema"].(string)
		if !ok {
			continue
		}

		var schema struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		}
		if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
			continue
		}

		modelTool := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        schema.Name,
				"description": schema.Description,
				"parameters":  schema.Parameters,
			},
		}

		modelTools = append(modelTools, modelTool)
	}

	if len(modelTools) > 0 {
		modelInput["tools"] = modelTools
	}

	return modelInput, nil
}
