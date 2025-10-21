namespace go coze.loop.evaluation.domain_openapi.eval_target
include "common.thrift"

typedef string EvalTargetType(ts.enum="true")
const EvalTargetType EvalTargetType_CozeBot = "coze_bot"
const EvalTargetType EvalTargetType_CozeLoopPrompt = "coze_loop_prompt"
const EvalTargetType EvalTargetType_Trace = "trace"
const EvalTargetType EvalTargetType_CozeWorkflow = "coze_workflow"
const EvalTargetType EvalTargetType_VolcengineAgent = "volcengine_agent"


typedef string CozeBotInfoType(ts.enum="true")
const CozeBotInfoType CozeBotInfoType_DraftBot = "draft_bot"
const CozeBotInfoType CozeBotInfoType_ProductBot = "product_bot"


typedef string EvalTargetRunStatus(ts.enum="true")
const EvalTargetRunStatus EvalTargetRunStatus_Success = "success"
const EvalTargetRunStatus EvalTargetRunStatus_Fail = "fail"


struct CreateEvalTargetParam {
    1: optional string source_target_id
    2: optional string source_target_version
    3: optional EvalTargetType eval_target_type
    4: optional CozeBotInfoType bot_info_type
    5: optional string bot_publish_version // 如果是发布版本则需要填充这个字段
}

struct EvalTargetRecord  {
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"')// 评估记录ID
    3: optional i64 target_id (api.js_conv='true', go.tag='json:"target_id"')
    4: optional i64 target_version_id (api.js_conv='true', go.tag='json:"target_version_id"')
    6: optional i64 item_id (api.js_conv='true', go.tag='json:"item_id"') // 评测集数据项ID
    7: optional i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"') // 评测集数据项轮次ID
    10: optional EvalTargetInputData eval_target_input_data // 输入数据
    11: optional EvalTargetOutputData eval_target_output_data  // 输出数据
    12: optional EvalTargetRunStatus status

    100: optional common.BaseInfo base_info
}

struct EvalTargetInputData {
    1: optional list<common.Message> history_messages      // 历史会话记录
    2: optional map <string, common.Content> input_fields       // 变量
    3: optional map<string, string> ext
}

struct EvalTargetOutputData {
    1: optional map<string, common.Content> output_fields           // 变量
    2: optional EvalTargetUsage eval_target_usage             // 运行消耗
    3: optional EvalTargetRunError eval_target_run_error         // 运行报错
    4: optional i64 time_consuming_ms (api.js_conv='true', go.tag='json:\"time_consuming_ms\"') // 运行耗时
}

struct EvalTargetUsage {
    1: i64 input_tokens (api.js_conv='true', go.tag='json:\"input_tokens\"')
    2: i64 output_tokens (api.js_conv='true', go.tag='json:\"output_tokens\"')
}

struct EvalTargetRunError {
    1: optional i32 code (go.tag='json:\"code\"')
    2: optional string message (go.tag='json:\"message\"')
}
