namespace go coze.loop.observability.domain.view

include "common.thrift"

enum Scope {
    TraceList        = 1  // trace_list（列表页）
    TraceDetailTree  = 2  // trace_detail_tree（详情页-tree）
    TraceDetailChat  = 3  // trace_detail_chat（详情页-chat）
}

struct View {
    1: required i64 id (api.js_conv="true", go.tag='json:"id"')
    2: optional string enterprise_id
    3: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    4: required string view_name
    5: optional common.PlatformType platform_type
    6: optional common.SpanListType spanList_type
    7: required string filters
    8: required bool is_system
    9: optional Scope scope
}

