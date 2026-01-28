namespace go coze.loop.foundation.user

include "../../../base.thrift"
include "./domain/user.thrift"

struct UserRegisterRequest {
    1: optional string email
    2: optional string password

    255: optional base.Base Base
}

struct UserRegisterResponse {
    1: optional user.UserInfoDetail user_info
    2: optional string token (api.cookie="sessionid,Value")
    3: optional i64 expire_time (api.cookie="sessionid,Expires")

    255: optional base.BaseResp  BaseResp
}

struct LoginByPasswordRequest {
    1: optional string email
    2: optional string password

    255: optional base.Base Base
}

struct LoginByPasswordResponse {
    1: optional user.UserInfoDetail user_info
    2: optional string token (api.cookie="sessionid,Value")
    3: optional i64 expire_time (api.cookie="sessionid,Expires")

    255: optional base.BaseResp  BaseResp
}

struct LogoutRequest {
    1: optional string token (api.cookie="sessionid")
    255: optional base.Base Base
}
struct LogoutResponse {
    255: optional base.BaseResp  BaseResp
}

struct ResetPasswordRequest {
    1: optional string email
    2: optional string password
    3: optional string code
    255: optional base.Base Base
}
struct ResetPasswordResponse {
    255: optional base.BaseResp  BaseResp
}

struct GetUserInfoByTokenRequest {
    1: optional string token (api.cookie="sessionid")

    255: optional base.Base Base
}
struct GetUserInfoByTokenResponse {
    1: optional user.UserInfoDetail user_info

    255: optional base.BaseResp  BaseResp
}

struct ModifyUserProfileRequest {
    1: optional string user_id (api.path="user_id")
    2: optional string name             // 用户唯一名称
    3: optional string nick_name        // 用户昵称
    4: optional string description      // 用户描述
    5: optional string avatar_uri       // 用户头像URI

    255: optional base.Base Base
}
struct ModifyUserProfileResponse {
    1: optional user.UserInfoDetail user_info

    255: optional base.BaseResp  BaseResp
}

struct GetUserInfoRequest {
    1: optional string user_id

    255: optional base.Base Base
}

struct GetUserInfoResponse {
    1: optional user.UserInfoDetail user_info

    255: optional base.BaseResp  BaseResp
}

struct MGetUserInfoRequest {
    1: optional list<string> user_ids

    255: optional base.Base Base
}

struct MGetUserInfoResponse {
    1: optional list<user.UserInfoDetail> user_infos

    255: base.BaseResp  BaseResp
}

struct QueryUserInfoResponse {
    1: optional list<user.UserInfoDetail> user_infos // 用户信息列表
    2: optional string page_token // 分页Token
    3: optional bool has_more // 是否还有下一页

    255: optional base.BaseResp  BaseResp
}

service UserService {
    // 用户注册相关接口
    UserRegisterResponse Register(1: UserRegisterRequest request) (api.post = "/api/foundation/v1/users/register")
    ResetPasswordResponse ResetPassword(1: ResetPasswordRequest request) (api.post = "/api/foundation/v1/users/reset_password")

    // 用户登陆相关接口
    LoginByPasswordResponse LoginByPassword(1: LoginByPasswordRequest request) (api.post = "/api/foundation/v1/users/login_by_password")
    LogoutResponse Logout(1: LogoutRequest request) (api.post = "/api/foundation/v1/users/logout")

    // 修改用户资料相关接口
    ModifyUserProfileResponse ModifyUserProfile(1: ModifyUserProfileRequest request) (api.put = "/api/foundation/v1/users/:user_id/update_profile")

    // 基于登陆态获取用户信息相关接口
    GetUserInfoByTokenResponse GetUserInfoByToken(1: GetUserInfoByTokenRequest request) (api.get = "/api/foundation/v1/users/session")

    // 获取用户信息
    GetUserInfoResponse GetUserInfo(1: GetUserInfoRequest request)
    // 批量获取用户信息
    MGetUserInfoResponse MGetUserInfo(1: MGetUserInfoRequest request)
}