namespace go volcengine.agentkit.ml_flow.datasetservicev2

include "../../../base.thrift"
include "../../../coze/loop/ml_flow/data/datasetv2.thrift"
include "../../../coze/loop/ml_flow/data/datasetv2job.thrift"
include "../../../coze/loop/ml_flow/data/filter.thrift"
include "../../../coze/loop/ml_flow/data/datasetv2similarity.thrift"
include "../../../coze/loop/ml_flow/coze.loop.ml_flow.datasetservicev2.thrift"

service DatasetServiceV2 {

    coze.loop.ml_flow.datasetservicev2.BatchUploadDatasetAttachmentsResp BatchUploadDatasetAttachments(1: coze.loop.ml_flow.datasetservicev2.BatchUploadDatasetAttachmentsReq req) (
        api.post = '/api/ml_flow/v2/BatchUploadDatasetAttachments', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.ListDatasetImportTemplateResp ListDatasetImportTemplate(1: coze.loop.ml_flow.datasetservicev2.ListDatasetImportTemplateReq req) (
        api.get = '/api/ml_flow/v2/ListDatasetImportTemplate', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.SignUploadFileTokenResp SignUploadFileToken(1: coze.loop.ml_flow.datasetservicev2.SignUploadFileTokenReq req) (
        api.get = '/api/ml_flow/v2/SignUploadFileToken', api.category = 'mlflow', api.tag = 'volc-agentkit-gen', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.ImportDatasetResp ImportDataset(1: coze.loop.ml_flow.datasetservicev2.ImportDatasetReq req) (
        api.post = '/api/ml_flow/v2/ImportDataset', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.GetDatasetIOJobResp GetDatasetIOJob(1: coze.loop.ml_flow.datasetservicev2.GetDatasetIOJobReq req) (
        api.get = '/api/ml_flow/v2/GetDatasetIOJob', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.SearchDatasetIOJobsOfDatasetResp SearchDatasetIOJobsOfDataset(1: coze.loop.ml_flow.datasetservicev2.SearchDatasetIOJobsOfDatasetReq req) (
        api.post = '/api/ml_flow/v2/SearchDatasetIOJobsOfDataset', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.CancelDatasetIOJobResp CancelDatasetIOJob(1: coze.loop.ml_flow.datasetservicev2.CancelDatasetIOJobReq req) (
        api.post = '/api/ml_flow/v2/CancelDatasetIOJob', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.ExportDatasetResp ExportDataset(1: coze.loop.ml_flow.datasetservicev2.ExportDatasetReq req) (
        api.post = '/api/ml_flow/v2/ExportDataset', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.ParseImportSourceFileResp ParseImportSourceFile(1: coze.loop.ml_flow.datasetservicev2.ParseImportSourceFileReq req) (
        api.post = '/api/ml_flow/v2/ParseImportSourceFile', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

}
