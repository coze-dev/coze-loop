namespace go coze.loop.observability

include "coze.loop.observability.trace.thrift"
include "coze.loop.observability.openapi.thrift"
include "coze.loop.observability.task.thrift"

service ObservabilityTraceService extends coze.loop.observability.trace.TraceService{}
service ObservabilityOpenAPIService extends coze.loop.observability.openapi.OpenAPIService{}
service ObservabilityTaskService extends coze.loop.observability.task.TaskService{}