namespace go coze.loop.observability.metric

include "../../../base.thrift"
include "./domain/filter.thrift"
include "./domain/common.thrift"
include "./domain/metric.thrift"


struct GetMetricsRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: required i64 start_time (api.js_conv='true', go.tag='json:"start_time"', api.body="start_time", vt.gt="0")
    3: required i64 end_time (api.js_conv='true', go.tag='json:"end_time"', api.body="end_time", vt.gt="0")
    4: required list<string> metric_names (api.body="metric_names", vt.min_size = "1")
    5: optional string granularity (api.body="granularity")
    6: optional filter.FilterFields filters (api.body="filters")
    7: optional common.PlatformType platform_type (api.body="platform_type")
    8: optional list<filter.FilterField> drill_down_fields (api.body="drill_down_fields")
    9: optional metric.Compare compare (api.body="compare")

    255: optional base.Base Base
}

struct GetMetricsResponse {
    1: optional map<string, metric.Metric> metrics
    2: optional map<string, metric.Metric> compared_metrics

    255: optional base.BaseResp BaseResp
}

service MetricService {
    GetMetricsResponse GetMetrics(1: GetMetricsRequest Req) (api.post='/api/observability/v1/metrics/list')
}
