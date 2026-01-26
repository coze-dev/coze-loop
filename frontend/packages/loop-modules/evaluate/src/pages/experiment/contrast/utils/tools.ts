// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { arrayToMap } from '@cozeloop/evaluate-components';
import {
  type ExperimentResult,
  type ExperimentTurnPayload,
  type ItemResult,
  type FieldData,
} from '@cozeloop/api-schema/evaluation';

import {
  type ColumnInfo,
  type ColumnRecord,
} from '@/types/experiment/experiment-contrast';
import { type DatasetRow } from '@/types';

export interface ExperimentContrastItem {
  id: Int64;
  turnID: Int64;
  groupID: Int64;
  groupIndex: number;
  turnIndex: number;
  datasetRow: DatasetRow;
  experimentsDatasetRow: Record<Int64, DatasetRow>;
  experimentResults: Record<Int64, ExperimentTurnPayload>;
}

export function experimentContrastToRecordItems(data: ItemResult[]) {
  const recordItems: ExperimentContrastItem[] = [];
  data.forEach(group => {
    group.turn_results?.forEach(turn => {
      const firstEvelSet = turn.experiment_results?.[0]?.payload?.eval_set;
      const record: ExperimentContrastItem = {
        id: `${group.item_id}_${turn.turn_id}`,
        groupID: group.item_id ?? '',
        turnID: turn.turn_id ?? '',
        groupIndex: Number(group.item_index) || 0,
        turnIndex: Number(turn.turn_index) || 0,
        datasetRow: arrayToMap<FieldData, FieldData>(
          firstEvelSet?.turn?.field_data_list ?? [],
          'key',
        ),
        experimentsDatasetRow: {},
        experimentResults: arrayToMap<ExperimentResult, ExperimentTurnPayload>(
          turn.experiment_results ?? [],
          'experiment_id',
          'payload',
        ),
      };
      turn.experiment_results?.forEach(experimentResult => {
        const evalSet = experimentResult.payload?.eval_set;
        record.experimentsDatasetRow[experimentResult.experiment_id ?? ''] =
          arrayToMap<FieldData, FieldData>(
            evalSet?.turn?.field_data_list ?? [],
            'key',
          );
      });
      recordItems.push(record);
    });
  });
  return recordItems;
}

export const getColumnRecords = (
  columnInfos: ColumnInfo[],
  result?: ExperimentTurnPayload,
) => {
  const res: ColumnRecord[] = [];
  columnInfos?.forEach(info => {
    if (info.type === 'evaluator') {
      res.push({
        type: 'evaluator',
        columnInfo: info,
        data: result?.evaluator_output?.evaluator_records?.[info.key],
      });
    } else if (info.type === 'annotation') {
      res.push({
        type: 'annotation',
        columnInfo: info,
        data: result?.annotate_result?.annotate_records?.[info.key],
      });
    }
  }) ?? [];

  return res;
};
