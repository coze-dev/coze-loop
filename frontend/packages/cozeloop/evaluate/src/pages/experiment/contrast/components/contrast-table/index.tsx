import { useEffect, useState } from 'react';

import { type PaginationResult } from 'ahooks/lib/usePagination/types';
import { safeJsonParse } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { type LogicFilter } from '@cozeloop/evaluate-components';
import { TableColActions, IDRender } from '@cozeloop/components';
import {
  type BatchGetExperimentResultResponse,
  type Experiment,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { Tooltip, type ColumnProps } from '@coze-arch/coze-design';

import { getDatasetColumns } from '@/utils/experiment';
import { type ColumnInfo } from '@/types/experiment/experiment-contrast';
import styles from '@/styles/table-row-hover-show-icon.module.less';
import { useExperimentDetailStore } from '@/hooks/use-experiment-detail-store';
import { useExperimentDetailActiveItem } from '@/hooks/use-experiment-detail-active-item';
import TableForExperiment from '@/components/table-for-experiment';

import ExperimentContrastItemDetail from '../item-detail';
import {
  type ExperimentContrastItem,
  experimentContrastToRecordItems,
} from '../../utils/tools';
import ExperimentContrastTableHeader from './table-header';
import { getExperimentContrastColumns } from './columns';

interface RequestParams {
  current: number;
  pageSize: number;
  logicFilter?: LogicFilter;
}

type Service = PaginationResult<
  { total: number; list: ExperimentContrastItem[] },
  [RequestParams]
>;

function experimentResultToRecordItems(res: BatchGetExperimentResultResponse) {
  const list = experimentContrastToRecordItems(res.item_results ?? []);
  return list;
}

function readLocalColumn(columnManageStorageKey: string) {
  const str = localStorage.getItem(columnManageStorageKey);
  const data:
    | {
        hiddenFieldMap: Record<Int64, boolean>;
        hiddenColumnMap: Record<Int64, boolean>;
      }
    | undefined = safeJsonParse(str) ?? undefined;
  return data;
}

// eslint-disable-next-line @coze-arch/max-line-per-function
export default function ExperimentContrastTable({
  spaceID,
  experiments = [],
  experimentIds,
  onExperimentChange,
}: {
  spaceID: string;
  experiments: Experiment[] | undefined;
  experimentIds: string[] | undefined;
  onExperimentChange?: (experiments: Experiment[]) => void;
}) {
  const [columns, setColumns] = useState<ColumnProps[]>([]);
  // 数据表头
  const [fieldSchemas, setFieldSchemas] = useState<FieldSchema[]>([]);

  const [columnInfosMap, setColumnInfosMap] = useState<
    Record<string, ColumnInfo[]>
  >({});

  const columnManageStorageKey = `experiment_contrast_detail_column_manage_${experimentIds?.[0]}`;

  const [hiddenColumnMap, setHiddenColumnMap] = useState<
    Record<string, boolean>
  >({});
  const [hiddenExperimentFieldMap, setHiddenExperimentFieldMap] = useState<
    Record<Int64, boolean>
  >({});

  const { service, logicFilter, onLogicFilterChange, expand, setExpand } =
    useExperimentDetailStore<ExperimentContrastItem>({
      experimentIds,
      experimentResultToRecordItems,
      pageSizeStorageKey: 'experiment_contrast_page_size',
    });

  const activeItemStore = useExperimentDetailActiveItem({
    experimentIds,
    logicFilter,
    experimentResultToRecordItems,
    defaultTotal: service.data?.result?.total || 0,
  });
  const { activeItem, setActiveItem, onItemStepChange } = activeItemStore;

  useEffect(() => {
    const res = service.data?.result;
    setFieldSchemas(res?.column_eval_set_fields ?? []);

    const result = (experimentIds || []).reduce(
      (prev, cur) => {
        prev[cur] = [];
        return prev;
      },
      {} as unknown as Record<string, ColumnInfo[]>,
    );

    res?.expt_column_evaluators?.forEach(item => {
      result[item.experiment_id || ''].push(
        ...(item.column_evaluators || []).map(evaluator => ({
          type: 'evaluator' as const,
          key: evaluator.evaluator_version_id || '',
          name: evaluator.name || '',
          data: evaluator,
        })),
      );
    });

    res?.expt_column_annotations?.forEach(item => {
      result[item.experiment_id || ''].push(
        ...(item.column_annotations || []).map(annotation => ({
          type: 'annotation' as const,
          key: annotation.tag_key_id || '',
          name: annotation.tag_key_name || '',
          data: annotation,
        })),
      );
    });
    setColumnInfosMap(result);
  }, [service.data?.result]);

  useEffect(() => {
    const newColumnInfosMap: Record<string, ColumnInfo[]> = {};
    for (const [key, value] of Object.entries(columnInfosMap)) {
      newColumnInfosMap[key] = value.filter(
        info => !hiddenExperimentFieldMap[info.key],
      );
    }

    const newColumns: ColumnProps<ExperimentContrastItem>[] = [
      {
        title: 'ID',
        disableColumnManage: true,
        dataIndex: 'groupID',
        key: 'id',
        width: 110,
        hidden: hiddenColumnMap.turnID ?? false,
        canManage: true,
        render: val => <IDRender id={val} useTag={true} />,
      },
      ...getDatasetColumns(fieldSchemas, { prefix: 'datasetRow.', expand }).map(
        column => ({
          ...column,
          hidden: hiddenColumnMap[column.key ?? ''] ?? false,
          canManage: true,
        }),
      ),
      ...getExperimentContrastColumns(experiments, {
        columnInfosMap: newColumnInfosMap,
        expand,
        spaceID,
        enableDelete: true,
        onExperimentChange,
        hiddenFieldMap: hiddenExperimentFieldMap,
        onRefresh: service.refresh,
      }).map(column => ({
        ...column,
        canManage: false,
      })),
      {
        title: I18n.t('operation'),
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
                onClick: () => {
                  setActiveItem(record);
                },
              },
            ]}
          />
        ),
      },
    ];
    setColumns(newColumns);
  }, [
    spaceID,
    experiments,
    columnInfosMap,
    expand,
    fieldSchemas,
    hiddenColumnMap,
    hiddenExperimentFieldMap,
  ]);

  useEffect(() => {
    const key = `experiment_contrast_detail_column_manage_${experimentIds?.[0]}`;
    const data = readLocalColumn(key);
    if (data?.hiddenColumnMap) {
      setHiddenColumnMap(data.hiddenColumnMap);
    }
    if (data?.hiddenFieldMap) {
      setHiddenExperimentFieldMap(data.hiddenFieldMap);
    }
  }, [experimentIds?.[0]]);

  const handleRefresh = () => service.refresh();

  return (
    <div className="h-full flex flex-col gap-3 overflow-hidden">
      <TableForExperiment
        service={service as Service}
        heightFull={true}
        header={
          <ExperimentContrastTableHeader
            experiments={experiments}
            columnInfosMap={columnInfosMap}
            columnManageStorageKey={columnManageStorageKey}
            columns={columns}
            logicFilter={logicFilter}
            setLogicFilter={onLogicFilterChange}
            expand={expand}
            setExpand={setExpand}
            hiddenFieldMap={hiddenExperimentFieldMap}
            setHiddenFieldMap={setHiddenExperimentFieldMap}
            setHiddenColumnMap={setHiddenColumnMap}
            onRefresh={handleRefresh}
          />
        }
        pageSizeStorageKey="experiment_contrast_page_size"
        tableProps={{
          className: styles['table-row-hover-show-icon'],
          rowKey: 'id',
          columns,
          bordered: true,
          onRow: record => ({
            onClick: () => {
              // 如果当前有选中的文本，不触发点击事件
              if (!window.getSelection()?.isCollapsed) {
                return;
              }
              setActiveItem(record);
            },
          }),
        }}
      />
      {activeItem ? (
        <ExperimentContrastItemDetail
          experiments={experiments}
          columnInfosMap={columnInfosMap}
          datasetFieldSchemas={fieldSchemas}
          activeItemStore={activeItemStore}
          spaceID={spaceID}
          onStepChange={onItemStepChange}
          onClose={() => setActiveItem(undefined)}
        />
      ) : null}
    </div>
  );
}
