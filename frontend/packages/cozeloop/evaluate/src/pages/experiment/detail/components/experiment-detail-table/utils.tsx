import { I18n } from '@cozeloop/i18n-adapter';
import {
  arrayToMap,
  getFieldColumnConfig,
  TagRender,
  TagInput,
} from '@cozeloop/evaluate-components';
import { IDRender, TableColActions } from '@cozeloop/components';
import {
  type ColumnEvaluator,
  type Experiment,
  type EvaluatorRecord,
  type FieldData,
  type ItemResult,
  type FieldSchema,
  FieldType,
  type ColumnAnnotation,
  type AnnotateRecord,
} from '@cozeloop/api-schema/evaluation';
import { Tooltip, type ColumnProps } from '@coze-arch/coze-design';

import {
  type ExperimentDetailColumn,
  type ExperimentItem,
} from '@/types/experiment/experiment-detail';
import { ExperimentGroupItemRunStatus } from '@/components/experiment';

import EvaluatorScore from '../evaluator-score';
import { EvaluatorColumnHeader } from './evaluator-column-header';
import { AnnotateColumnHeader } from './annotate-column-header';

const experimentDataToRecordItems = (data: ItemResult[]) => {
  const recordItems: ExperimentItem[] = [];
  data.forEach(group => {
    group.turn_results?.forEach(turn => {
      // eslint-disable-next-line complexity
      turn.experiment_results?.forEach(experiment => {
        const {
          eval_set,
          evaluator_output,
          target_output,
          system_info,
          annotate_result,
        } = experiment.payload ?? {};
        const evaluatorsResult: Record<string, EvaluatorRecord | undefined> =
          {};
        Object.entries(evaluator_output?.evaluator_records ?? {}).forEach(
          ([evaluatorVersionId, record]) => {
            evaluatorsResult[evaluatorVersionId ?? ''] = record;
          },
        );

        const annotateResult: Record<string, AnnotateRecord | undefined> = {};

        Object.entries(annotate_result?.annotate_records ?? {}).forEach(
          ([tagKeyId, record]) => {
            annotateResult[tagKeyId ?? ''] = record;
          },
        );

        const actualOutput =
          target_output?.eval_target_record?.eval_target_output_data
            ?.output_fields?.actual_output;
        const evalTargetTraceID = target_output?.eval_target_record?.trace_id;

        recordItems.push({
          experimentID: experiment.experiment_id,
          id: `${group.item_id}_${turn.turn_id}`,
          groupID: group.item_id,
          turnID: turn.turn_id ?? '',
          groupIndex: Number(group.item_index) || 0,
          turnIndex: Number(turn.turn_index) || 0,
          datasetRow: arrayToMap<FieldData, FieldData>(
            eval_set?.turn?.field_data_list ?? [],
            'key',
          ),
          actualOutput,
          targetErrorMsg:
            target_output?.eval_target_record?.eval_target_output_data
              ?.eval_target_run_error?.message,
          evaluatorsResult,
          annotateResult,
          runState: system_info?.turn_run_state,
          groupRunState: group?.system_info?.run_state,
          itemErrorMsg: system_info?.error?.detail,
          logID: system_info?.log_id,
          evalTargetTraceID,
        });
      });
    });
  });
  return recordItems;
};

const getEvaluatorColumns = (params: {
  columnEvaluators: ColumnEvaluator[];
  spaceID: Int64;
  experiment: Experiment | undefined;
  handleRefresh: () => void;
}) => {
  const { columnEvaluators, spaceID, experiment, handleRefresh } = params;
  const evaluatorColumns: ExperimentDetailColumn[] = columnEvaluators.map(
    evaluator => ({
      title: (
        <EvaluatorColumnHeader
          evaluator={evaluator}
          tagProps={{ className: 'font-normal' }}
        />
      ),
      // 用来在列管理里面使用的title
      displayName: evaluator.name ?? '-',
      dataIndex: `evaluatorsResult.${evaluator.evaluator_version_id}_${evaluator.name}`,
      key: `${evaluator.evaluator_version_id}_${evaluator.name}`,
      align: 'right',
      width: 180,
      // 本期不支持排序
      // sorter: true,
      // sortIcon: LoopTableSortIcon,
      render(_: unknown, record: ExperimentItem) {
        const evaluatorRecord =
          record.evaluatorsResult?.[evaluator?.evaluator_version_id];
        return (
          <EvaluatorScore
            evaluatorRecord={evaluatorRecord}
            spaceID={spaceID}
            traceID={evaluatorRecord?.trace_id ?? ''}
            evaluatorRecordID={evaluatorRecord?.id ?? ''}
            align="right"
            experiment={experiment}
            onRefresh={handleRefresh}
          />
        );
      },
    }),
  );
  return evaluatorColumns;
};

