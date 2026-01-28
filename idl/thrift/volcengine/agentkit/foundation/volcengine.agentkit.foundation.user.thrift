namespace go volcengine.agentkit.foundation.user

include "../../../base.thrift"
include "../../../coze/loop/foundation/domain/user.thrift"
include "../../../coze/loop/foundation/coze.loop.foundation.user.thrift"

struct GetUserInfoRequest {
    1: optional string user_id
    2: optional string user_name
    3: optional i32 app_id
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base Base
}

struct MGetUserInfoRequest {
    1: optional list<string> user_ids
    2: optional list<string> user_names
    3: optional list<string> ext_user_ids
    4: optional i32 app_id
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base Base
}

struct QueryUserInfoRequest {
    1: required string name_like
    2: optional i32 page_size (vt.ge = '1', vt.le = '200')
    3: optional string page_token
    4: optional i32 app_id
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base Base
}

service UserService {

    coze.loop.foundation.user.GetUserInfoResponse GetUserInfo(1: GetUserInfoRequest request) (
        api.post = '/api/user/v1/GetUserInfo', api.category = 'loopfoundation', api.tag = 'volc-agentkit-gen', api.top_operation_type = 'query', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.foundation.user.MGetUserInfoResponse MGetUserInfo(1: MGetUserInfoRequest request) (
        api.post = '/api/user/v1/MGetUserInfo', api.category = 'loopfoundation', api.tag = 'volc-agentkit-gen', api.top_operation_type = 'query', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.foundation.user.QueryUserInfoResponse QueryUserInfo(1: QueryUserInfoRequest request) (
        api.post = '/api/user/v1/QueryUserInfo', api.category = 'loopfoundation', api.tag = 'volc-agentkit-gen', api.top_operation_type = 'query', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

}
