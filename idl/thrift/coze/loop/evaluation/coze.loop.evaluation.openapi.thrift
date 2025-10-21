namespace go coze.loop.evaluation.openapi

include "../../../base.thrift"
include "coze.loop.evaluation.spi.thrift"

struct ReportEvalTargetInvokeResultRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional i64 invoke_id (api.js_conv="true", go.tag = 'json:"invoke_id"')
    3: optional coze.loop.evaluation.spi.InvokeEvalTargetStatus status
    4: optional string callee

    // set output if status=SUCCESS
    10: optional coze.loop.evaluation.spi.InvokeEvalTargetOutput output
    // set output if status=SUCCESS
    11: optional coze.loop.evaluation.spi.InvokeEvalTargetUsage usage
    // set error_message if status=FAILED
    20: optional string error_message

    255: optional base.Base Base
}

struct ReportEvalTargetInvokeResultResponse {
    255: base.BaseResp BaseResp
}

service EvaluationOpenAPIService {
    ReportEvalTargetInvokeResultResponse ReportEvalTargetInvokeResult(1: ReportEvalTargetInvokeResultRequest req) (api.category="openapi", api.post = "/v1/loop/eval_targets/result")
}
