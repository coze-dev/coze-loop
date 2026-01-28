namespace go volcengine.agentkit.foundation

include "volcengine.agentkit.foundation.space.thrift"
include "volcengine.agentkit.foundation.user.thrift"

service SpaceService extends volcengine.agentkit.foundation.space.SpaceService{} (agw.js_conv = 'str', agw.cli_conv = 'str', api.tag = 'volc-agentkit-service')

service UserService extends volcengine.agentkit.foundation.user.UserService{} (agw.js_conv = 'str', agw.cli_conv = 'str', api.tag = 'volc-agentkit-service')
