// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const DATASET_LIST_COLUMN_STORAGE_KEY = 'dataset-column';
export const getDatasetColumnSortStorageKey = (datasetID: string) =>
  `${DATASET_LIST_COLUMN_STORAGE_KEY}-${datasetID}`;
