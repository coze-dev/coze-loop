/* eslint-disable max-lines-per-function */
import { useEffect, useMemo, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  ColumnsManage,
  dealColumnsFromStorage,
  LogicEditor,
  type LogicField,
  type SemiTableSort,
} from '@cozeloop/evaluate-components';
import {
  type Experiment,
  FieldType,
  type ColumnEvaluator,
  type FieldSchema,
  type BatchGetExperimentResultResponse,
  ExptStatus,
  type FilterOperatorType,
  type ItemRunState,
  type KeywordSearch,
  type FilterField,
  type ColumnAnnotation,
} from '@cozeloop/api-schema/evaluation';
import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import {
  Divider,
  EmptyState,
  Radio,
  RadioGroup,
  TextArea,
  Toast,
} from '@coze-arch/coze-design';

import { getActualOutputColumn, isTraceTargetExpr } from '@/utils/experiment';
import {
  type Filter,
  type Service,
} from '@/types/experiment/experiment-detail-table';
import {
  type ExperimentDetailColumn,
  type ExperimentItem,
} from '@/types/experiment/experiment-detail';
import styles from '@/styles/table-row-hover-show-icon.module.less';
import { useExperimentDetailStore } from '@/hooks/use-experiment-detail-store';
import { useExperimentDetailActiveItem } from '@/hooks/use-experiment-detail-active-item';
import TableForExperiment, {
  TableHeader,
} from '@/components/table-for-experiment';
import { ExprGroupItemRunStatusSelect } from '@/components/experiment';
import TableCellExpand from '@/components/common/table-cell-expand';

import ExperimentItemDetail from '../experiment-item-detail';
import {
  experimentDataToRecordItems,
  getActionColumn,
  getEvaluatorColumns,
  getBaseColumn,
  getExprDetailDatasetColumns,
  getAnnotationColumns,
} from './utils';
import { TraceTargetTraceDetailPanel } from './trace-target-trace-detail-panel';
import { getFilterFields, MAX_SEARCH_LENGTH } from './logic-filter-fields';
import { AddAnnotateColumn } from './add-annotate-col-settings';

import filterStyles from './filter.module.less';

const filterFields: {
  key: keyof Filter;
  type: FieldType;
  operator?: FilterOperatorType;
}[] = [
  {
    key: 'status',
    type: FieldType.ItemRunState,
  },
];

const experimentResultToRecordItems = (
  res: BatchGetExperimentResultResponse,
) => {
  const list = experimentDataToRecordItems(res.item_results ?? []);
  return list;
};

