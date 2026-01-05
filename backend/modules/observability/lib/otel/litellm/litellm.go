// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package litellm

import (
	"encoding/json"

	"fmt"
)

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
		name, _ := toolMap["name"].(string)
		parametersStr, _ := toolMap["parameters"].(string)
		parametersStrObj := map[string]interface{}{}
		if parametersStr != "" {
			if err := json.Unmarshal([]byte(parametersStr), &parametersStrObj); err != nil {
			}
		}
		description, _ := toolMap["description"].(string)
		modelTool := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        name,
				"description": description,
				"parameters":  parametersStrObj,
			},
		}

		modelTools = append(modelTools, modelTool)
	}

	if len(modelTools) > 0 {
		modelInput["tools"] = modelTools
	}

	return modelInput, nil
}
