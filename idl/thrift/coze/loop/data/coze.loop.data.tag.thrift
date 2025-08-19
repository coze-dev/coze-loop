namespace go coze.loop.data.tag

include "../../../base.thrift"
include "./domain/tag.thrift"
include "domain/dataset.thrift"

struct CreateTagRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt="0")
    2: required string tag_key_name
    3: optional string description
    4: optional tag.TagContentSpec tag_content_spec
    5: optional list<tag.TagValue> tag_values
    6: optional list<tag.TagDomainType> tag_domain_types
    7: optional tag.TagContentType tag_content_type
    8: optional string version

    255: optional base.Base Base
}

struct CreateTagResponse {
    1: optional i64 tag_key_id (api.js_conv="true", go.tag='json:"tag_key_id"')

    255: optional base.BaseResp BaseResp
}

struct UpdateTagRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt="0")
    2: required i64 tag_key_id (api.js_conv="true", go.tag='json:"tag_key_id"', vt.gt="0", api.path="tag_key_id")
    3: required string tag_key_name
    4: optional string description
    5: optional tag.TagContentSpec tag_content_spec
    6: optional list<tag.TagValue> tag_values
    7: optional list<tag.TagDomainType> tag_domain_types
    8: optional tag.TagContentType tag_content_type
    9: optional string version

    255: optional base.Base Base
}

struct UpdateTagResponse {
    255: optional base.BaseResp BaseResp
}

struct BatchUpdateTagStatusRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt="0")
    2: required list<i64> tag_key_ids (api.js_conv="true", go.tag='json:"tag_key_ids"')
    3: required tag.TagStatus to_status

    255: optional base.Base Base
}

struct BatchUpdateTagStatusResponse {
    1: optional map<i64, string> err_info (api.js_conv="true", go.tag='json:"err_info"')

    255: optional base.BaseResp BaseResp
}

struct SearchTagsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt="0")
    2: optional string tag_key_name_like
    3: optional list<string> created_bys
    4: optional list<tag.TagDomainType> domain_types
    5: optional list<tag.TagContentType> content_types
    6: optional string tag_key_name

    /* pagination */
    100: optional i32 page_number (vt.gt="0")
    101: optional i32 page_size (vt.gt="0", vt.le="200")
    102: optional string page_token
    103: optional dataset.OrderBy order_by

    255: optional base.Base Base
}

struct SearchTagsResponse {
    1: optional list<tag.TagInfo> tagInfos

    100: optional string next_page_token
    101: optional i64 total (api.js_conv="true", go.tag='json:"total"')

    255: optional base.BaseResp BaseResp
}

struct GetTagDetailRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt="0")
    2: required i64 tag_key_id (api.js_conv="true", api.path="tag_key_id", go.tag='json:"tag_key_id"', vt.gt="0")

    /* pagination */
    100: optional i32 page_number (vt.gt="0")
    101: optional i32 page_size (vt.gt="0", vt.le="200")
    102: optional string page_token
    103: optional dataset.OrderBy order_by

    255: optional base.Base Base
}

struct GetTagDetailResponse {
    1: optional list<tag.TagInfo> tags

    100: optional string next_page_token
    101: optional i64 total (api.js_conv="true", go.tag='json:"total"')

    255: optional base.BaseResp BaseResp
}

struct GetTagSpecRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt="0")

    255: optional base.Base Base
}

struct GetTagSpecResponse {
    1: optional i64 max_height // 最大高度
    2: optional i64 max_width  // 最大宽度(一层最多有多少个)
    3: optional i64 max_total  // 最多个数(各层加一起总数)

    255: optional base.BaseResp BaseResp
}

struct BatchGetTagsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt="0")
    2: required list<i64> tag_key_ids (api.js_conv="true", go.tag='json:"tag_key_ids"')

    255: optional base.Base Base
}

struct BatchGetTagsResponse {
    1: optional list<tag.TagInfo> tag_info_list

    255: optional base.BaseResp BaseResp
}

service TagService {
    /* Tag */
    // 新增标签
    CreateTagResponse CreateTag(1: CreateTagRequest req) (api.post="/api/data/v1/tags")
    // 更新标签
    UpdateTagResponse UpdateTag(1: UpdateTagRequest req) (api.patch="/api/data/v1/tags/:tag_key_id")
    // 批量更新标签状态
    BatchUpdateTagStatusResponse BatchUpdateTagStatus(1: BatchUpdateTagStatusRequest req) (api.post="/api/data/v1/tags/batch_update_status")
    // 搜索标签
    SearchTagsResponse SearchTags(1: SearchTagsRequest req) (api.post="/api/data/v1/tags/search")
    // 标签详情
    GetTagDetailResponse GetTagDetail(1: GetTagDetailRequest req) (api.post="/api/data/v1/tags/:tag_key_id/detail")
    // 获取标签限制
    GetTagSpecResponse GetTagSpec(1: GetTagSpecRequest req) (api.get="/api/data/v1/tag_spec")
    // 批量获取标签
    BatchGetTagsResponse BatchGetTags(1: BatchGetTagsRequest req) (api.post="/api/data/v1/tags/batch_get")
}