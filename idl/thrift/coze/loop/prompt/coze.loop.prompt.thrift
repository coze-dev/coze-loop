namespace go coze.loop.prompt

include "coze.loop.prompt.manage.thrift"
include "coze.loop.prompt.tool_manage.thrift"
include "coze.loop.prompt.debug.thrift"
include "coze.loop.prompt.openapi.thrift"
include "coze.loop.prompt.execute.thrift"

service PromptManageService extends coze.loop.prompt.manage.PromptManageService{}
service ToolManageService extends coze.loop.prompt.tool_manage.ToolManageService{}
service PromptDebugService extends coze.loop.prompt.debug.PromptDebugService{}
service PromptExecuteService extends coze.loop.prompt.execute.PromptExecuteService{}
service PromptOpenAPIService extends coze.loop.prompt.openapi.PromptOpenAPIService{}
