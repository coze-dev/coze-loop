namespace go stone.fornax.ml_flow.domain.tag

enum TagStatus {
    Active = 1,         // 启用
    Inactive = 2,       // 禁用
    Deprecated = 99     // 弃用,旧版本状态
}

enum TagType {
    Tag = 1,    // 标签类型
    Option = 2  // 单选类型
}

enum OperationType {
    Create = 1, // 创建
    Update = 2, // 更新
    Delete = 3  // 删除
}

enum ChangeTargetType {
    Tag = 1,                // tag
    TagName = 2,           // tag name
    TagDescription = 3,    // tag description
    TagStatus = 4,         // tag status
    TagType = 5,           // tag type
    TagValueName = 6,     // tag value name
    TagValueStatus = 7    // tag value status
    TagContentType = 8,   // tag content type
}

enum TagContentType {
    Categorical = 1,        // 分类
    Boolean = 2,            // 布尔
    ContinuousNumber = 3,   // 连续分值
    FreeText = 4,           // 自由文本
}

enum TagDomainType {
    Data = 1,               // 数据基座
    Observe = 2,            // 观测
    Evaluation = 3,         // 评测
}

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
    1: optional i64 ID (api.js_conv="str")
    2: optional i32 appID
    3: optional i64 spaceID (api.js_conv="str")
    4: optional i32 versionNum                              // 数字版本号
    5: optional string version                              // SemVer 三段式版本号
    6: optional i64 tagKeyID (api.js_conv="str")            // tag key id
    7: optional string tagKeyName                           // tag key name
    8: optional string description                          // 描述
    9: optional TagStatus status                            // 状态，启用active、禁用inactive、弃用deprecated(最新版之前的版本的状态)
    10: optional TagType tagType                            // 类型: tag: 标签管理中的标签类型; option: 临时单选类型
    11: optional i64 parentTagKeyID (api.js_conv="str")
    12: optional list<TagValue> tagValues                   // 标签值
    13: optional list<ChangeLog> changeLogs                     // 变更历史
    14: optional TagContentType content_type                                                // 内容类型
    15: optional TagContentSpec content_spec                                                // 内容约束
    16: optional list<TagDomainType> domain_type_list                                          // 应用领域

    101: optional string createdBy
    102: optional i64 createdAt (api.js_conv="str")
    103: optional string updatedBy
    104: optional i64 updatedAt (api.js_conv="str")
}

struct TagValue {
    1: optional i64 ID (api.js_conv="str")                  // 创建时不传
    2: optional i32 appID                                   // 创建时不传
    3: optional i64 spaceID (api.js_conv="str")             // 创建时不传
    4: optional i64 tagKeyID (api.js_conv="str")            // 创建时不传
    5: optional i64 tagValueID (api.js_conv="str")          // 创建时不传
    6: optional string tagValueName                         // 标签值
    7: optional string description                          // 描述
    8: optional TagStatus status                            // 状态，启用active、禁用inactive、弃用deprecated(最新版之前的版本的状态)
    9: optional i32 versionNum                              // 数字版本号
    10: optional i64 parentValueID (api.js_conv="str")      // 父标签选项的ID
    11: optional list<TagValue> children                    // 子标签
    12: optional bool isSystem // 是否是系统标签而非用户标签

    100: optional string createdBy
    101: optional i64 createAt(api.js_conv="str")
    102: optional string updatedBy
    103: optional i64 updatedAt(api.js_conv="str")
}

struct ChangeLog{
    1: optional ChangeTargetType target     // 变更的属性
    2: optional OperationType operation     // 变更类型: create, update, delete
    3: optional string beforeValue          // 变更前的值
    4: optional string afterValue           // 变更后的值
    5: optional string targetValue          // 变更属性的值：如果是标签选项变更，该值为变更属选项值名字
}