/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */
import { useNavigate } from 'react-router-dom';
import { useMemo, useState } from 'react';

import { isEmpty } from 'lodash-es';
import dayjs from 'dayjs';
import { usePagination, useRequest } from 'ahooks';
import { GuardPoint, useGuards } from '@cozeloop/guard';
import {
  type ColumnItem,
  TableColActions,
  TableWithPagination,
  PrimaryPage,
  UserProfile,
  DEFAULT_PAGE_SIZE,
  dealColumnsWithStorage,
  ColumnSelector,
  setColumnsManageStorage,
} from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozIllusAdd,
  IconCozIllusEmpty,
} from '@coze-arch/coze-design/illustrations';
import { IconCozPlus, IconCozRefresh } from '@coze-arch/coze-design/icons';
import {
  Button,
  EmptyState,
  Modal,
  Tag,
  Tooltip,
  Typography,
  type ColumnProps,
} from '@coze-arch/coze-design';

import { type FilterParams } from './types';
import { EvaluatorListFilter } from './evaluator-list-filter';

const columnManageStorageKey = 'evaluator_list_column_manage';

function EvaluatorListPage() {
  const { spaceID } = useSpace();
  const navigate = useNavigate();

  const [filterParams, setFilterParams] = useState<FilterParams>();
  const [defaultColumns, setDefaultColumns] = useState<ColumnItem[]>([]);
  const isSearch =
    filterParams?.search_name || !isEmpty(filterParams?.creator_ids);

  const guards = useGuards({
    points: [
      GuardPoint['eval.evaluators.copy'],
      GuardPoint['eval.evaluators.delete'],
    ],
  });

  const service = usePagination(
    async ({ current, pageSize }) => {
      const { evaluators, total } = await StoneEvaluationApi.ListEvaluators({
        workspace_id: spaceID,
        ...filterParams,
        page_size: pageSize,
        page_number: current,
      });

      return {
        list: evaluators || [],
        total: Number(total || 0),
      };
    },
    {
      defaultPageSize: DEFAULT_PAGE_SIZE,
      refreshDeps: [filterParams],
    },
  );

  const handleCopy = (record: Evaluator) => {
    Modal.info({
      size: 'large',
      className: 'w-[420px]',
      title: '复制评估器配置',
      content: `复制${record.name}配置，并新建评估器`,
      onOk: () => navigate(`create/${record.evaluator_id}`),
      showCancelButton: true,
      cancelText: '取消',
      okText: '确认',
    });
  };

  const deleteService = useRequest(
    async (record: Evaluator) =>
      await StoneEvaluationApi.DeleteEvaluator({
        workspace_id: spaceID,
        evaluator_id: record.evaluator_id ?? '',
      }),
    {
      manual: true,
      onSuccess: () => service.refresh(),
    },
  );

  const columns: ColumnItem[] = useMemo(() => {
    const newDefaultColumns: ColumnItem[] = [
      {
        title: '评估器名称',
        value: '评估器名称',
        dataIndex: 'name',
        key: 'name',
        width: 200,
        render: (text: Evaluator['name'], record: Evaluator) => (
          <div className="flex flex-row items-center">
            <Typography.Text
              className="flex-shrink"
              style={{
                fontSize: 'inherit',
              }}
              ellipsis={{ rows: 1, showTooltip: true }}
            >
              {text || '-'}
            </Typography.Text>
            {record.draft_submitted === false ? (
              <Tag
                color="yellow"
                className="ml-2 flex-shrink-0 !h-5 !px-2 !py-[2px] rounded-[3px] "
              >
                {'修改未提交'}
              </Tag>
            ) : null}
          </div>
        ),
        checked: true,
        disabled: true,
      },
      {
        title: '最新版本',
        value: '最新版本',
        dataIndex: 'latest_version',
        key: 'latest_version',
        width: 100,
        render: (text: Evaluator['latest_version']) =>
          text ? (
            <Tag
              color="primary"
              className="!h-5 !px-2 !py-[2px] rounded-[3px] mr-1"
            >
              {text}
            </Tag>
          ) : (
            '-'
          ),
        checked: true,
      },
      // {
      //   title: '类型',
      //   value: '类型',
      //   dataIndex: 'evaluator_type',
      //   key: 'evaluator_type',
      //   render: (text: Evaluator['evaluator_type']) =>
      //     text ? (
      //       <Tag color="brand">
      //         {
      //           // @ts-expect-error 类型问题
      //           evaluatorTypeMap[text]
      //         }
      //       </Tag>
      //     ) : (
      //       '-'
      //     ),
      // },
      {
        title: '描述',
        value: '描述',
        dataIndex: 'description',
        key: 'description',
        width: 285,
        render: (text: Evaluator['description']) => (
          <Typography.Text
            style={{ fontSize: 'inherit' }}
            ellipsis={{ rows: 1, showTooltip: true }}
          >
            {text || '-'}
          </Typography.Text>
        ),
        checked: true,
      },
      {
        title: '更新人',
        value: '更新人',
        dataIndex: 'base_info.updated_by',
        key: 'updated_by',
        width: 170,
        render: (text: NonNullable<Evaluator['base_info']>['updated_by']) =>
          text?.name ? (
            <UserProfile avatarUrl={text?.avatar_url} name={text?.name} />
          ) : (
            '-'
          ),
        checked: true,
      },
      {
        title: '更新时间',
        value: '更新时间',
        dataIndex: 'base_info.updated_at',
        sorter: true,
        key: 'updated_at',
        width: 200,
        render: (text: NonNullable<Evaluator['base_info']>['updated_at']) =>
          text ? dayjs(Number(text)).format('YYYY-MM-DD HH:mm:ss') : '-',
        checked: true,
      },
      {
        title: '创建人',
        value: '创建人',
        dataIndex: 'base_info.created_by',
        key: 'created_by',
        width: 170,
        render: (text: NonNullable<Evaluator['base_info']>['created_by']) =>
          text?.name ? (
            <UserProfile avatarUrl={text?.avatar_url} name={text?.name} />
          ) : (
            '-'
          ),
        checked: true,
      },
      {
        title: '创建时间',
        value: '创建时间',
        dataIndex: 'base_info.created_at',
        key: 'created_at',
        sorter: true,
        width: 200,
        render: (text: NonNullable<Evaluator['base_info']>['created_at']) =>
          text ? dayjs(Number(text)).format('YYYY-MM-DD HH:mm:ss') : '-',
        checked: true,
      },
      {
        title: '操作',
        value: '操作',
        key: 'action',
        width: 142,
        fixed: 'right',
        render: (_: unknown, record: Evaluator) => (
          <TableColActions
            actions={[
              {
                label: '详情',
                onClick: () => navigate(`${record.evaluator_id}`),
              },
              {
                label: '复制',
                disabled:
                  guards.data[GuardPoint['eval.evaluators.copy']].readonly,
                onClick: () => handleCopy(record),
              },
              {
                label: '删除',
                type: 'danger',
                disabled:
                  guards.data[GuardPoint['eval.evaluators.delete']].readonly,
                onClick: () =>
                  Modal.error({
                    size: 'large',
                    className: 'w-[420px]',
                    title: `确定删除评估器：${record.name}？`,
                    content: '此操作不可逆，请慎重操作',
                    onOk: () => deleteService.runAsync(record),
                    showCancelButton: true,
                    cancelText: '取消',
                    okText: '删除',
                  }),
              },
            ]}
            maxCount={2}
          />
        ),
        checked: true,
        disabled: true,
      },
    ];
    const newColumns: ColumnItem[] = dealColumnsWithStorage(
      columnManageStorageKey,
      newDefaultColumns,
    );
    setDefaultColumns(newDefaultColumns);
    return newColumns;
  }, []);

  const [currentColumns, setCurrentColumns] =
    useState<ColumnProps<Evaluator>[]>(columns);

  return (
    <PrimaryPage
      pageTitle="评估器"
      filterSlot={
        <div className="flex flex-row justify-between">
          <EvaluatorListFilter
            filterParams={filterParams}
            onFilter={setFilterParams}
          />
          <div className="flex flex-row items-center gap-[8px]">
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
              onChange={items => {
                setCurrentColumns(items.filter(i => i.checked));
                setColumnsManageStorage(columnManageStorageKey, items);
              }}
            />
            <Button
              type="primary"
              icon={<IconCozPlus />}
              onClick={() => navigate('create')}
            >
              {'新建评估器'}
            </Button>
          </div>
        </div>
      }
    >
      <div className="flex-1 h-full w-full flex flex-col gap-3 overflow-hidden">
        <TableWithPagination<Evaluator>
          service={service}
          heightFull={true}
          tableProps={{
            rowKey: 'evaluator_id',
            columns: currentColumns,
            sticky: { top: 0 },
            onRow: record => ({
              onClick: () => navigate(`${record.evaluator_id}`),
            }),
            onChange: ({ sorter, extra }) => {
              if (extra?.changeType === 'sorter' && sorter) {
                let field: string | undefined = undefined;
                switch (sorter.dataIndex) {
                  case 'base_info.created_at':
                    field = 'created_at';
                    break;
                  case 'base_info.updated_at':
                    field = 'updated_at';
                    break;
                  default:
                    break;
                }
                if (sorter.dataIndex) {
                  setFilterParams({
                    ...filterParams,
                    order_bys: sorter.sortOrder
                      ? [
                          {
                            field,
                            is_asc: sorter.sortOrder === 'ascend',
                          },
                        ]
                      : undefined,
                  });
                }
              }
            },
          }}
          empty={
            isSearch ? (
              <EmptyState
                size="full_screen"
                icon={<IconCozIllusEmpty />}
                title="未能找到相关结果"
                description={'请尝试其他关键词或修改筛选项'}
              />
            ) : (
              <EmptyState
                size="full_screen"
                icon={<IconCozIllusAdd />}
                title="暂无评估器"
                description={'点击右上角创建按钮进行创建'}
              />
            )
          }
        />
      </div>
    </PrimaryPage>
  );
}

export default EvaluatorListPage;
