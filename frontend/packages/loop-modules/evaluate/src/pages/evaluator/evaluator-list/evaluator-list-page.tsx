// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */
import { useNavigate } from 'react-router-dom';
import { useMemo, useState } from 'react';

import { isEmpty } from 'lodash-es';
import dayjs from 'dayjs';
import classNames from 'classnames';
import { usePagination, useRequest } from 'ahooks';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, useGuards } from '@cozeloop/guard';
import { EvaluatorIcon } from '@cozeloop/evaluate-components';
import {
  type ColumnItem,
  TableColActions,
  TableWithPagination,
  UserProfile,
  DEFAULT_PAGE_SIZE,
  dealColumnsWithStorage,
  ColumnSelector,
  setColumnsManageStorage,
} from '@cozeloop/components';
import { useNavigateModule, useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  EvaluatorType,
  type Evaluator,
  type EvaluatorTemplate,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozIllusAdd,
  IconCozIllusEmpty,
} from '@coze-arch/coze-design/illustrations';
import {
  IconCozAi,
  IconCozArrowDown,
  IconCozCode,
  IconCozPlus,
  IconCozRefresh,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  EmptyState,
  Menu,
  Modal,
  Tag,
  Tooltip,
  Typography,
  type ColumnProps,
} from '@coze-arch/coze-design';

import { codeEvaluatorLanguageMapReverse } from '@/constants';

import { EvaluatorTypeTagText } from '../evaluator-template/types';
import { EvaluatorTemplateListPanel } from '../evaluator-template/evaluator-template-list-panel';
import { type FilterParams } from './types';
import { EvaluatorListFilter } from './evaluator-list-filter';

const columnManageStorageKey = 'evaluator_list_column_manage';

