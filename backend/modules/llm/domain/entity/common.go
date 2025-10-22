// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type Scenario string

const (
	ScenarioDefault          Scenario = "default"
	ScenarioPromptDebug      Scenario = "prompt_debug"
	ScenarioEvalTarget       Scenario = "eval_target"
	ScenarioEvaluator        Scenario = "evaluator"
	ScenarioPromptAsAService Scenario = "prompt_as_a_service" // ptaas
	ScenarioDataSynthesis    Scenario = "data_synthesis"
)

func ScenarioValue(scenario *Scenario) Scenario {
	if scenario == nil {
		return ScenarioDefault
	}
	return *scenario
}
