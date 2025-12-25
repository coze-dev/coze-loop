// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type EvaluationSetItemTableData } from '@cozeloop/evaluate-components';
import {
  type EvaluationSetItem,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';

export const codeEvaluatorConvertEvaluationSetItemListToTableData = (
  evaluationSetItemList: EvaluationSetItem[],
  fieldSchemas: FieldSchema[],
): EvaluationSetItemTableData[] => {
  const resList: EvaluationSetItemTableData[] = [];
  evaluationSetItemList?.forEach(item => {
    let turns = item?.turns;
    if (!turns?.length) {
      // 添加空数据渲染
      turns = [{}];
    }
    turns.forEach(turn => {
      const fieldDataMap = {};
      fieldSchemas?.forEach(fieldSchema => {
        if (!fieldSchema.name) {
          return;
        }
        const fieldData = turn?.field_data_list?.find(
          field => field.name === fieldSchema.name,
        );
        fieldDataMap[fieldSchema.name] = fieldData;
      });
      resList.push({
        ...item,
        trunFieldData: {
          id: turn?.id,
          fieldDataMap,
        },
      });
    });
  });
  return resList;
};
