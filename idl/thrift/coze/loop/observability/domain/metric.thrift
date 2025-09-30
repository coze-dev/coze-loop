namespace go coze.loop.observability.domain.metric

typedef string CompareType (ts.enum="true")
const CompareType CompareType_YoY = "yoy" // 同比
const CompareType CompareType_MoM = "mom" // 环比

struct Metric {
    1: optional string summary
    2: optional map<string, string> pie
    3: optional map<string, list<MetricPoint>> time_series
}

struct MetricPoint {
    1: optional string timestamp
    2: optional string value
}

struct Compare {
    1: optional CompareType compare_type
    2: optional i64 shift_seconds (api.js_conv='true', go.tag='json:"shift_seconds"', vt.gt='0')
}