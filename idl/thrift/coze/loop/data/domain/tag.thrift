namespace go coze.loop.data.domain.tag

include "./common.thrift"

typedef string TagStatus (ts.enum="true")           // 标签状态
const TagStatus TagStatus_Active = "active"         // 启用
const TagStatus TagStatus_Inactive = "inactive"     // 禁用
const TagStatus TagStatus_Deprecated = "deprecated" // 旧版本状态

typedef string TagType (ts.enum="true")             // 标签类型
const TagType TagType_Tag = "tag"                   // 标签
const TagType TagType_Option = "option"             // 单选类型,不在标签管理中

typedef string OperationType (ts.enum="true")       // 操作类型
const OperationType OperationType_Create = "create" // 创建
const OperationType OperationType_Update = "update" // 更新
const OperationType OperationType_Delete = "delete" // 删除

typedef string ChangeTargetType (ts.enum="true")                            // 变更对象
const ChangeTargetType ChangeTargetType_Tag = "tag"                         // 标签
const ChangeTargetType ChangeTargetType_TagName = "tag_name"                // 标签名
const ChangeTargetType ChangeTargetType_TagDescription = "tag_description"  // 标签描述
const ChangeTargetType ChangeTargetType_TagStatus = "tag_status"            // 标签状态
const ChangeTargetType ChangeTargetType_TagType = "tag_type"                // 标签类型
const ChangeTargetType ChangeTargetType_TagContentType = "tag_content_type" // 标签内容类型
const ChangeTargetType ChangeTargetType_TagValueName = "tag_value_name"     // 标签选项值
const ChangeTargetType ChangeTargetType_TagValueStatus = "tag_value_status" // 标签选项状态

typedef string TagDomainType (ts.enum="true")
const TagDomainType TagDomainType_Data = "data"               // 数据基座
const TagDomainType TagDomainType_Observe = "observe"         // 观测
const TagDomainType TagDomainType_Evaluation = "evaluation"   // 评测

typedef string TagContentType (ts.enum="true")
const TagContentType TagContentType_Categorical = "categorical"             // 分类标签
const TagContentType TagContentType_Boolean = "boolean"                     // 布尔标签
const TagContentType TagContentType_ContinuousNumber = "continuous_number"  // 连续分支类型
const TagContentType TagContentType_FreeText = "free_text"                  // 自由文本

struct TagContentSpec {
    1: optional ContinuousNumberSpec continuous_number_spec
}

struct ContinuousNumberSpec {
    1: optional double min_value
    2: optional string min_value_description
    3: optional double max_value
    4: optional string max_value_description
}

struct TagInfo {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"')
    2: optional i32 appID
    3: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    4: optional i32 version_num                                                             // 数字版本号
    5: optional string version                                                              // SemVer 三段式版本号
    6: optional i64 tag_key_id (api.js_conv="true", go.tag='json:"tag_key_id"')             // tag key id
    7: optional string tag_key_name                                                         // tag key name
    8: optional string description                                                          // 描述
    9: optional TagStatus status                                                            // 状态，启用active、禁用inactive、弃用deprecated(最新版之前的版本的状态)
    10: optional TagType tag_type                                                           // 类型: tag: 标签管理中的标签类型; option: 临时单选类型
    11: optional i64 parent_tag_key_id (api.js_conv="true", go.tag='json:"parent_tag_key_id"')
    12: optional list<TagValue> tag_values                                                  // 标签值
    13: optional list<ChangeLog> change_logs                                                // 变更历史
    14: optional TagContentType content_type                                                // 内容类型
    15: optional TagContentSpec content_spec                                                // 内容约束
    16: optional list<TagDomainType> domain_type_list                                          // 应用领域

    // 基础信息
    100: optional common.BaseInfo base_info
}

struct TagValue {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"')                                      // 主键
    2: optional i32 app_id
    3: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    4: optional i64 tag_key_id (api.js_conv="true", go.tag='json:"tag_key_id"')                      // tag_key_id
    5: optional i64 tag_value_id (api.js_conv="true", go.tag='json:"tag_value_id"')                  // tag_value_id
    6: optional string tag_value_name                                                           // 标签值
    7: optional string description                                                              // 描述
    8: optional TagStatus status                                                                // 状态
    9: optional i32 version_num                                                                 // 数字版本号
    10: optional i64 parent_tag_value_id (api.js_conv="true", go.tag='json:"parent_tag_value_id"')   // 父标签选项的ID
    11: optional list<TagValue> children                                                        // 子标签
    12: optional bool is_system                                                                 // 是否是系统标签而非用户标签

    // 基础信息
    100: optional common.BaseInfo base_info
}

struct ChangeLog{
    1: optional ChangeTargetType target     // 变更的属性
    2: optional OperationType operation     // 变更类型: create, update, delete
    3: optional string before_value         // 变更前的值
    4: optional string after_value          // 变更后的值
    5: optional string target_value         // 变更属性的值：如果是标签选项变更，该值为变更属选项值名字
}