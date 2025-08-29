/* eslint-disable @coze-arch/max-line-per-function */
import { useState } from 'react';

import { isEmpty } from 'lodash-es';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import {
  TableColActions,
  TableWithPagination,
  ColumnSelector,
  PrimaryPage,
} from '@cozeloop/components';
import { useNavigateModule, useSpace } from '@cozeloop/biz-hooks-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozIllusAdd,
  IconCozIllusNone,
} from '@coze-arch/coze-design/illustrations';
import { IconCozPlus, IconCozRefresh } from '@coze-arch/coze-design/icons';
import {
  Button,
  type ColumnProps,
  EmptyState,
  Modal,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import { DatasetDetailEditModal } from '../dataset-detail-edit-modal';
import { useDatasetList } from './use-dataset-list';
import { useColumnManage } from './use-column-manage';
import { ListFilter } from './list-filter';

export const DatasetList = () => {
  const { spaceID } = useSpace();
  const { onFilterChange, filter, service, setFilter } = useDatasetList();
  const navigate = useNavigateModule();
  const handleDatasetBaseInfoEdit = (row: EvaluationSet) => {
    navigate(`evaluation/datasets/${row.id}`);
  };
  const [selectedDataset, setSelectedDataset] = useState<EvaluationSet>();
  const isSearch = filter?.name || !isEmpty(filter?.creators);
  const handleDelete = (row: EvaluationSet) => {
    Modal.error({
      size: 'large',
      className: 'w-[420px]',
      type: 'dialog',
      title: '删除评测集',
      content: (
        <Typography.Text className="break-all">
          确定删除评测集
          <Typography.Text className="!font-medium mx-[2px]">
            {row.name}
          </Typography.Text>
          吗？此修改将不可逆。
        </Typography.Text>
      ),
      autoLoading: true,
      onOk: async () => {
        await StoneEvaluationApi.DeleteEvaluationSet({
          workspace_id: spaceID,
          evaluation_set_id: row.id as string,
        });
        service.refresh();
      },
      showCancelButton: true,
      cancelText: '取消',
      okText: '删除',
    });
  };

  const guards = useGuard({
    point: GuardPoint['eval.datasets.delete'],
  });

  const { columns, setColumns, defaultColumns } = useColumnManage();
  const allColumns: ColumnProps[] = [
    ...columns,
    {
      title: '操作',
      key: 'actions',
      width: 100,
      fixed: 'right',
      render: (_, record) => (
        <TableColActions
          actions={[
            {
              label: '详情',
              onClick: () => handleDatasetBaseInfoEdit(record),
            },
            {
              label: '删除',
              type: 'danger',
              onClick: () => handleDelete(record),
              disabled: guards.data.readonly,
            },
          ]}
          maxCount={1}
        />
      ),
    },
  ];

  return (
    <PrimaryPage
      pageTitle="评测集"
      filterSlot={
        <div className="flex justify-between">
          <ListFilter filter={filter} setFilter={onFilterChange} />
          <div className="flex gap-[8px]">
            <Tooltip content="刷新" theme="dark">
              <Button
                color="primary"
                icon={<IconCozRefresh />}
                onClick={() => {
                  service.refresh();
                }}
              ></Button>
            </Tooltip>
            <ColumnSelector
              columns={columns}
              defaultColumns={defaultColumns}
              onChange={setColumns}
            />
            <Button
              color="hgltplus"
              icon={<IconCozPlus />}
              onClick={() => {
                navigate('evaluation/datasets/create');
              }}
            >
              新建评测集
            </Button>
          </div>
        </div>
      }
    >
      <TableWithPagination<EvaluationSet>
        service={service}
        heightFull={true}
        tableProps={{
          rowKey: 'id',
          columns: allColumns,
          sticky: { top: 0 },
          onRow: record => ({
            onClick: () => handleDatasetBaseInfoEdit(record),
          }),
          onChange: data => {
            if (data.extra?.changeType === 'sorter') {
              setFilter({
                ...filter,
                order_bys:
                  data.sorter?.sortOrder === false
                    ? undefined
                    : [
                        {
                          field: data.sorter?.key,
                          is_asc: data.sorter?.sortOrder === 'ascend',
                        },
                      ],
              });
            }
          },
        }}
        empty={
          isSearch ? (
            <EmptyState
              size="full_screen"
              icon={<IconCozIllusNone />}
              title="未能找到相关结果"
              description={'请尝试其他关键词或修改筛选项'}
            />
          ) : (
            <EmptyState
              size="full_screen"
              icon={<IconCozIllusAdd />}
              title="暂无评测集"
              description={'点击右上角新建评测集按钮进行创建'}
            />
          )
        }
      />
      {selectedDataset ? (
        <DatasetDetailEditModal
          datasetDetail={selectedDataset}
          onSuccess={() => {
            setSelectedDataset(undefined);
            service.refresh();
          }}
          onCancel={() => {
            setSelectedDataset(undefined);
          }}
          visible={true}
          showTrigger={false}
        />
      ) : null}
    </PrimaryPage>
  );
};
