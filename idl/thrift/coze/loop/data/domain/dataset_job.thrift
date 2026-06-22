namespace go coze.loop.data.domain.dataset_job

include "dataset.thrift"

// 通用任务类型
enum JobType {
    ImportFromFile = 1
    ExportToFile = 2
    ExportToDataset = 3
}

// 通用任务状态
enum JobStatus {
    Undefined = 0
    Pending = 1   // 待处理
    Running = 2   // 处理中
    Completed = 3 // 已完成
    Failed = 4    // 失败
    Cancelled = 5 // 已取消
}

const string LogLevelInfo = "info"
const string LogLevelError = "error"
const string LogLevelWarning = "warning"

// 通用任务日志
struct JobLog {
    1: required string content
    2: required string level
    3: required i64 timestamp (api.js_conv='true', go.tag='json:"timestamp"')
    10: required bool hidden
}

enum FileFormat {
    JSONL = 1
    Parquet = 2
    CSV = 3
    XLSX = 4

    /*[100, 200) 压缩格式*/
    ZIP = 100
}

enum SourceType {
    File = 1
    Dataset = 2
    // SDD: add-single-trajectory-offline-eval — Trace 来源（按 trace_id 列表导入轨迹列），由 observability 模块解析
    Trace = 3
}

struct DatasetIOFile {
    1: required dataset.StorageProvider provider (vt.defined_only='true')
    2: required string path (vt.min_size='1')
    3: optional FileFormat format                                             // 数据文件的格式
    4: optional FileFormat compress_format                                     // 压缩包格式
    5: optional list<string> files                                            // path 为文件夹或压缩包时，数据文件列表, 服务端设置
    6: optional string original_file_name                                       // 原始的文件名，创建文件时由前端写入。为空则与 path 保持一致
    7: optional string download_url                                            // 文件下载地址
    100: optional string provider_id                                          // 存储提供方ID，目前主要在 provider==imagex 时生效
    101: optional ProviderAuth provider_auth                                   // 存储提供方鉴权信息，目前主要在 provider==imagex 时生效
}

struct ProviderAuth {
    1: optional i64 provider_account_id (api.js_conv="true", go.tag='json:"provider_account_id"') // provider == VETOS 时，此处存储的是用户在 fornax 上托管的方舟账号的ID
}

struct DatasetIODataset {
    1: optional i64 space_id (api.js_conv='true', go.tag='json:"space_id"')
    2: required i64 dataset_id (api.js_conv='true', go.tag='json:"dataset_id"')
    3: optional i64 version_id (api.js_conv='true', go.tag='json:"version_id"')
}

// SDD: add-single-trajectory-offline-eval — Trace 来源载体；评测域 ParseImportSourceFile / 创建带导入的评测集会通过此结构传入 trace_id 列表
struct DatasetIOTrace {
    1: required list<string> trace_ids (vt.min_size='1', vt.max_size='10')   // 单次最多 10 个 trace（observability 现有上限）
    2: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"')
    3: optional i64 start_time (api.js_conv='true', go.tag='json:"start_time"')   // trace 解析查询窗口起点，毫秒
    4: optional i64 end_time (api.js_conv='true', go.tag='json:"end_time"')       // trace 解析查询窗口终点，毫秒
    5: optional string platform_type                                              // observability 侧 PlatformType 透传（如 cozeloop / openagent）
    10: optional string filter_json                                               // 进阶过滤；保留扩展位
}

struct DatasetIOEndpoint {
    1: optional DatasetIOFile file
    2: optional DatasetIODataset dataset
    // SDD: add-single-trajectory-offline-eval — Trace 来源端点；source_type=Trace 时填充
    3: optional DatasetIOTrace trace
}

// DatasetIOJob 数据集导入导出任务
struct DatasetIOJob {
    1: required i64 id (api.js_conv='true', go.tag='json:"id"')
    2: optional i32 app_id
    3: required i64 space_id (api.js_conv='true', go.tag='json:"space_id"')
    4: required i64 dataset_id (api.js_conv='true', go.tag='json:"dataset_id"')   // 导入导出到文件时，为数据集 ID；数据集间转移时，为目标数据集 ID
    5: required JobType job_type
    6: required DatasetIOEndpoint source
    7: required DatasetIOEndpoint target
    8: optional list<FieldMapping> field_mappings      // 字段映射
    9: optional DatasetIOJobOption option

    /* 运行数据, [20, 100) */
    20: optional JobStatus status
    21: optional DatasetIOJobProgress progress
    22: optional list<dataset.ItemErrorGroup> errors

    /* 通用信息 */
    100: optional string created_by
    101: optional i64 created_at (api.js_conv='true', go.tag='json:"created_at"')
    102: optional string updated_by
    103: optional i64 updated_at (api.js_conv='true', go.tag='json:"updated_at"')
    104: optional i64 started_at (api.js_conv='true', go.tag='json:"started_at"')
    105: optional i64 ended_at (api.js_conv='true', go.tag='json:"ended_at"')
}

struct DatasetIOJobOption {
    1: optional bool overwrite_dataset // 覆盖数据集
}

struct DatasetIOJobProgress {
    2: optional i64 total (api.js_conv='true', go.tag='json:"total"')                                // 总量
    3: optional i64 processed (api.js_conv='true', go.tag='json:"processed"')                         // 已处理数量
    4: optional i64 added (api.js_conv='true', go.tag='json:"added"')                             // 已成功处理的数量

    /*子任务*/
    10: optional string name                              // 可空, 表示子任务的名称
    11: optional list<DatasetIOJobProgress> sub_progresses // 子任务的进度
}

struct FieldMapping {
    1: required string source (vt.min_size='1')
    2: required string target (vt.min_size='1')
}