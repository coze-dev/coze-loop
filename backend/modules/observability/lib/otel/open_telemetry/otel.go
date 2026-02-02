// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package open_telemetry

import "fmt"

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

		function, ok := toolMap["function"].(map[string]interface{})
		if !ok {
			function = toolMap // maybe no function key, it has been a raw function
		}

		name, _ := function["name"]
		if name == nil {
			name = ""
		}
		description, _ := function["description"]
		if description == nil {
			description = ""
		}
		parameters, _ := function["parameters"]
		if parameters == nil {
			parameters = "{}"
		}
		modelTool := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        name,
				"description": description,
				"parameters":  parameters,
			},
		}

		modelTools = append(modelTools, modelTool)
	}

	if len(modelTools) > 0 {
		modelInput["tools"] = modelTools
	}

	return modelInput, nil
}
