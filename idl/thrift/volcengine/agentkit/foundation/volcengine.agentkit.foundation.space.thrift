namespace go volcengine.agentkit.foundation.space

include "../../../base.thrift"
include "../../../coze/loop/foundation/domain/space.thrift"
include "../../../coze/loop/foundation/coze.loop.foundation.space.thrift"

struct EnsureMappingSpaceRequest {
    1: required string identifier (vt.min_size = '1')
    2: required i64 app_id (vt.gt = '0')
    10: optional bool include_space
    11: optional bool skip_resource_init
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base Base
}

struct GetMappingSpaceRequest {
    1: required string identifier (vt.min_size = '1')
    2: required i64 app_id (vt.gt = '0')
    10: optional bool include_space
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base Base
}

service SpaceService {

    coze.loop.foundation.space.EnsureMappingSpaceResponse EnsureMappingSpace(1: EnsureMappingSpaceRequest request) (
        api.post = '/api/space_manage/v1/EnsureMappingSpace', api.category = 'loopfoundation', api.tag = 'volc-agentkit-gen', api.top_operation_type = 'create', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.foundation.space.GetMappingSpaceResponse GetMappingSpace(1: GetMappingSpaceRequest request) (
        api.post = '/api/space_manage/v1/GetMappingSpace', api.category = 'loopfoundation', api.tag = 'volc-agentkit-gen', api.top_operation_type = 'query', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

}
