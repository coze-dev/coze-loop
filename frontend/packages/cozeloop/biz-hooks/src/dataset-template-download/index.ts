// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export enum DatasetCategory {
  /** 数据集 */
  General = 1,
  /** 训练集 (暂无) */
  Training = 2,
  /** 验证集 (暂无) */
  Validation = 3,
  /** 评测集 (暂无) */
  Evaluation = 4,
}

export interface ListDatasetImportTemplateReq {
  spaceID: string;
  datasetID?: string;
  category?: DatasetCategory;
}

export enum FileFormat {
  JSONL = 1,
  Parquet = 2,
  CSV = 3,
  XLSX = 4,
  /** [100, 200) 压缩格式 */
  ZIP = 100,
}

export const FILE_FORMAT_MAP = {
  [FileFormat.JSONL]: 'JSONL',
  [FileFormat.Parquet]: 'Parquet',
  [FileFormat.CSV]: 'CSV',
  [FileFormat.XLSX]: 'Excel',
  [FileFormat.ZIP]: 'ZIP',
};
export interface ListDatasetImportTemplateResp {
  templates?: Array<ImportTemplate>;
}
export interface ImportTemplate {
  url?: string;
  format?: FileFormat;
}

export const useDatasetTemplateDownload = () => {
  const getDatasetTemplate = async (
    req: ListDatasetImportTemplateReq,
  ): Promise<Array<ImportTemplate> | undefined> => Promise.resolve(undefined);
  return { getDatasetTemplate };
};