// eslint-disable-next-line @coze-arch/max-line-per-function, complexity
export default function ({
  spaceID,
  experimentID,
  refreshKey,
  experiment,
  onRefreshPage,
}: {
  spaceID: string;
  experimentID: string;
  refreshKey: string;
  experiment: Experiment | undefined;
  onRefreshPage: () => void;
}) {
  const [columns, setColumns] = useState<ExperimentDetailColumn[]>([]);
  const [defaultColumns, setDefaultColumns] = useState<
    ExperimentDetailColumn[]
  >([]);
  const [fieldSchemas, setFieldSchemas] = useState<FieldSchema[]>([]);
  const [columnAnnotations, setColumnAnnotations] = useState<
    ColumnAnnotation[]
  >([]);
  const [columnEvaluators, setColumnEvaluators] = useState<ColumnEvaluator[]>(
    [],
  );
  const [itemTraceVisible, setTraceVisible] = useState(false);
  const [keyword, setKeyword] = useState('');
  // 标注模式
  const [mode, setMode] = useState<'default' | 'quick_annotate'>('default');

  const logicFields: LogicField[] = useMemo(
    () => getFilterFields(columnEvaluators, fieldSchemas, columnAnnotations),
    [columnEvaluators, fieldSchemas, columnAnnotations],
  );

  const experimentIds = useMemo(() => [experimentID], [experimentID]);

  const keywordSearch = useMemo(() => {
    if (!keyword) {
      return undefined;
    }
    const newKeywordSearch: KeywordSearch = {
      keyword,
      filter_fields: columns
        .map(column => (column.hidden ? undefined : column.filterField))
        .filter(Boolean) as FilterField[],
    };
    return newKeywordSearch;
  }, [keyword, columns]);

  const {
    service,
    filter,
    setFilter,
    onFilterDebounceChange,
    logicFilter,
    setLogicFilter,
    onLogicFilterChange,
    onSortChange,
    expand,
    setExpand,
  } = useExperimentDetailStore<ExperimentItem, Filter>({
    experimentIds,
    experimentResultToRecordItems,
    pageSizeStorageKey: 'experiment_detail_page_size',
    filterFields,
    refreshKey,
    keywordSearch,
  });

  const activeItemStore = useExperimentDetailActiveItem({
    experimentIds,
    filter,
    logicFilter,
    filterFields,
    keywordSearch,
    experimentResultToRecordItems,
    defaultTotal: service.data?.total ?? 0,
  });
  const {
    activeItem,
    setActiveItem,
    itemDetailVisible,
    setItemDetailVisible,
    onItemStepChange,
  } = activeItemStore;

  const columnManageStorageKey = `experiment_detail_column_manage_${experimentID}`;

  useEffect(() => {
    const res = service.data?.result;
    setFieldSchemas(res?.column_eval_set_fields ?? []);
    setColumnEvaluators(
      res?.expt_column_evaluators?.filter(
        item => item.experiment_id === experimentID,
      )?.[0]?.column_evaluators ?? [],
    );
    setColumnAnnotations(
      res?.expt_column_annotations?.filter(
        item => item.experiment_id === experimentID,
      )?.[0]?.column_annotations ?? [],
    );
  }, [service.data?.result, experimentID]);

  const handleRefresh = () => {
    service.refresh();
  };

  useEffect(() => {
    const {
      column_evaluators = [],
      column_eval_set_fields = [],
      expt_column_annotations = [],
    } = service.data?.result ?? {};
    const isTraceTarget = isTraceTargetExpr(experiment);
    const actualOutputColumns = isTraceTarget
      ? []
      : [
          getActualOutputColumn({
            expand,
            traceIdPath: 'evalTargetTraceID',
            experiment,
            column: {
              filterField: {
                field_type: FieldType.ActualOutput,
                field_key: 'actual_output',
              },
            },
          }),
        ];
    // 评估器列
    const evaluatorColumns: ExperimentDetailColumn[] = getEvaluatorColumns({
      columnEvaluators: column_evaluators,
      spaceID,
      experiment,
      handleRefresh,
    });
    // 标注列
    const annotationColumns = getAnnotationColumns({
      spaceID,
      experimentID,
      annotations:
        expt_column_annotations.filter(
          item => item.experiment_id === experimentID,
        )?.[0]?.column_annotations ?? [],
      mode,
      handleRefresh,
    });
    // 操作列
    const actionColumn: ExperimentDetailColumn = getActionColumn({
      onClick: (record: ExperimentItem) => {
        setActiveItem(record);
        if (isTraceTarget) {
          setTraceVisible(true);
        } else {
          setItemDetailVisible(true);
        }
      },
    });
    // 列配置
    const newColumns: ExperimentDetailColumn[] = [
      ...getBaseColumn({ fixed: mode === 'quick_annotate' }),
      ...getExprDetailDatasetColumns(column_eval_set_fields, {
        prefix: 'datasetRow.',
        expand,
      }),
      ...actualOutputColumns,
      ...evaluatorColumns,
      ...annotationColumns,
    ];

    setColumns([
      ...dealColumnsFromStorage(newColumns, columnManageStorageKey),
      actionColumn,
    ]);
    setDefaultColumns([...newColumns, actionColumn]);
  }, [service.data, spaceID, expand, experiment, mode]);

  const filters = (
    <>
      <div className="w-60">
        <TextArea
          placeholder={`${I18n.t('enter_keyword_search_max_length', { MAX_SEARCH_LENGTH })}`}
          rows={1}
          showClear={true}
          value={keyword}
          onChange={val => {
            if (val && val.length > MAX_SEARCH_LENGTH) {
              Toast.warning(
                `${I18n.t('keyword_search_length_limited_truncated', { MAX_SEARCH_LENGTH })}`,
              );
            }
            const newVal = val?.slice(0, MAX_SEARCH_LENGTH);
            setKeyword(newVal);
            onFilterDebounceChange();
          }}
          onKeyDown={event => {
            // 阻止默认的换行行为，使用 Shift + Enter 换行的多行输入框
            if (event.key === 'Enter' && !event.shiftKey) {
              event.preventDefault();
            }
          }}
          onEnterPress={() => {
            onFilterDebounceChange();
          }}
        />
      </div>
      <ExprGroupItemRunStatusSelect
        style={{ minWidth: 170 }}
        value={filter?.status}
        onChange={val => {
          setFilter(oldState => ({
            ...oldState,
            status: val as ItemRunState[],
          }));
          onFilterDebounceChange();
        }}
      />
      <LogicEditor
        fields={logicFields}
        value={logicFilter}
        enableCascadeMode={true}
        popoverProps={{ contentClassName: filterStyles['logic-filter-style'] }}
        onConfirm={newVal => {
          setLogicFilter(newVal ?? {});
          onLogicFilterChange(newVal);
        }}
      />
    </>
  );
  // 操作
  const actions = (
    <>
      {columnAnnotations.length ? (
        <>
          <RadioGroup
            value={mode}
            onChange={e => {
              setMode(e.target.value as 'default' | 'quick_annotate');
              setExpand(e.target.value === 'quick_annotate');
            }}
          >
            <Radio value="default">
              {I18n.t('fornax_prompt_compare_normal')}
            </Radio>
            <Radio value="quick_annotate">
              {I18n.t('quick_annotation_mode')}
            </Radio>
          </RadioGroup>
          <Divider margin={16} layout="vertical" />
        </>
      ) : null}
      <AddAnnotateColumn
        spaceID={spaceID}
        experimentID={experimentID}
        data={columnAnnotations}
        onAnnotateAdd={() => {
          handleRefresh();
          setMode('quick_annotate');
          setExpand(true);
        }}
        onAnnotateDelete={() => {
          handleRefresh();
          if (columnAnnotations.length <= 1) {
            setMode('default');
          }
        }}
      />
      <TableCellExpand expand={expand} onChange={setExpand} />
      <ColumnsManage
        columns={columns}
        defaultColumns={defaultColumns}
        storageKey={columnManageStorageKey}
        onColumnsChange={setColumns}
      />
    </>
  );

  // 表格属性
  const tableProps = {
    className: styles['table-row-hover-show-icon'],
    rowKey: 'id',
    columns,
    onRow: record => ({
      onClick: () => {
        // 如果当前有选中的文本，或者处在快速标注模式，不触发点击事件
        if (!window.getSelection()?.isCollapsed || mode === 'quick_annotate') {
          return;
        }
        setActiveItem(record);
        const isTraceTarget = isTraceTargetExpr(experiment);
        if (isTraceTarget) {
          setTraceVisible(true);
        } else {
          setItemDetailVisible(true);
          setMode('default');
        }
      },
    }),
    onChange(changeInfo) {
      if (changeInfo.extra?.changeType === 'sorter' && changeInfo.sorter?.key) {
        onSortChange(changeInfo.sorter as unknown as SemiTableSort);
      }
    },
  };

  // 表格空状态
  const tableEmpty =
    experiment?.status === ExptStatus.Pending ? (
      <EmptyState
        size="full_screen"
        icon={<IconCozIllusAdd />}
        title={I18n.t('experiment_initializing')}
        description={
          <>
            {I18n.t('wait_a_few_seconds')}
            <span
              className="text-[rgb(var(--coze-up-brand-9))] cursor-pointer"
              onClick={onRefreshPage}
            >
              {I18n.t('task_refresh')}
            </span>
            {I18n.t('page_view')}
          </>
        }
      />
    ) : (
      <EmptyState
        size="full_screen"
        icon={<IconCozIllusAdd />}
        title={I18n.t('no_data')}
      />
    );

  return (
    <>
      <TableForExperiment<ExperimentItem>
        service={service as Service}
        heightFull={true}
        header={<TableHeader actions={actions} filters={filters} />}
        pageSizeStorageKey="experiment_detail_page_size"
        empty={tableEmpty}
        tableProps={tableProps}
      />
      {activeItem && itemDetailVisible ? (
        <ExperimentItemDetail
          spaceID={spaceID}
          activeItemStore={activeItemStore}
          fieldSchemas={fieldSchemas}
          columnEvaluators={columnEvaluators}
          columnAnnotations={columnAnnotations}
          onClose={() => {
            setItemDetailVisible(false);
            handleRefresh();
          }}
          onStepChange={onItemStepChange}
          // onAnnotateChange={handleRefresh}
          // onCreateOption={handleRefresh}
        />
      ) : null}

      {itemTraceVisible ? (
        <TraceTargetTraceDetailPanel
          // 和服务端的约定，基于Trace的在线评测情况下，traceID和spanID的数据在评测集中存放
          traceID={activeItem?.datasetRow?.trace_id?.content?.text ?? ''}
          spanID={activeItem?.datasetRow?.span_id?.content?.text ?? ''}
          experiment={experiment}
          onClose={() => setTraceVisible(false)}
        />
      ) : null}
    </>
  );
}
