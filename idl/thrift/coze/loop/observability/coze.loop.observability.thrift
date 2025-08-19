namespace go coze.loop.observability

include "coze.loop.observability.trace.thrift"
include "coze.loop.observability.openapi.thrift"

service ObservabilityTraceService extends coze.loop.observability.trace.TraceService{}
service ObservabilityOpenAPIService extends coze.loop.observability.openapi.OpenAPIService{}