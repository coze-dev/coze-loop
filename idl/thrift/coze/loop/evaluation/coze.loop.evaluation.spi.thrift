namespace go coze.loop.evaluation.spi
include "../../../base.thrift"

struct SearchEvalTargetRequest {
    1: optional i64 workspace_id // 空间id
    2: optional string keyword // 搜索关键字，如需使用请用户自行实现

    20: optional map<string, string> ext // 扩展字段：目前会透传regoin和空间id信息，key名如下：search_region、search_space_id

    100: optional i32 page_size
    101: optional string page_token

    255: optional base.Base Base
}

struct SearchEvalTargetResponse {
    1: optional list<CustomEvalTarget> custom_eval_targets

    100: optional string next_page_token
    101: optional bool has_more

    255: base.BaseResp BaseResp (api.none="true")
}

struct CustomEvalTarget {
    1: optional string id // 唯一键，平台不消费，仅做透传
    2: optional string name    // 名称，平台用于展示在对象搜索下拉列表
    3: optional string avatar_url    // 头像url，平台用于展示在对象搜索下拉列表
}

struct InvokeEvalTargetRequest {
    1: optional i64 workspace_id    // 空间id
    2: optional InvokeEvalTargetInput input  // 输入信息
    3: optional CustomEvalTarget custom_eval_target    // 如果创建实验时选了二级对象，则会透传search接口返回的二级对象信息

    255: optional base.Base Base (api.none="true");
}

struct InvokeEvalTargetResponse {
    1: optional InvokeEvalTargetStatus status
    // set output if status=SUCCESS
    2: optional InvokeEvalTargetOutput output
    // set usage if status=SUCCESS
    3: optional InvokeEvalTargetUsage usage
    // set error_message if status=FAILED
    10: optional string error_message

    255: base.BaseResp BaseResp (api.none="true")
}

struct InvokeEvalTargetInput {
    1: optional map<string, Content> eval_set_fields // 评测集字段信息，key=评测集列名,value=评测集列值

    20: optional map<string, string> ext   // 扩展字段，动态参数会通过ext字段传递
}

enum InvokeEvalTargetStatus {
    UNKNOWN = 0
    SUCCESS = 1
    FAILED = 2
}

// 新增
struct InvokeEvalTargetOutput {
    1: optional Content actual_output

    20: optional map<string, string> ext     // 扩展字段，用户如果想返回一些额外信息可以塞在这个字段
}

struct Content {
    1: optional ContentType content_type    // 类型
    10: optional string text // 当content_type=text，则从此字段中取值
    11: optional Image image    // 当content_type=image，则从此字段中取图片信息
    12: optional list<Content> multi_part   // 当content_type=multi_part，则从此字段遍历获取多模态的值
}

typedef string ContentType(ts.enum="true")
const ContentType ContentType_Text = "text" // 文本类型：string、integer、float、boolean、object、array都属于文本类型
const ContentType ContentType_Image = "image"
const ContentType ContentType_MultiPart = "multi_part"  // 多模态，例如图+文

struct Image {
    1: optional string url
}

struct InvokeEvalTargetUsage {
    1: optional i64 input_tokens     // 输入token消耗
    2: optional i64 output_tokens    // 输出token消耗
}

struct AsyncInvokeEvalTargetRequest {
    1: optional i64 workspace_id
    2: optional i64 invoke_id  // 执行id，传递给自定义对象，在回传结果时透传
    4: optional InvokeEvalTargetInput input  // 执行输入信息
    5: optional CustomEvalTarget custom_eval_target    // 如果创建实验时选了二级对象，则会透传二级对象信息

    255: optional base.Base Base (api.none="true");
}

struct AsyncInvokeEvalTargetResponse {
    255: base.BaseResp BaseResp (api.none="true")
}

// the run status enumerate for custom evaluator
enum InvokeEvaluatorRunStatus {
    UNKNOWN = 0
    SUCCESS = 1
    FAILED = 2
}

// the custom evaluator identity and parameter information
struct InvokeCustomEvaluator {
    1: optional string provider_evaluator_code // provider-side evaluator identity code
}

// the input data structure for custom evaluator
struct InvokeEvaluatorInputData {
    1: optional map<string, Content> input_fields    // key-value structure of input variables required by the evaluator
    2: optional map<string, Content> evaluate_dataset_fields      // key-value structure of dataset variables required by the evaluator
    3: optional map<string, Content> evaluate_target_output_fields // key-value structure of target output variables required by the evaluator

    20: optional map<string, string> ext // dynamic fields for inject parameters
}

// the output data structure for custom evaluator
struct InvokeEvaluatorOutputData {
    1: optional InvokeEvaluatorResult evaluator_result
    2: optional InvokeEvaluatorUsage evaluator_usage
    3: optional InvokeEvaluatorRunError evaluator_run_error
}

// the result data structure for custom evaluator
struct InvokeEvaluatorResult {
    1: optional double score
    2: optional string reasoning
}

// the usage data structure for custom evaluator
struct InvokeEvaluatorUsage {
    1: optional i64 input_tokens
    2: optional i64 output_tokens
}

// the error data structure for custom evaluator
struct InvokeEvaluatorRunError {
    1: optional i32 code
    2: optional string message
}

// invoke custom evaluator request
struct InvokeEvaluatorRequest {
    1: optional i64 workspace_id
    2: optional InvokeCustomEvaluator evaluator
    3: optional InvokeEvaluatorInputData input_data

    255: optional base.Base Base
}

// invoke custom evaluator response
struct InvokeEvaluatorResponse {
    1: optional InvokeEvaluatorOutputData output_data
    2: optional InvokeEvaluatorRunStatus status

    255: base.BaseResp BaseResp
}

service EvaluationSPIService {
    SearchEvalTargetResponse SearchEvalTarget(1: SearchEvalTargetRequest req)   // 搜索评测对象
    InvokeEvalTargetResponse InvokeEvalTarget(1: InvokeEvalTargetRequest req)   // 执行
    AsyncInvokeEvalTargetResponse AsyncInvokeEvalTarget(1: AsyncInvokeEvalTargetRequest req)    // 异步执行

    // invoke custom evaluator
    InvokeEvaluatorResponse InvokeEvaluator(1: InvokeEvaluatorRequest req)
}
