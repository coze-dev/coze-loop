// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */

import { type ReactNode, useEffect, useState } from 'react';

import { usePagination, useUpdateEffect } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type Version,
  type ColumnItem,
  setColumnsManageStorage,
  dealColumnsWithStorage,
  IDRender,
} from '@cozeloop/components';
import {
  type EvaluationSetItem,
  type FieldSchema,
  type EvaluationSet,
  type FieldData,
  type OrderBy,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tooltip, Typography, type ColumnProps } from '@coze-arch/coze-design';

import { DatasetSingleColumnEdit } from '../dataset-column-edit/single-column-edit';
import { useDatasetItemExpand } from '../../dataset-item-expand/use-dataset-item-expand';
import { DatasetFieldItemRender } from '../../dataset-item/dataset-field-render';
import { getDatasetColumnSortStorageKey } from '../../../utils/column-manage/dataset-column-storage';
import { useSearchParam } from '../../../hooks/use-search-param';
import { DEFAULT_PAGE_SIZE } from '../../../const';
export interface EvaluationSetItemTableData extends EvaluationSetItem {
  trunFieldData: {
    id?: string;
    fieldDataMap: Record<string, FieldData>;
  };
}
export const DRAFT_VERSION = 'draft';

export const useDatasetItemList = ({
  datasetDetail,
  spaceID,
  refreshDatasetDetail,
}: {
  datasetDetail?: EvaluationSet;
  spaceID: string;
  refreshDatasetDetail: () => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
}): any => {
  const { getSearchParams } = useSearchParam();
  const defaultVersionID = getSearchParams('version');
  /** fieldSchemas 默认值 */
  const defaultVersion = defaultVersionID
    ? {
        id: defaultVersionID,
        isDraft: false,
      }
    : {
        id: DRAFT_VERSION,
        isDraft: true,
        version: '0.0.0',
        description: I18n.t('current_draft'),
      };
  const draftFieldSchemas =
    datasetDetail?.evaluation_set_version?.evaluation_set_schema
      ?.field_schemas || [];
  // 当前版本，默认为草稿
  const [currentVersion, setCurrentVersion] = useState<Version>(defaultVersion);
  const isDraftVersion = currentVersion?.id === DRAFT_VERSION;
  const [orderBy, setOrderBy] = useState<OrderBy>();
  // 获取表格行数据
  const service = usePagination(
    async (paginationData: {
      current: number;
      pageSize?: number;
    }): Promise<{
      total: number;
      list: EvaluationSetItemTableData[];
      latestFieldSchemas?: FieldSchema[];
    }> => {
      const { current, pageSize } = paginationData;
      let schemaData: FieldSchema[] = draftFieldSchemas;
      if (currentVersion?.id !== DRAFT_VERSION) {
        const fieldVersionData =
          await StoneEvaluationApi.GetEvaluationSetVersion({
            evaluation_set_id: datasetDetail?.id as string,
            workspace_id: spaceID,
            version_id: currentVersion?.id,
          });
        schemaData =
          fieldVersionData?.version?.evaluation_set_schema?.field_schemas || [];
        if (!currentVersion?.version) {
          setCurrentVersion({
            ...currentVersion,
            version: fieldVersionData?.version?.version,
            description: fieldVersionData?.version?.description,
          });
        }
      }
      const res = await StoneEvaluationApi.ListEvaluationSetItems({
        evaluation_set_id: datasetDetail?.id as string,
        workspace_id: spaceID,
        page_number: current,
        page_size: pageSize ?? DEFAULT_PAGE_SIZE,
        version_id:
          currentVersion?.id === DRAFT_VERSION ? undefined : currentVersion?.id,
        order_bys: orderBy ? [orderBy] : undefined,
      });
      const tableData = convertEvaluationSetItemListToTableData(
        res.items ?? [],
        schemaData ?? [],
      );
      return {
        total: isNaN(Number(res.total)) ? 0 : Number(res.total),
        list: tableData,
        latestFieldSchemas: schemaData,
      };
    },
    {
      defaultPageSize: DEFAULT_PAGE_SIZE,
      refreshDeps: [draftFieldSchemas, currentVersion?.id, orderBy],
    },
  );
  // useEffect(() => {
  //   service.refresh();
  // }, [draftFieldSchemas,]);

  const isEmptyDataset = service.data?.total === 0;

  const latestFieldSchemas = service.data?.latestFieldSchemas;

  /** 展开收起节点 */
  const { ExpandNode, expand } = useDatasetItemExpand();

  const [selectedItem, setSelectedItem] = useState<{
    item?: EvaluationSetItemTableData;
    isEdit: boolean;
    index: number;
  }>({
    isEdit: false,
    index: 0,
  });
  const onSwitch = (type: 'pre' | 'next') => {
    if (type === 'next') {
      setSelectedItem({
        item: service?.data?.list?.[selectedItem.index + 1],
        index: selectedItem.index + 1,
        isEdit: selectedItem.isEdit,
      });
    } else {
      setSelectedItem({
        item: service?.data?.list?.[selectedItem.index - 1],
        index: selectedItem.index - 1,
        isEdit: selectedItem.isEdit,
      });
    }
  };

  const switchConfig = {
    canSwithPre: selectedItem.index > 0,
    canSwithNext:
      (service?.data?.list &&
        selectedItem.index < service?.data?.list?.length - 1) ||
      false,
    onSwith: onSwitch,
  };

  const { defaultColumnsItems: initDefaultColumnsItems, sortColumns } =
    getDefaultColumnsItems({
      fieldSchemas: draftFieldSchemas ?? [],
      expand,
      refresh: refreshDatasetDetail,
      datasetDetail,
      isDraftVersion,
    });
  const [defaultColumnsItems, setDefaultColumnsItems] = useState<ColumnItem[]>(
    initDefaultColumnsItems,
  );

  const [columns, setColumns] = useState<ColumnItem[]>(sortColumns);
  const storageKey = getDatasetColumnSortStorageKey(
    datasetDetail?.id as string,
  );
  const onColumnsChange = (newColumns: ColumnItem[]) => {
    setColumns(newColumns);
    setColumnsManageStorage(storageKey, newColumns);
  };

  useUpdateEffect(() => {
    const newColumns = columns?.map(column => {
      const fieldSchema = latestFieldSchemas?.find(
        field => field.key === column.key,
      );
      if (fieldSchema) {
        return {
          ...column,
          ...getFieldColumnConfig({
            field: fieldSchema,
            prefix: 'trunFieldData.fieldDataMap.',
            expand,
            editNode: isDraftVersion ? (
              <DatasetSingleColumnEdit
                currentField={fieldSchema}
                onRefresh={refreshDatasetDetail}
                datasetDetail={datasetDetail}
                disabledDataTypeSelect={!isEmptyDataset}
              />
            ) : null,
          }),
        };
      }
      return column;
    });
    setColumns(newColumns);
  }, [expand]);

  useEffect(() => {
    const {
      sortColumns: newSortColumns,
      defaultColumnsItems: newDefaultColumnsItems,
    } = getDefaultColumnsItems({
      fieldSchemas: latestFieldSchemas ?? [],
      expand,
      refresh: refreshDatasetDetail,
      datasetDetail,
      isDraftVersion,
      isEmptyDataset,
    });
    setColumns(newSortColumns);
    setDefaultColumnsItems(newDefaultColumnsItems);
  }, [latestFieldSchemas]);
  return {
    service,
    fieldSchemas: latestFieldSchemas ?? [],
    setColumns: onColumnsChange,
    columns,
    selectedItem,
    setSelectedItem,
    defaultColumnsItems,
    currentVersion,
    setCurrentVersion,
    ExpandNode,
    switchConfig,
    setOrderBy,
  };
};

