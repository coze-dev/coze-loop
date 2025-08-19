namespace go coze.loop.foundation.domain.user


// UserInfoDetail 用户详细信息，包含姓名、头像等
struct UserInfoDetail {
    1: optional string name                     // 唯一名称
    2: optional string nick_name                // 用户昵称
    3: optional string avatar_url               // 用户头像url
    4: optional string email                    // 用户邮箱
    5: optional string mobile                   // 手机号
    6: optional string user_id                  // 用户在租户内的唯一标识
}

typedef string UserStatus (ts.enum="true")
const UserStatus active = "active"
const UserStatus deactivated = "deactivated"
const UserStatus offboarded = "offboarded"