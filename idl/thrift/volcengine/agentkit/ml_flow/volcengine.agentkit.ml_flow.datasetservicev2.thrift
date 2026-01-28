namespace go volcengine.agentkit.ml_flow.datasetservicev2

include "../../../base.thrift"
include "../../../coze/loop/ml_flow/data/datasetv2.thrift"
include "../../../coze/loop/ml_flow/data/datasetv2job.thrift"
include "../../../coze/loop/ml_flow/data/filter.thrift"
include "../../../coze/loop/ml_flow/data/datasetv2similarity.thrift"
include "../../../coze/loop/ml_flow/coze.loop.ml_flow.datasetservicev2.thrift"

struct BatchUploadDatasetAttachmentsReq {
    1: required i64 spaceID (api.js_conv = 'str', api.query = 'WorkspaceId', vt.gt = '0')
    2: optional i64 datasetID (api.js_conv = 'str', vt.gt = '0')
    3: optional datasetv2.ContentType contentType
    50: optional list<datasetv2.ImageField> images (vt.min_size = '1', vt.max_size = '100')
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base base
}

struct ListDatasetImportTemplateReq {
    1: required i64 spaceID (api.js_conv = 'str', api.query = 'WorkspaceId', vt.gt = '0')
    2: optional i64 datasetID (api.js_conv = 'str', vt.gt = '0')
    3: optional datasetv2.DatasetCategory category (vt.defined_only = 'true')
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base base
}

struct SignUploadFileTokenReq {
    1: optional i64 spaceID (api.js_conv = 'str', vt.not_nil = 'true', vt.gt = '0', api.query = 'WorkspaceId')
    2: optional datasetv2.StorageProvider storage (vt.not_nil = 'true', vt.defined_only = 'true')
    3: optional string fileName
    10: optional string imageXServiceID
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base base
}

struct ImportDatasetReq {
    1: optional i64 spaceID (api.js_conv = 'str', vt.not_nil = 'true', vt.gt = '0', api.query = 'WorkspaceId')
    2: required i64 datasetID (api.js_conv = 'str', api.query = 'DatasetID', vt.gt = '0')
    3: optional datasetv2job.DatasetIOFile file (vt.not_nil = 'true')
    4: optional list<datasetv2job.FieldMapping> fieldMappings (vt.min_size = '1')
    5: optional datasetv2job.DatasetIOJobOption option
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base base
}

struct GetDatasetIOJobReq {
    1: optional i64 spaceID (api.js_conv = 'str', vt.not_nil = 'true', vt.gt = '0', api.query = 'WorkspaceId')
    2: required i64 jobID (api.js_conv = 'str', api.query = 'JobID', vt.gt = '0')
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base base
}

struct SearchDatasetIOJobsOfDatasetReq {
    1: optional i64 spaceID (api.js_conv = 'str', vt.not_nil = 'true', vt.gt = '0', api.query = 'WorkspaceId')
    2: required i64 datasetID (api.js_conv = 'str', api.query = 'DatasetID', vt.gt = '0')
    3: optional list<datasetv2job.JobType> types
    4: optional list<datasetv2job.JobStatus> statuses
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base base
}

struct CancelDatasetIOJobReq {
    1: required i64 spaceID (api.js_conv = 'str', api.query = 'WorkspaceId', vt.gt = '0')
    2: required i64 jobID (api.js_conv = 'str', api.query = 'JobID', vt.gt = '0')
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base base
}

struct ExportDatasetReq {
    1: required i64 spaceID (api.js_conv = 'str', api.query = 'WorkspaceId', vt.gt = '0')
    2: required i64 datasetID (api.js_conv = 'str', api.query = 'DatasetID', vt.gt = '0')
    3: optional i64 versionID (api.js_conv = 'str')
    4: required datasetv2job.SourceType targetType (vt.defined_only = 'true')
    5: required datasetv2job.DatasetIOEndpoint target
    6: optional datasetv2job.WriteMode writeMode
    7: optional list<datasetv2job.FieldMapping> fieldMappings
    8: optional list<i64> itemIDs
    9: optional datasetv2job.ExportETLStrategy etlStrategy
    10: optional list<datasetv2job.InternalFieldExportConfig> internalFieldExportConfig
    11: optional datasetv2job.Visibility visibility (vt.defined_only = 'true')
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base base
}

struct ParseImportSourceFileReq {
    1: required i64 spaceID (api.js_conv = 'str', api.query = 'WorkspaceId', vt.gt = '0')
    2: optional datasetv2job.DatasetIOFile file (vt.not_nil = 'true')
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base base
}

service DatasetServiceV2 {

    coze.loop.ml_flow.datasetservicev2.BatchUploadDatasetAttachmentsResp BatchUploadDatasetAttachments(1: BatchUploadDatasetAttachmentsReq req) (
        api.post = '/api/ml_flow/v2/BatchUploadDatasetAttachments', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.ListDatasetImportTemplateResp ListDatasetImportTemplate(1: ListDatasetImportTemplateReq req) (
        api.get = '/api/ml_flow/v2/ListDatasetImportTemplate', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.SignUploadFileTokenResp SignUploadFileToken(1: SignUploadFileTokenReq req) (
        api.get = '/api/ml_flow/v2/SignUploadFileToken', api.category = 'mlflow', api.tag = 'volc-agentkit-gen', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.ImportDatasetResp ImportDataset(1: ImportDatasetReq req) (
        api.post = '/api/ml_flow/v2/ImportDataset', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.GetDatasetIOJobResp GetDatasetIOJob(1: GetDatasetIOJobReq req) (
        api.get = '/api/ml_flow/v2/GetDatasetIOJob', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.SearchDatasetIOJobsOfDatasetResp SearchDatasetIOJobsOfDataset(1: SearchDatasetIOJobsOfDatasetReq req) (
        api.post = '/api/ml_flow/v2/SearchDatasetIOJobsOfDataset', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.CancelDatasetIOJobResp CancelDatasetIOJob(1: CancelDatasetIOJobReq req) (
        api.post = '/api/ml_flow/v2/CancelDatasetIOJob', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.ExportDatasetResp ExportDataset(1: ExportDatasetReq req) (
        api.post = '/api/ml_flow/v2/ExportDataset', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    coze.loop.ml_flow.datasetservicev2.ParseImportSourceFileResp ParseImportSourceFile(1: ParseImportSourceFileReq req) (
        api.post = '/api/ml_flow/v2/ParseImportSourceFile', api.category = 'mlflow', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

}