function EvaluatorListPage({ className }: { className?: string }) {
  const { spaceID } = useSpace();
  const navigate = useNavigate();
  const navigateModule = useNavigateModule();

  const [filterParams, setFilterParams] = useState<FilterParams>();
  const [defaultColumns, setDefaultColumns] = useState<ColumnItem[]>([]);
  const [templatePanelVisible, setTemplatePanelVisible] = useState(false);
  const [evaluatorType, setEvaluatorType] = useState<
    EvaluatorTypeTagText | undefined
  >(EvaluatorTypeTagText.Prompt);
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
      title: I18n.t('copy_evaluator_config'),
      content: I18n.t('copy_and_create_evaluator', {
        name: record.name,
      }),
      onOk: () => {
        if (record.evaluator_type === EvaluatorType.Code) {
          navigate(`create/code/${record.evaluator_id}`);
        } else {
          navigate(`create/llm/${record.evaluator_id}`);
        }
      },
      showCancelButton: true,
      cancelText: I18n.t('cancel'),
      okText: I18n.t('confirm'),
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
        title: I18n.t('name'),
        value: I18n.t('name'),
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
                {I18n.t('unsubmitted_changes')}
              </Tag>
            ) : null}
          </div>
        ),

        checked: true,
        disabled: true,
      },
      {
        title: I18n.t('type'),
        value: I18n.t('type'),
        dataIndex: 'evaluator_type',
        key: 'evaluator_type',
        width: 100,
        render: (text: Evaluator['evaluator_type']) => (
          <div className="flex items-center gap-1">
            <EvaluatorIcon iconSize={12} evaluatorType={text} />
            <span
              className="text-[13px] font-normal"
              style={{ color: 'var(--coz-fg-plus, rgba(8, 13, 30, 0.90))' }}
            >
              {text === EvaluatorType.Code ? 'Code' : 'LLM'}
            </span>
          </div>
        ),
        checked: true,
      },
      {
        title: I18n.t('latest_version'),
        value: I18n.t('latest_version'),
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
      {
        title: I18n.t('description'),
        value: I18n.t('description'),
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
        title: I18n.t('updater'),
        value: I18n.t('updater'),
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
        title: I18n.t('update_time'),
        value: I18n.t('update_time'),
        dataIndex: 'base_info.updated_at',
        sorter: true,
        key: 'updated_at',
        width: 200,
        render: (text: NonNullable<Evaluator['base_info']>['updated_at']) =>
          text ? dayjs(Number(text)).format('YYYY-MM-DD HH:mm:ss') : '-',
        checked: true,
      },
      {
        title: I18n.t('creator'),
        value: I18n.t('creator'),
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
        title: I18n.t('create_time'),
        value: I18n.t('create_time'),
        dataIndex: 'base_info.created_at',
        key: 'created_at',
        sorter: true,
        width: 200,
        render: (text: NonNullable<Evaluator['base_info']>['created_at']) =>
          text ? dayjs(Number(text)).format('YYYY-MM-DD HH:mm:ss') : '-',
        checked: true,
      },
      {
        title: I18n.t('operation'),
        value: I18n.t('operation'),
        key: 'action',
        width: 142,
        fixed: 'right',
        render: (_: unknown, record: Evaluator) => (
          <TableColActions
            actions={[
              {
                label: I18n.t('detail'),
                onClick: () => {
                  if (record.evaluator_type === EvaluatorType.Code) {
                    navigate(`code/${record.evaluator_id}`);
                  } else {
                    navigate(`${record.evaluator_id}`);
                  }
                },
              },
              {
                label: I18n.t('copy'),
                disabled:
                  guards.data[GuardPoint['eval.evaluators.copy']].readonly,
                onClick: () => handleCopy(record),
              },
              {
                label: I18n.t('delete'),
                type: 'danger',
                disabled:
                  guards.data[GuardPoint['eval.evaluators.delete']].readonly,
                onClick: () =>
                  Modal.error({
                    size: 'large',
                    className: 'w-[420px]',
                    title: I18n.t('confirm_delete_evaluator', {
                      name: record.name,
                    }),
                    content: I18n.t('caution_of_operation'),
                    onOk: () => deleteService.runAsync(record),
                    showCancelButton: true,
                    cancelText: I18n.t('cancel'),
                    okText: I18n.t('delete'),
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

  const tableOnChange = ({ sorter, extra }: Record<string, any>) => {
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
  };

  const handleApply = (
    template: EvaluatorTemplate,
    options?: {
      /** 仅当评估器类型为代码评估器，并且用户选择了语言类型时，此参数才会生效 */
      codeLanguageType?: string;
    },
  ) => {
    const isCode = template.evaluator_type === EvaluatorType.Code;
    if (isCode) {
      // FIXME: @wuwenqi 这里来自额外参数
      const languageType =
        codeEvaluatorLanguageMapReverse[options?.codeLanguageType || 'python'];
      navigateModule(
        `evaluation/evaluators/create/code?templateKey=${template.id}&templateLang=${languageType}`,
      );
    } else {
      navigateModule(
        `evaluation/evaluators/create/llm?templateKey=${template.id}`,
      );
    }
  };

  const handleOnClickCard = (template: EvaluatorTemplate) => {
    sendEvent(EVENT_NAMES.cozeloop_evaluator_sample_detail_view, {
      evaluator_sample_card_name: template?.name,
    });
  };

  const handleOnClickCreateLLM = () => {
    sendEvent(EVENT_NAMES.cozeloop_selfbuild_evaluator_type_choose, {
      evaluator_type: EvaluatorTypeTagText.Prompt,
    });

    setEvaluatorType(EvaluatorTypeTagText.Prompt);
    setTemplatePanelVisible(true);
  };

  const handleOnClickCreateCode = () => {
    sendEvent(EVENT_NAMES.cozeloop_selfbuild_evaluator_type_choose, {
      evaluator_type: EvaluatorTypeTagText.Code,
    });
    setEvaluatorType(EvaluatorTypeTagText.Code);
    setTemplatePanelVisible(true);
  };

  const filterSlot = (
    <div className="flex flex-row justify-between">
      <EvaluatorListFilter
        filterParams={filterParams}
        onFilter={setFilterParams}
      />

      <div className="flex flex-row items-center gap-[8px]">
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
          onChange={items => {
            setCurrentColumns(items.filter(i => i.checked));
            setColumnsManageStorage(columnManageStorageKey, items);
          }}
        />
        <Menu
          position="bottomRight"
          clickToHide={true}
          render={
            <Menu.SubMenu className="w-[150px]" mode="menu">
              <Menu.Item onClick={handleOnClickCreateLLM}>
                <div className="flex flex-row items-center">
                  <IconCozAi className="mr-1" />
                  <span>{I18n.t('evaluate_llm_evaluator')}</span>
                </div>
              </Menu.Item>
              <Menu.Item onClick={handleOnClickCreateCode}>
                <div className="flex flex-row items-center">
                  <IconCozCode className="mr-1" />
                  <span>{I18n.t('evaluate_code_evaluator')}</span>
                </div>
              </Menu.Item>
            </Menu.SubMenu>
          }
        >
          <Button type="primary" icon={<IconCozPlus />}>
            <span className="mr-1">{I18n.t('new_evaluator')}</span>
            <IconCozArrowDown />
          </Button>
        </Menu>
      </div>
    </div>
  );

  return (
    <div
      className={classNames(
        'flex flex-col h-full w-full overflow-hidden',
        className,
      )}
    >
      <div className="w-full flex-0 pt-4 pb-3">{filterSlot}</div>
      <div className="flex-1 h-full w-full flex flex-col gap-3 overflow-hidden">
        <TableWithPagination<Evaluator>
          service={service}
          heightFull={true}
          tableProps={{
            rowKey: 'evaluator_id',
            columns: currentColumns,
            sticky: { top: 0 },
            onRow: record => ({
              onClick: () => {
                if (record.evaluator_type === EvaluatorType.Code) {
                  navigate(`code/${record.evaluator_id}`);
                } else {
                  navigate(`${record.evaluator_id}`);
                }
              },
            }),
            onChange: tableOnChange,
          }}
          empty={
            isSearch ? (
              <EmptyState
                size="full_screen"
                icon={<IconCozIllusEmpty />}
                title={I18n.t('no_results_found')}
                description={I18n.t('try_other_keywords')}
              />
            ) : (
              <EmptyState
                size="full_screen"
                icon={<IconCozIllusAdd />}
                title={I18n.t('no_evaluator')}
                description={I18n.t('click_to_create')}
              />
            )
          }
        />
      </div>
      {templatePanelVisible ? (
        <EvaluatorTemplateListPanel
          defaultEvaluatorType={evaluatorType}
          onApply={handleApply}
          onClickCard={handleOnClickCard}
          onClose={() => {
            setEvaluatorType(undefined);
            setTemplatePanelVisible(false);
          }}
        />
      ) : null}
    </div>
  );
}

export default EvaluatorListPage;