export const getFieldColumnConfig = ({
  field,
  prefix,
  expand,
  editNode,
}: {
  field: FieldSchema;
  prefix?: string;
  expand?: boolean;
  editNode?: ReactNode;
}) => ({
  title: (
    <div className="flex items-center group justify-between">
      <div className="flex-1 flex overflow-hidden items-center">
        <Typography.Text
          ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
          style={{
            fontWeight: 'inherit',
            fontSize: 'inherit',
            color: 'inherit',
          }}
        >
          {field.name}
        </Typography.Text>
        {field?.description ? (
          <Tooltip content={field?.description} theme="dark">
            <div className="h-[16px] ml-1 text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)] flex items-center">
              <IconCozInfoCircle />
            </div>
          </Tooltip>
        ) : null}
      </div>
      {editNode ? (
        <div className="text-right w-[16px] invisible leading-[10px] group-hover:visible">
          {editNode}
        </div>
      ) : null}
    </div>
  ),
  key: field.key || '',
  displayName: field.name || '',
  width: 200,
  dataIndex: `${prefix}${field.key}`,
  render: (row: FieldData) => (
    <DatasetFieldItemRender
      fieldSchema={field}
      fieldData={row}
      expand={expand}
    />
  ),
});

export const convertEvaluationSetItemListToTableData = (
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
        if (!fieldSchema.key) {
          return;
        }
        const fieldData = turn?.field_data_list?.find(
          field => field.key === fieldSchema.key,
        );
        fieldDataMap[fieldSchema.key] = fieldData;
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

export const getDefaultColumnsItems = ({
  fieldSchemas,
  expand,
  refresh,
  datasetDetail,
  isDraftVersion,
  isEmptyDataset,
}: {
  fieldSchemas: FieldSchema[];
  expand: boolean;
  refresh: () => void;
  datasetDetail?: EvaluationSet;
  isDraftVersion: boolean;
  isEmptyDataset?: boolean;
}) => {
  const defaultColumns: (ColumnProps & { disabled?: boolean })[] = [
    {
      title: 'ID',
      key: 'ID',
      displayName: 'ID',
      dataIndex: 'item_id',
      width: 120,
      fixed: 'left',
      checked: true,
      disabled: true,
      render: (id: string) => <IDRender useTag={true} id={id} />,
    },
    ...(fieldSchemas?.map(field =>
      getFieldColumnConfig({
        field,
        prefix: 'trunFieldData.fieldDataMap.',
        expand,
        editNode: isDraftVersion ? (
          <DatasetSingleColumnEdit
            currentField={field}
            onRefresh={refresh}
            datasetDetail={datasetDetail}
            disabledDataTypeSelect={!isEmptyDataset}
          />
        ) : null,
      }),
    ) || []),
  ];
  const defaultColumnsItems: ColumnItem[] = defaultColumns.map(item => ({
    ...item,
    key: item.key as string,
    value: item.displayName as string,
    disabled: item.disabled || false,
    checked: true,
  }));
  const storageKey = getDatasetColumnSortStorageKey(
    datasetDetail?.id as string,
  );
  const sortColumnList = dealColumnsWithStorage(storageKey, [
    ...defaultColumnsItems,
  ]);
  return {
    defaultColumnsItems,
    sortColumns: sortColumnList,
  };
};
