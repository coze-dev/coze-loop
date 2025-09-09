// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useState } from 'react';

import { isEmpty } from 'lodash-es';
import { I18n } from '@cozeloop/i18n-adapter';
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
      title: I18n.t('delete_review_set'),
      content: (
        <Typography.Text className="break-all">
          {I18n.t('cozeloop_open_evaluate_confirm_delete_evaluation_set')}
          <Typography.Text className="!font-medium mx-[2px]">
            {row.name}
          </Typography.Text>
          {I18n.t('this_change_irreversible')}
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
      cancelText: I18n.t('global_btn_cancel'),
      okText: I18n.t('space_member_role_type_del_btn'),
    });
  };

  const guards = useGuard({
    point: GuardPoint['eval.datasets.delete'],
  });

  const { columns, setColumns, defaultColumns } = useColumnManage();
  const allColumns: ColumnProps[] = [
    ...columns,
    {
      title: I18n.t('operation'),
      key: 'actions',
      width: 100,
      fixed: 'right',
      render: (_, record) => (
        <TableColActions
          actions={[
            {
              label: I18n.t('detail'),
              onClick: () => handleDatasetBaseInfoEdit(record),
            },
            {
              label: I18n.t('delete'),
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
      pageTitle={I18n.t('evaluation_set')}
      filterSlot={
        <div className="flex justify-between">
          <ListFilter filter={filter} setFilter={onFilterChange} />
          <div className="flex gap-[8px]">
            <Tooltip content={I18n.t('refresh')} theme="dark">
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
              {I18n.t('create_evaluation_set')}
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
              title={I18n.t('failed_to_find_related_results')}
              description={I18n.t('try_other_keywords')}
            />
          ) : (
            <EmptyState
              size="full_screen"
              icon={<IconCozIllusAdd />}
              title={I18n.t('no_evaluation_dataset')}
              description={I18n.t('evaluation_set_click_the_button_to_create')}
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
