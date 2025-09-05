import { useEffect, useMemo, useState } from 'react';

import classNames from 'classnames';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { TableWithPagination } from '@cozeloop/components';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import {
  type ListExperimentsRequest,
  type ListExperimentsResponse,
  type Evaluator,
  type Experiment,
} from '@cozeloop/api-schema/evaluation';
import {
  IconCozAnalytics,
  IconCozArrowDown,
  IconCozInfoCircle,
  IconCozLineChart,
} from '@coze-arch/coze-design/icons';
import { Button, Radio, Select, Tooltip } from '@coze-arch/coze-design';

import { ExperimentListEmptyState } from '../previews/experiment-list-empty-state';
import { ExperimentRowSelectionActions } from '../experiment-row-selection-actions';
import { ExperimentContrastChart } from '../contrast-chart';
import { EvaluatorPreview } from '../../previews/evaluator-preview';
import { ColumnsManage } from '../../common';
import { type SemiTableSort } from '../../../utils/order-by';
import {
  uniqueExperimentsEvaluators,
  getTableSelectionRows,
} from '../../../utils/experiment';
import {
  type ExperimentListColumnsOptions,
  useExperimentListStore,
} from '../../../hooks/use-experiment-list-store';
import RelatedExperimentHeader, {
  type FilterValues,
  type ChartConfigValues,
  filterFields,
} from './related-experiment-header';

import styles from './index.module.less';

const defaultColumnsOptions: ExperimentListColumnsOptions = {
  enableSort: true,
  enableIdColumn: true,
  columnManageStorageKey: 'related_experiment_list_column_manage',
};

// 数据集关联实验中，数据集是固定的不支持筛选
const defaultDisabledFields = ['eval_set'];

