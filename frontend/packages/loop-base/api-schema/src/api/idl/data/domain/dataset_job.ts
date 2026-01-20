// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as dataset from './dataset';
export { dataset };
/** 通用任务类型 */
export enum JobType {
  ImportFromFile = 1,
  ExportToFile = 2,
  ExportToDataset = 3,
}
/** 通用任务状态 */
export enum JobStatus {
  Undefined = 0,
  /** 待处理 */
  Pending = 1,
  /** 处理中 */
  Running = 2,
  /** 已完成 */
  Completed = 3,
  /** 失败 */
  Failed = 4,
  /** 已取消 */
  Cancelled = 5,
}
export const LogLevelInfo = "info";
export const LogLevelError = "error";
export const LogLevelWarning = "warning";
/** 通用任务日志 */
export interface JobLog {
  content: string,
  level: string,
  timestamp: string,
  hidden: boolean,
}
export enum FileFormat {
  JSONL = 1,
  Parquet = 2,
  CSV = 3,
  XLSX = 4,
  /** [100, 200) 压缩格 */
  ZIP = 100,
}
export interface DatasetIOFile {
  provider: dataset.StorageProvider,
  path: string,
  /** 数据文件的格式 */
  format?: FileFormat,
  /** 压缩包格式 */
  compress_format?: FileFormat,
  /** path 为文件夹或压缩包时，数据文件列表, 服务端设置 */
  files?: string[],
}
export interface DatasetIODataset {
  space_id?: string,
  dataset_id: string,
  version_id?: string,
}
export interface DatasetIOEndpoint {
  file?: DatasetIOFile,
  dataset?: DatasetIODataset,
}
/** DatasetIOJob 数据集导入导出任务 */
export interface DatasetIOJob {
  id: string,
  app_id?: number,
  space_id: string,
  /** 导入导出到文件时，为数据集 ID；数据集间转移时，为目标数据集 ID */
  dataset_id: string,
  job_type: JobType,
  source: DatasetIOEndpoint,
  target: DatasetIOEndpoint,
  /** 字段映射 */
  field_mappings?: FieldMapping[],
  option?: DatasetIOJobOption,
  /** 运行数据, [20, 100) */
  status?: JobStatus,
  progress?: DatasetIOJobProgress,
  errors?: dataset.ItemErrorGroup[],
  /** 通用信息 */
  created_by?: string,
  created_at?: string,
  updated_by?: string,
  updated_at?: string,
  started_at?: string,
  ended_at?: string,
}
export interface DatasetIOJobOption {
  /** 覆盖数据集 */
  overwrite_dataset?: boolean
}
export interface DatasetIOJobProgress {
  /** 总量 */
  total?: string,
  /** 已处理数量 */
  processed?: string,
  /** 已成功处理的数量 */
  added?: string,
  /**
   * 子任
   * 可空, 表示子任务的名称
  */
  name?: string,
  /** 子任务的进度 */
  sub_progresses?: DatasetIOJobProgress[],
}
export interface FieldMapping {
  source: string,
  target: string,
}