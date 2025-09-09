// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';

export const getDefaultColumnMap = (
  datasetDetail?: EvaluationSet,
  csvHeaders?: string[],
) =>
  datasetDetail?.evaluation_set_version?.evaluation_set_schema?.field_schemas
    ?.filter(item => !!item.name)
    ?.map(item => ({
      target: item.name,
      source: csvHeaders?.includes(item.name || '') ? item.name : '',
      description: item.description,
      fieldSchema: item,
    }));
