import { useEffect } from 'react';

import { uniq } from 'lodash-es';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  ColumnsManage,
  RefreshButton,
  ExperimentStatusSelect,
  ExperimentNameSearch,
  verifyContrastExperiment,
  ExperimentListEmptyState,
  EvaluateSetSelect,
  ExperimentEvaluatorLogicFilter,
  getTableSelectionRows,
  TypographyText,
  useExperimentListStore,
  type SemiTableSort,
} from '@cozeloop/evaluate-components';
import {
  type ExptStatus,
  type Experiment,
} from '@cozeloop/api-schema/evaluation';
import { IconCozWarningCircleFill } from '@coze-arch/coze-design/icons';
import { Modal, Space, Tag, Typography } from '@coze-arch/coze-design';

import TableForExperiment, {
  TableHeader,
} from '@/components/table-for-experiment';

import { filterFields, type Filter } from './logic-filter';

export function hasSameDataset(list?: Experiment[]): boolean {
  const firstDatasetId = list?.[0]?.eval_set?.id;
  if (!firstDatasetId) {
    return true;
  }
  return list.every(item => item?.eval_set?.id === firstDatasetId);
}

function mergeExperiments(experiments: Experiment[]) {
  const mergedExperiments: Experiment[] = [];
  const map: Map<Int64, boolean> = new Map();
  experiments.forEach(experiment => {
    const experimentId = experiment.id ?? '';
    if (map.has(experimentId)) {
      return;
    }
    map.set(experimentId, true);
    mergedExperiments.push(experiment);
  });
  return mergedExperiments;
}

const disabledLogicFilterFields = ['eval_set'];
const columnsOptions = {
  enableSort: true,
  enableIdColumn: false,
  enableActionColumn: false,
  columnManageStorageKey: 'contrast_experiment_list_column_manage',
};

