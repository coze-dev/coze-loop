namespace go coze.loop.data.domain.common

// 用户信息
struct UserInfo {
	1: optional string name         // 姓名
	2: optional string en_name      // 英文名称
	3: optional string avatar_url   // 用户头像url
	4: optional string avatar_thumb // 72 * 72 头像
	5: optional string open_id      // 用户应用内唯一标识
	6: optional string union_id     // 用户应用开发商内唯一标识
    8: optional string user_id      // 用户在租户内的唯一标识
    9: optional string email        // 用户邮箱
}

// 基础信息
struct BaseInfo {
    1: optional UserInfo created_by
    2: optional UserInfo updated_by
    3: optional i64 created_at (api.js_conv="true", go.tag = 'json:"created_at"')
    4: optional i64 updated_at (api.js_conv="true", go.tag = 'json:"updated_at"')
}