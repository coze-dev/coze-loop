namespace go coze.loop.evaluation.domain_openapi.eval_target
include "common.thrift"

typedef string EvalTargetType(ts.enum="true")
const EvalTargetType EvalTargetType_CozeBot = "coze_bot"
const EvalTargetType EvalTargetType_CozeLoopPrompt = "coze_loop_prompt"
const EvalTargetType EvalTargetType_Trace = "trace"
const EvalTargetType EvalTargetType_CozeWorkflow = "coze_workflow"
const EvalTargetType EvalTargetType_VolcengineAgent = "volcengine_agent"
const EvalTargetType EvalTargetType_CustomRPCServer = "custom_rpc_server"


typedef string CozeBotInfoType(ts.enum="true")
const CozeBotInfoType CozeBotInfoType_DraftBot = "draft_bot"
const CozeBotInfoType CozeBotInfoType_ProductBot = "product_bot"


typedef string EvalTargetRunStatus(ts.enum="true")
const EvalTargetRunStatus EvalTargetRunStatus_Success = "success"
const EvalTargetRunStatus EvalTargetRunStatus_Fail = "fail"



struct EvalTargetRecord  {
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"')// 评估记录ID
    2: optional i64 target_id (api.js_conv='true', go.tag='json:"target_id"')
    3: optional i64 target_version_id (api.js_conv='true', go.tag='json:"target_version_id"')
    4: optional i64 item_id (api.js_conv='true', go.tag='json:"item_id"') // 评测集数据项ID
    5: optional i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"') // 评测集数据项轮次ID

    20: optional EvalTargetOutputData eval_target_output_data  // 输出数据
    21: optional EvalTargetRunStatus status

    100: optional common.BaseInfo base_info
}

struct EvalTargetOutputData {
    1: optional map<string, common.Content> output_fields           // 输出字段，目前key只支持actual_output
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