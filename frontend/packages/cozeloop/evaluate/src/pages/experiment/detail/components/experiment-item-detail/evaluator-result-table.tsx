// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { TypographyText } from '@cozeloop/evaluate-components';
import { LoopTable } from '@cozeloop/components';
import {
  type EvaluatorRecord,
  type ColumnEvaluator,
  EvaluatorRunStatus,
} from '@cozeloop/api-schema/evaluation';
import { type ColumnProps } from '@coze-arch/coze-design';

import styles from '@/styles/table-row-hover-show-icon.module.less';
import { useExperiment } from '@/hooks/use-experiment';
import {
  EvaluatorColumnPreview,
  EvaluatorRunStatusPreview,
} from '@/components/experiment';

import EvaluatorScore from '../evaluator-score';

export default function EvaluatorResultTable({
  spaceID,
  evaluatorRecordMap = {},
  columnEvaluators = [],
  onRefresh,
}: {
  spaceID: Int64;
  evaluatorRecordMap: Record<Int64, EvaluatorRecord | undefined>;
  columnEvaluators: ColumnEvaluator[];
  onRefresh?: () => void;
}) {
  const [columns, setColumns] = useState<ColumnProps[]>([]);
  const experiment = useExperiment();
  useEffect(() => {
    const hasEvaluatorError = Object.values(evaluatorRecordMap).some(
      item => item?.status === EvaluatorRunStatus.Fail,
    );
    const newColumns: ColumnProps<ColumnEvaluator>[] = [
      {
        title: I18n.t('evaluator'),
        dataIndex: 'name',
        key: 'name',
        width: 100,
        render(_, record) {
          return (
            <EvaluatorColumnPreview evaluator={record} enableLinkJump={true} />
          );
        },
      },
      {
        title: I18n.t('status'),
        dataIndex: 'status',
        key: 'status',
        width: hasEvaluatorError ? 200 : 73,
        render: (_, record) => {
          const result = evaluatorRecordMap[record?.evaluator_version_id];
          const { status, evaluator_output_data } = result ?? {};
          const isError =
            Boolean(evaluator_output_data?.evaluator_run_error?.message) ||
            true;
          return (
            <EvaluatorRunStatusPreview
              status={status}
              onlyIcon={true}
              className="h-5"
              extra={
                isError ? (
                  <TypographyText
                    className="!text-[var(--coz-fg-hglt-red)]"
                    tooltipTheme="light"
                  >
                    {evaluator_output_data?.evaluator_run_error?.message}
                  </TypographyText>
                ) : null
              }
            />
          );
        },
      },
      {
        title: I18n.t('score'),
        dataIndex: 'score',
        key: 'score',
        width: 100,
        render: (_, record) => {
          const evaluatorRecord =
            evaluatorRecordMap[record?.evaluator_version_id];
          return (
            <EvaluatorScore
              spaceID={spaceID}
              traceID={evaluatorRecord?.trace_id ?? ''}
              evaluatorRecordID={evaluatorRecord?.id ?? ''}
              evaluatorRecord={evaluatorRecord}
              experiment={experiment}
              onRefresh={onRefresh}
            />
          );
        },
      },
      {
        title: I18n.t('score_reason'),
        dataIndex: 'reasoning',
        key: 'reasoning',
        align: 'left',
        width: 200,
        render: (_, record) => {
          const result = evaluatorRecordMap[record.evaluator_version_id];
          const { evaluator_result } = result?.evaluator_output_data ?? {};
          return (
            <TypographyText tooltipTheme="light">
              {evaluator_result?.correction?.explain ??
                evaluator_result?.reasoning ??
                '-'}
            </TypographyText>
          );
        },
      },
    ];
    setColumns(newColumns);
  }, [columnEvaluators, spaceID, evaluatorRecordMap]);
  return (
    <div>
      <LoopTable
        tableProps={{
          className: styles['table-row-hover-show-icon'],
          rowKey: 'dataIndex',
          columns,
          dataSource: columnEvaluators,
        }}
      />
    </div>
  );
}
