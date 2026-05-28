namespace go coze.loop.observability.domain.trace

struct Trace {
    1: optional string trace_id
    2: optional TokenCost tokens
    3: optional i64 size
}

struct TokenCost {
    1: required i64 input_token (api.js_conv='true', go.tag='json:"input_token"')
    2: required i64 output_token (api.js_conv='true', go.tag='json:"output_token"')
}