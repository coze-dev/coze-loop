namespace go coze.loop.foundation

include "coze.loop.foundation.auth.thrift"
include "coze.loop.foundation.authn.thrift"
include "./coze.loop.foundation.space.thrift"
include "./coze.loop.foundation.user.thrift"
include "./coze.loop.foundation.file.thrift"
include "./coze.loop.foundation.openapi.thrift"


service AuthService extends coze.loop.foundation.auth.AuthService{}
service AuthNService extends coze.loop.foundation.authn.AuthNService{}
service UserService extends coze.loop.foundation.user.UserService{}
service SpaceService extends coze.loop.foundation.space.SpaceService{}
service FileService extends coze.loop.foundation.file.FileService{}
service FoundationOpenAPIService extends coze.loop.foundation.openapi.FoundationOpenAPIService{}