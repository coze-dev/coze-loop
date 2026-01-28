namespace go coze.loop.ml_flow
include "./coze.loop.ml_flow.datasetservicev2.thrift"

service MLFLowDatasetServiceV2 extends coze.loop.ml_flow.datasetservicev2.DatasetServiceV2{} (api.js_conv = "str", agw.cli_conv = "str", agw.preserve_base="true", api.tag = 'volc-agentkit-service')
