namespace go coze.loop.observability.domain.metric

struct Metric {
    1: optional string Summary
    2: optional map<string, string> Pie
    3: optional map<string, list<MetricPoint>> TimeSeries
}

struct MetricPoint {
    1: optional string Timestamp
    2: optional string Value
}