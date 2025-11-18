namespace go coze.loop.llm.domain.common

typedef string Scenario (ts.enum="true")
const Scenario scenario_default = "default"
const Scenario scenario_prompt_debug = "prompt_debug"
const Scenario scenario_prompt_as_a_service = "prompt_as_a_service"
const Scenario scenario_eval_target = "eval_target"
const Scenario scenario_evaluator = "evaluator"
const Scenario scenario_data_synthesis = "data_synthesis"
