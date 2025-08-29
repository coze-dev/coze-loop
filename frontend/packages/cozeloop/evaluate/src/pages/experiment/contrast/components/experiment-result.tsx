import { useMemo } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import {
  AutoOverflowList,
  EvaluatorNameScore,
  AnnotationNameScore,
} from '@cozeloop/evaluate-components';
import {
  type Experiment,
  type ExperimentTurnPayload,
  type EvaluatorRecord,
  type ColumnEvaluator,
  type ColumnAnnotation,
  type AnnotateRecord,
} from '@cozeloop/api-schema/evaluation';

import {
  type ColumnInfo,
  type ColumnRecord,
} from '@/types/experiment/experiment-contrast';
import {
  ActualOutputWithTrace,
  ExperimentRunDataSummary,
} from '@/components/experiment';

import { getColumnRecords } from '../utils/tools';

export interface ExperimentContrastResultProps {
  result: ExperimentTurnPayload | undefined;
  experiment: Experiment | undefined;
  expand?: boolean;
  hiddenFieldMap?: Record<string, boolean>;
  spaceID?: Int64;
  onRefresh?: () => void;
  columnInfos?: ColumnInfo[];
}

export default function ExperimentResult({
  result,
  experiment,
  expand,
  spaceID,
  hiddenFieldMap = {},
  onRefresh,
  columnInfos,
}: ExperimentContrastResultProps) {
  const items = useMemo(
    () => getColumnRecords(columnInfos ?? [], result),
    [columnInfos, result],
  );

  const actualOutput =
    result?.target_output?.eval_target_record?.eval_target_output_data
      ?.output_fields?.actual_output;
  const targetTraceID = result?.target_output?.eval_target_record?.trace_id;
  const onReportCalibration = () => {
    sendEvent(EVENT_NAMES.cozeloop_experiment_detailsdrawer_editscore, {
      from: 'experiment_contrast_result',
    });
  };
  const onReportEvaluatorTrace = () => {
    sendEvent(EVENT_NAMES.cozeloop_experiment_detailsdrawer_trace, {
      from: 'experiment_contrast_result',
    });
  };
  return (
    <div className="flex flex-col gap-2" onClick={e => e.stopPropagation()}>
      <ActualOutputWithTrace
        expand={expand}
        content={actualOutput}
        traceID={targetTraceID}
        startTime={experiment?.start_time}
        endTime={experiment?.end_time}
      />
      <AutoOverflowList<ColumnRecord>
        itemKey={'current_version.id'}
        items={items}
        itemRender={({ item, inOverflowPopover }) => {
          if (item.type === 'evaluator') {
            const evaluatorRecord = (item.data as EvaluatorRecord) ?? {};
            const evaluatorResult =
              evaluatorRecord.evaluator_output_data?.evaluator_result;

            return (
              <EvaluatorNameScore
                key={item.columnInfo.key}
                evaluator={item.columnInfo.data as ColumnEvaluator}
                evaluatorResult={evaluatorResult}
                experiment={experiment}
                updateUser={evaluatorRecord.base_info?.updated_by}
                spaceID={spaceID}
                traceID={evaluatorRecord.trace_id}
                evaluatorRecordID={evaluatorRecord.id}
                enablePopover={!inOverflowPopover}
                enableEditScore={false}
                border={!inOverflowPopover}
                showVersion={true}
                defaultShowAction={inOverflowPopover}
                onEditScoreSuccess={onRefresh}
                onReportCalibration={onReportCalibration}
                onReportEvaluatorTrace={onReportEvaluatorTrace}
              />
            );
          } else if (item.type === 'annotation') {
            return (
              <AnnotationNameScore
                key={item.columnInfo.key}
                annotation={item.columnInfo.data as ColumnAnnotation}
                annotationResult={item.data as AnnotateRecord}
                enablePopover={!inOverflowPopover}
                border={!inOverflowPopover}
                defaultShowAction={inOverflowPopover}
              />
            );
          }
          return <>-</>;
        }}
      />
      <ExperimentRunDataSummary
        result={result}
        latencyHidden={true}
        tokenHidden={true}
        statusHidden={hiddenFieldMap.status}
      />
    </div>
  );
}