const getBaseColumn = (params: {
  idColumnTitle?: string;
  fixed?: boolean;
}): ExperimentDetailColumn[] => [
  {
    title: '',
    // 用来在列管理里面使用的title
    displayName: I18n.t('status'),
    // 不支持列管理
    disableColumnManage: true,
    dataIndex: 'status',
    key: 'status',
    width: 60,
    fixed: params.fixed,
    render: (_, record: ExperimentItem) => (
      <ExperimentGroupItemRunStatus
        status={record.groupRunState}
        onlyIcon={true}
      />
    ),
  },
  {
    title: params.idColumnTitle || 'ID',
    disableColumnManage: true,
    dataIndex: 'groupID',
    key: 'id',
    width: 110,
    fixed: params.fixed,
    render: val => <IDRender id={val} useTag={true} />,
  },
];

const getActionColumn = (params: {
  onClick: (record: ExperimentItem) => void;
}): ExperimentDetailColumn => {
  const { onClick } = params;

  return {
    title: I18n.t('operation'),
    disableColumnManage: true,
    dataIndex: 'action',
    key: 'action',
    fixed: 'right',
    align: 'left',
    width: 68,
    render: (_, record) => (
      <TableColActions
        actions={[
          {
            label: (
              <Tooltip content={I18n.t('detail')} theme="dark">
                {I18n.t('detail')}
              </Tooltip>
            ),
            onClick: () => onClick(record),
          },
        ]}
      />
    ),
  };
};

/** 实验详情表格中使用的数据集列配置 */
export function getExprDetailDatasetColumns(
  datasetFields: FieldSchema[],
  options?: { prefix?: string; expand?: boolean },
) {
  const { prefix = '', expand = false } = options ?? {};
  const columns: ExperimentDetailColumn[] = datasetFields.map(field => {
    const column: ExperimentDetailColumn = getFieldColumnConfig({
      field,
      prefix,
      expand,
    });
    // 用来筛选的字段配置
    column.filterField = {
      field_type: FieldType.EvalSetColumn,
      field_key: field.key,
    };
    return column;
  });
  return columns;
}

const getAnnotationColumns = (params: {
  annotations: ColumnAnnotation[];
  spaceID: string;
  experimentID: string;
  mode: 'default' | 'quick_annotate';
  handleRefresh: () => void;
}): ColumnProps<ExperimentItem>[] => {
  const { annotations, spaceID, experimentID, mode, handleRefresh } = params;
  return annotations.map(annotation => ({
    title: (
      <AnnotateColumnHeader
        annotation={annotation}
        spaceID={spaceID}
        experimentID={experimentID}
        onDelete={handleRefresh}
      />
    ),
    dataIndex: `annotate_${annotation.tag_key_id}`,
    key: annotation.tag_key_id,
    width: 240,
    render: (_, record: ExperimentItem) =>
      mode === 'quick_annotate' ? (
        <div onClick={e => e.stopPropagation()}>
          <TagInput
            key={`${annotation.tag_key_id}_${record.id}`}
            spaceID={params.spaceID}
            experimentID={record.experimentID}
            groupID={record.groupID as string}
            turnID={record.turnID as string}
            annotation={annotation}
            annotateRecord={record.annotateResult[annotation.tag_key_id || '']}
            useSelectBoolean={true}
            onChange={handleRefresh}
            onCreateOption={handleRefresh}
          />
        </div>
      ) : (
        <TagRender
          key={`${annotation.tag_key_id}_${record.id}`}
          annotation={annotation}
          annotateRecord={record.annotateResult[annotation.tag_key_id || '']}
        />
      ),
  }));
};
export {
  experimentDataToRecordItems,
  getEvaluatorColumns,
  getActionColumn,
  getBaseColumn,
  getAnnotationColumns,
};