// eslint-disable-next-line @coze-arch/max-line-per-function
export default function ExperimentSelectModal({
  contrastExperiments,
  defaultFilter,
  disabledFilterFields,
  onOk,
  onClose,
  onReportCompare,
}: {
  /** 对比试验组，数组中第一个认为是基准试验 */
  contrastExperiments?: Experiment[];
  /** 默认筛选条件 */
  defaultFilter?: Filter;
  /** 禁用某些筛选条件的编辑 */
  disabledFilterFields?: (keyof Filter)[];
  onOk?: (experimentIds: Int64[], experiments: Experiment[]) => void;
  onClose?: () => void;
  onReportCompare?: (status: string) => void;
}) {
  const {
    service,
    columns,
    defaultColumns,
    setColumns,
    filter,
    setFilter,
    logicFilter,
    selectedExperiments,
    setSelectedExperiments,
    hasFilterCondition,
    onSortChange,
    onFilterDebounceChange,
    onLogicFilterChange,
  } = useExperimentListStore<Filter>({
    defaultFilter,
    filterFields,
    columnsOptions,
    pageSizeStorageKey: 'experiment_list_contrast_page_size',
  });

  const baseExperiment = contrastExperiments?.[0];

  useEffect(() => {
    const baseId = baseExperiment?.id;
    if (baseId) {
      setSelectedExperiments(oldData =>
        mergeExperiments([
          baseExperiment,
          ...(contrastExperiments ?? []),
          ...oldData,
        ]),
      );
    }
  }, [baseExperiment, contrastExperiments]);

  const updateFilter = (name: keyof Filter, val: unknown) => {
    setFilter(oldState => ({ ...oldState, [name]: val }));
    onFilterDebounceChange();
  };

  const baseExperimentId = baseExperiment?.id;

  const selectedRowKeys = selectedExperiments.map(e => e.id ?? '');
  const tableHeader = (
    <TableHeader
      className="shrink-0"
      filters={
        <>
          <ExperimentNameSearch
            value={filter?.name}
            onChange={val => updateFilter('name', val)}
          />
          <EvaluateSetSelect
            prefix={I18n.t('evaluation_set')}
            value={filter?.eval_set}
            disabled={disabledFilterFields?.includes('eval_set')}
            multiple={true}
            maxTagCount={1}
            onChangeWithObject={false}
            renderSelectedItem={(item: {
              name?: string;
              label?: React.ReactNode;
              value?: string;
            }) => ({
              isRenderInTag: false,
              content: (
                <Typography.Text
                  className="!max-w-[100px]"
                  ellipsis={{ showTooltip: true }}
                >
                  <>{item?.name || item?.label || item?.value}</>
                </Typography.Text>
              ),
            })}
            onChange={val => {
              updateFilter('eval_set', val);
            }}
          />
          <ExperimentStatusSelect
            value={filter?.status}
            disabled={disabledFilterFields?.includes('status')}
            maxTagCount={undefined}
            onChange={val => updateFilter('status', val as ExptStatus[])}
          />
          <ExperimentEvaluatorLogicFilter
            value={logicFilter}
            // 评测集改为外露了，这里将评测集从逻辑条件中移除
            disabledFields={disabledLogicFilterFields}
            onChange={onLogicFilterChange}
          />
        </>
      }
      actions={
        <>
          <RefreshButton onRefresh={service.refresh} />
          <ColumnsManage
            columns={columns}
            defaultColumns={defaultColumns}
            storageKey={columnsOptions.columnManageStorageKey}
            onColumnsChange={setColumns}
          />
        </>
      }
    />
  );
  return (
    <Modal
      title={I18n.t('select_experiment')}
      visible={true}
      okButtonProps={{ disabled: selectedExperiments.length < 2 }}
      okText={I18n.t('run_experiment_comparison')}
      cancelText={I18n.t('cancel')}
      onOk={() => {
        const experiments = mergeExperiments([
          ...(baseExperiment ? [baseExperiment] : []),
          ...selectedExperiments,
        ]);
        if (!verifyContrastExperiment(experiments)) {
          onReportCompare?.('fail');
          return;
        } else {
          onReportCompare?.('success');
        }
        sendEvent(EVENT_NAMES.cozeloop_experiment_compare, {
          from: 'experiment_detail_compare_modal',
        });
        const keys = experiments.map(e => e.id ?? '');
        onOk?.(keys, selectedExperiments);
      }}
      onCancel={onClose}
      width="90%"
      height="fill"
    >
      <div className="h-full flex flex-col gap-4 overflow-hidden">
        <Space
          style={{
            backgroundColor:
              'rgba(var(--coze-yellow-0),var(--coze-yellow-0-alpha))',
          }}
          className="coz-fg-primary pl-4 rounded-[6px]  w-full h-[32px] flex items-center bg-[rgba(var(--coze-yellow-0), var(--coze-yellow-0-alpha))]"
        >
          <IconCozWarningCircleFill className="text-orange-500" />
          <span>{I18n.t('only_experiments_compared_tip')}</span>
        </Space>

        <div className="shrink-0 flex items-center gap-1">
          <span className="font-medium">{I18n.t('selected')}</span>
          <div className="flex items-center gap-1">
            {selectedExperiments.map(experiment => (
              <Tag
                key={experiment?.id}
                color="brand"
                className="max-w-[160px]"
                closable={baseExperimentId !== experiment.id}
                onClose={() => {
                  setSelectedExperiments(oldData =>
                    oldData.filter(e => e.id !== experiment.id),
                  );
                }}
              >
                <TypographyText>{experiment?.name}</TypographyText>
              </Tag>
            ))}
          </div>
        </div>
        <div className="grow overflow-hidden">
          <TableForExperiment<Experiment>
            service={service}
            heightFull={true}
            header={tableHeader}
            pageSizeStorageKey="experiment_list_contrast_page_size"
            tableProps={{
              rowKey: 'id',
              columns,
              rowSelection: {
                selectedRowKeys,
                getCheckboxProps(record: Experiment) {
                  const disabled =
                    baseExperimentId !== undefined &&
                    record.id === baseExperimentId;
                  return {
                    disabled,
                    name: record.name,
                    checked: selectedRowKeys.includes(record.id ?? ''),
                  };
                },
                onChange(newKeys = [], rows = []) {
                  const newExperiments = getTableSelectionRows(
                    uniq([
                      ...(baseExperimentId ? [baseExperimentId as string] : []),
                      ...(newKeys as string[]),
                    ]),
                    [...(baseExperiment ? [baseExperiment] : []), ...rows],
                    selectedExperiments,
                  );
                  setSelectedExperiments(newExperiments);
                },
              },
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
              <ExperimentListEmptyState
                hasFilterCondition={hasFilterCondition}
              />
            }
          />
        </div>
      </div>
    </Modal>
  );
}