// eslint-disable-next-line @coze-arch/max-line-per-function, complexity, max-lines-per-function
export function DatasetRelatedExperiment({
  spaceID = '',
  datasetID = '',
  className,
  showEvalSetTooltip,
  showEvalTargetTooltip,
  disabledLogicFilterFields,
  experimentsColumnsOptions,
  refreshKey,
  sourceName,
  sourcePath,
  disableBatchOperate,
  pullExperiments,
}: {
  spaceID: Int64;
  datasetID?: Int64;
  className?: string;
  showEvalSetTooltip?: boolean;
  showEvalTargetTooltip?: boolean;
  disabledLogicFilterFields?: string[];
  experimentsColumnsOptions?: ExperimentListColumnsOptions;
  refreshKey?: string | number;
  disableBatchOperate?: boolean;
  // 拉取实验列表的接口，默认使用 StoneEvaluationApi.PullExperiments
  pullExperiments?: (
    req: ListExperimentsRequest,
  ) => Promise<ListExperimentsResponse>;
  /** 来源名称，例如实验列表、关联实验、对比实验等 */
  sourceName?: string;
  /** 所属页面来源路径，用来点击返回时使用 */
  sourcePath?: string;
}) {
  const navigateModule = useNavigateModule();
  const [evaluators, setEvaluators] = useState<Evaluator[]>([]);
  const [chartConfig, setChartConfig] = useState<ChartConfigValues>({
    chartType: 'line',
    chartVisible: true,
    evaluators: [],
  });

  const defaultFilter = useMemo(() => ({ eval_set: [datasetID] }), [datasetID]);
  const columnsOptions = useMemo(
    () => ({
      ...defaultColumnsOptions,
      ...(experimentsColumnsOptions ?? {}),
      detailJumpSourcePath: sourcePath,
      onDetailClick: () => {
        sendEvent('cozeloop_experiment_detail_view', {
          from: sourceName,
        });
      },
    }),
    [experimentsColumnsOptions, sourceName],
  );

  const {
    service,
    columns,
    defaultColumns,
    setColumns,
    filter,
    setFilter,
    logicFilter,
    batchOperate,
    setBatchOperate,
    selectedExperiments,
    setSelectedExperiments,
    hasFilterCondition,
    refreshAsync,
    onSortChange,
    onFilterDebounceChange,
    onLogicFilterChange,
  } = useExperimentListStore<FilterValues>({
    defaultFilter,
    filterFields,
    columnsOptions,
    pullExperiments,
    source: sourceName,
  });

  const experiments = service.data?.list;

  const handleFilterChange = (name: keyof FilterValues, val: unknown) => {
    setFilter(oldFilter => ({ ...(oldFilter ?? {}), [name]: val }));
    onFilterDebounceChange();
  };

  useEffect(() => {
    const newEvaluators = uniqueExperimentsEvaluators(experiments ?? []);
    setEvaluators(newEvaluators);

    setChartConfig(oldState => ({
      ...oldState,
      evaluators: newEvaluators.map(
        evaluator => evaluator?.current_version?.id ?? '',
      ),
    }));
  }, [experiments]);

  useEffect(() => {
    if (refreshKey !== undefined) {
      refreshAsync();
    }
  }, [refreshKey]);

  const updateChartConfig = (name: keyof ChartConfigValues, val: unknown) => {
    setChartConfig(oldState => ({ ...oldState, [name]: val }));
  };
  const expand = chartConfig.chartVisible ?? true;

  const chartHeader = (
    <div className="flex items-center gap-1 w-full h-8">
      <div className="text-sm font-semibold">{I18n.t('overview')}</div>
      <Tooltip
        theme="dark"
        content={I18n.t('aggregate_statistics_score_on_metrics')}
      >
        <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
      </Tooltip>
      <IconCozArrowDown
        className={classNames('cursor-pointer', expand ? '' : '-rotate-90')}
        onClick={() => updateChartConfig('chartVisible', !expand)}
      />
      {expand ? (
        <div className="ml-auto flex items-center gap-2">
          <Radio.Group
            type="button"
            className="!gap-0"
            value={chartConfig.chartType}
            onChange={e => {
              updateChartConfig('chartType', e.target.value);
            }}
          >
            <Tooltip content={I18n.t('line_chart')} theme="dark">
              <Radio
                value="line"
                addonClassName="flex items-center"
                addonStyle={{ padding: '4px 6px' }}
              >
                <IconCozLineChart className="text-xxl" />
              </Radio>
            </Tooltip>
            <Tooltip content={I18n.t('bar_chart')} theme="dark">
              <Radio
                value="bar"
                addonClassName="flex items-center"
                addonStyle={{ padding: '4px 6px' }}
              >
                <IconCozAnalytics className="text-xxl" />
              </Radio>
            </Tooltip>
          </Radio.Group>
          <Select
            prefix={I18n.t('indicator')}
            placeholder={I18n.t('please_select', { field: '' })}
            style={{ minWidth: 200 }}
            multiple={true}
            maxTagCount={1}
            className={styles.select}
            value={chartConfig.evaluators}
            optionList={evaluators?.map(evaluator => ({
              label: (
                <EvaluatorPreview
                  evaluator={evaluator}
                  className="overflow-hidden ml-1"
                  style={{ maxWidth: 120 }}
                />
              ),
              value: evaluator?.current_version?.id ?? '',
            }))}
            onChange={val => updateChartConfig('evaluators', val)}
          />
        </div>
      ) : null}
    </div>
  );

  const tableHeader = (
    <RelatedExperimentHeader
      filter={filter}
      onFilterChange={handleFilterChange}
      logicFilter={logicFilter}
      onLogicFilterChange={onLogicFilterChange}
      disabledFields={disabledLogicFilterFields ?? defaultDisabledFields}
      actions={
        batchOperate && !disableBatchOperate ? (
          <ExperimentRowSelectionActions
            spaceID={spaceID}
            experiments={selectedExperiments}
            onRefresh={() => service.refresh()}
            onCancelSelect={() => {
              setBatchOperate(false);
              setSelectedExperiments([]);
            }}
            onReportCompare={s => {
              sendEvent(EVENT_NAMES.cozeloop_experiment_compare_count, {
                from: 'evaluate_set_related_expts',
                status: s ?? 'success',
              });
              sendEvent(EVENT_NAMES.cozeloop_experiment_compare, {
                from: 'dataset_related_experiment_batch_select',
              });
            }}
          />
        ) : (
          <>
            <ColumnsManage
              columns={columns}
              defaultColumns={defaultColumns}
              storageKey={columnsOptions.columnManageStorageKey}
              onColumnsChange={setColumns}
            />
            {disableBatchOperate ? null : (
              <Button
                color="primary"
                onClick={() => {
                  setBatchOperate(!batchOperate);
                  setSelectedExperiments([]);
                }}
              >
                {I18n.t('batch_select')}
              </Button>
            )}
          </>
        )
      }
    />
  );
  return (
    <div className={classNames('py-4 flex flex-col', className)}>
      <div className="flex flex-col gap-3">
        {chartHeader}

        {expand ? (
          <div className="w-full shrink-0 overflow-y-hidden overflow-x-auto styled-scrollbar">
            <ExperimentContrastChart
              spaceID={spaceID}
              experiments={experiments}
              experimentIds={experiments?.map(e => e.id ?? '')}
              selectedEvaluatorsId={chartConfig.evaluators}
              loading={service.loading}
              showActions={false}
              chartType={chartConfig.chartType ?? 'line'}
              layout="horizontal"
              xFieldValueType="id"
              showAggregatorTypeSelect={false}
              onlyBaseExperimentEvluators={false}
              cardHeaderStyle={{ height: 44 }}
              cardBodyStyle={{ height: 226 }}
              showEvalSetTooltip={showEvalSetTooltip}
              showEvalTargetTooltip={showEvalTargetTooltip}
              onRefresh={() => service.refresh()}
            />
          </div>
        ) : null}
      </div>
      <div className="text-sm font-semibold mt-5 mb-3">
        {I18n.t('experiments_list')}
      </div>
      <TableWithPagination<Experiment>
        service={service}
        heightFull={false}
        header={tableHeader}
        showSizeChanger={false}
        tableProps={{
          rowKey: 'id',
          columns,
          rowSelection: batchOperate
            ? {
                selectedRowKeys: selectedExperiments.map(e => e.id ?? ''),
                onChange(newKeys = [], rows = []) {
                  const newExperiments = getTableSelectionRows(
                    newKeys as string[],
                    rows,
                    selectedExperiments,
                  );
                  setSelectedExperiments(newExperiments);
                },
              }
            : false,
          onRow: (record: Experiment) => ({
            onClick: () => {
              // 如果当前有选中的文本，不触发点击事件
              if (!window.getSelection()?.isCollapsed) {
                return;
              }
              sendEvent('cozeloop_experiment_detail_view', {
                from: sourceName,
              });
              navigateModule(
                `evaluation/experiments/${record.id}`,
                sourcePath
                  ? {
                      state: { from: sourcePath },
                    }
                  : undefined,
              );
            },
          }),
          onChange(changeInfo) {
            if (
              changeInfo.extra?.changeType === 'sorter' &&
              changeInfo.sorter?.key
            ) {
              onSortChange(changeInfo.sorter as unknown as SemiTableSort);
            }
          },
        }}
        empty={
          <ExperimentListEmptyState hasFilterCondition={hasFilterCondition} />
        }
      />
    </div>
  );
}
