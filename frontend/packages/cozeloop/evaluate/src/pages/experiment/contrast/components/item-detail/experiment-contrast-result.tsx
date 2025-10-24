// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, useState } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  TraceTrigger,
  EvaluatorNameScore,
  useGlobalEvalConfig,
  ChipSelect,
  AnnotationNameScore,
} from '@cozeloop/evaluate-components';
import {
  type AnnotateRecord,
  type ColumnAnnotation,
  type ColumnEvaluator,
  type EvaluatorRecord,
  type Experiment,
  type ExperimentTurnPayload,
} from '@cozeloop/api-schema/evaluation';
import { FieldDisplayFormat } from '@cozeloop/api-schema/data';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Divider, Tooltip } from '@coze-arch/coze-design';

import { CellContentRender } from '@/utils/experiment';
import { type ColumnInfo } from '@/types/experiment/experiment-contrast';
import { FORMAT_LIST } from '@/types';
import { ExperimentRunDataSummary } from '@/components/experiment';

import { getColumnRecords } from '../../utils/tools';

export default function ExperimentContrastResult({
  result,
  experiment,
  expand,
  spaceID,
  columnInfos,
  onRefresh,
}: {
  experiment: Experiment | undefined;
  result: ExperimentTurnPayload | undefined;
  expand?: boolean;
  spaceID?: Int64;
  columnInfos?: ColumnInfo[];
  onRefresh?: () => void;
}) {
  const { traceEvalTargetPlatformType } = useGlobalEvalConfig();
  const actualOutput =
    result?.target_output?.eval_target_record?.eval_target_output_data
      ?.output_fields?.actual_output;
  const targetTraceID = result?.target_output?.eval_target_record?.trace_id;
  const onReportCalibration = () => {
    sendEvent(EVENT_NAMES.cozeloop_experiment_detailsdrawer_editscore, {
      from: 'experiment_contrast_item_detail',
    });
  };
  const [format, setFormat] = useState<FieldDisplayFormat>(
    actualOutput?.format || FieldDisplayFormat.Markdown,
  );
  const onReportEvaluatorTrace = () => {
    sendEvent(EVENT_NAMES.cozeloop_experiment_detailsdrawer_trace, {
      from: 'experiment_contrast_item_detail',
    });
  };

  const items = useMemo(
    () => getColumnRecords(columnInfos ?? [], result),
    [columnInfos, result],
  );

  return (
    <div className="group flex flex-col gap-2 h-full group">
      <div className="flex gap-2 flex-wrap">
        {items.map(item => {
          if (item.type === 'evaluator') {
            const evaluatorRecord = (item.data as EvaluatorRecord) ?? {};
            const evaluatorResult =
              evaluatorRecord.evaluator_output_data?.evaluator_result;

            return (
              <div key={item.columnInfo.key} className="max-w-[100%]">
                <EvaluatorNameScore
                  key={item.columnInfo.key}
                  evaluator={item.columnInfo.data as ColumnEvaluator}
                  evaluatorResult={evaluatorResult}
                  experiment={experiment}
                  updateUser={evaluatorRecord?.base_info?.updated_by}
                  spaceID={spaceID}
                  traceID={evaluatorRecord?.trace_id}
                  evaluatorRecordID={evaluatorRecord?.id}
                  enablePopover={true}
                  showVersion={true}
                  onEditScoreSuccess={onRefresh}
                  onReportCalibration={onReportCalibration}
                  onReportEvaluatorTrace={onReportEvaluatorTrace}
                />
              </div>
            );
          } else if (item.type === 'annotation') {
            return (
              <AnnotationNameScore
                key={item.columnInfo.key}
                annotation={item.columnInfo.data as ColumnAnnotation}
                annotationResult={item.data as AnnotateRecord}
                enablePopover={true}
              />
            );
          }
          return <>-</>;
        })}
      </div>
      <ExperimentRunDataSummary
        result={result}
        latencyHidden={true}
        tokenHidden={true}
      />
      <Divider />
      <div className="flex items-center justify-between">
        <div className="flex gap-1 items-center">
          <div className="text-[var(--coz-fg-secondary)]">actual_output</div>
          <Tooltip
            theme="dark"
            content={I18n.t('evaluation_object_actual_output')}
          >
            <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
          </Tooltip>
        </div>
        <ChipSelect
          chipRender="selectedItem"
          className="invisible group-hover:visible"
          value={format}
          optionList={FORMAT_LIST}
          onChange={value => {
            setFormat(value as FieldDisplayFormat);
          }}
        ></ChipSelect>
      </div>

      <div className="group flex leading-5 w-full grow min-h-[20px] overflow-hidden">
        <CellContentRender
          expand={expand}
          content={actualOutput}
          displayFormat={true}
          className="!max-h-[none]"
        />
        {targetTraceID ? (
          <div className="flex ml-auto" onClick={e => e.stopPropagation()}>
            <TraceTrigger
              className="ml-1 invisible group-hover:visible"
              traceID={targetTraceID ?? ''}
              platformType={traceEvalTargetPlatformType}
              startTime={experiment?.start_time}
              endTime={experiment?.end_time}
            />
          </div>
        ) : null}
      </div>
    </div>
  );
}
