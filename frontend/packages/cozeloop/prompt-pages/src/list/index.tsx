// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-len */
/* eslint-disable @coze-arch/max-line-per-function */
import { useRef, useState } from 'react';

import { useDebounce, usePagination } from 'ahooks';
import { PromptCreate } from '@cozeloop/prompt-components';
import { Guard, GuardPoint, useGuard, useGuards } from '@cozeloop/guard';
import {
  DEFAULT_PAGE_SIZE,
  PrimaryPage,
  TableColActions,
  TableWithPagination,
} from '@cozeloop/components';
import {
  useBaseURL,
  useNavigateModule,
  useSpace,
} from '@cozeloop/biz-hooks-adapter';
import { UserSelect } from '@cozeloop/biz-components-adapter';
import { useModalData } from '@cozeloop/base-hooks';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import { StonePromptApi, type promptManage } from '@cozeloop/api-schema';
import {
  IconCozIllusAdd,
  IconCozIllusEmpty,
} from '@coze-arch/coze-design/illustrations';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  Button,
  type ColumnProps,
  EmptyState,
  Form,
  type FormApi,
  Search,
  withField,
} from '@coze-arch/coze-design';

import { PromptDelete } from '@/components/prompt-delete';

import { columns } from './column';

import styles from './index.module.less';

const FormUserSelect = withField(UserSelect);
const FormSearch = withField(Search);

interface PromptSearchProps {
  key_word?: string;
  order_by?: promptManage.ListPromptOrderBy;
  asc?: boolean;
  created_bys?: string[];
}

export function PromptList() {
  const navigate = useNavigateModule();
  const { spaceID } = useSpace();
  // const globalDisabled = useGuard({ point: GuardPoint['pe.prompt.global'] });
  const creatorSearchGuard = useGuard({
    point: GuardPoint['pe.prompts.search_by_creator'],
  });

  const createModal = useModalData<Prompt>();
  const formApi = useRef<FormApi<PromptSearchProps>>();
  const [filterRecord, setFilterRecord] = useState<PromptSearchProps>();
  const debouncedFilterRecord = useDebounce(filterRecord, { wait: 300 });

  const service = usePagination(
    ({ current, pageSize }) =>
      StonePromptApi.ListPrompt({
        workspace_id: spaceID,
        page_num: current,
        page_size: pageSize,
        ...debouncedFilterRecord,
      }).then(res => {
        const newList = res.prompts?.map(it => {
          const user = res.users?.find(
            u => u.user_id === it?.prompt_basic?.created_by,
          );
          return { ...it, user };
        });
        return {
          list: newList || [],
          total: Number(res.total || 0),
        };
      }),
    {
      defaultPageSize: DEFAULT_PAGE_SIZE,
      refreshDeps: [debouncedFilterRecord, spaceID],
    },
  );

  const deleteModal = useModalData<Prompt>();

  const { baseURL } = useBaseURL();

  const guard = useGuards({
    points: [GuardPoint['pe.prompts.delete'], GuardPoint['pe.prompts.history']],
  });

  const operateCol: ColumnProps<Prompt> = {
    title: '操作',
    key: 'action',
    dataIndex: 'action',
    width: 160,
    align: 'left',
    fixed: 'right',
    render: (_: unknown, row: Prompt) => (
      <TableColActions
        actions={[
          {
            label: '详情',
            onClick: () => navigate(`pe/prompts/${row.id}`),
          },
          {
            label: '调用记录',
            disabled: guard.data['pe.prompts.history'].readonly,
            onClick: () =>
              window.open(
                `${baseURL}/observation/traces?relation=and&selected_span_type=root_span&trace_filters=%257B%2522query_and_or%2522%253A%2522and%2522%252C%2522filter_fields%2522%253A%255B%257B%2522field_name%2522%253A%2522prompt_key%2522%252C%2522logic_field_name_type%2522%253A%2522prompt_key%2522%252C%2522query_type%2522%253A%2522in%2522%252C%2522values%2522%253A%255B%2522${row.prompt_key}%2522%255D%257D%255D%257D&trace_platform=prompt`,
              ),
          },
          {
            label: '删除',
            onClick: () => {
              if (row?.id) {
                deleteModal.open(row);
              }
            },
            type: 'danger',
          },
        ]}
      />
    ),
  };

  const newColumns = [...columns, operateCol];

  const onFilterValueChange = (allValues?: PromptSearchProps) => {
    setFilterRecord({ ...allValues });
  };

  return (
    <PrimaryPage
      pageTitle="Prompt 开发"
      filterSlot={
        <div className="flex align-center justify-between">
          <Form<PromptSearchProps>
            className={styles['prompt-form']}
            onValueChange={onFilterValueChange}
            getFormApi={api => (formApi.current = api)}
          >
            <FormSearch
              field="key_word"
              placeholder="搜索 Prompt Key 或 Prompt 名称"
              width={360}
              noLabel
            />
            {!creatorSearchGuard.data.readonly && (
              <FormUserSelect
                field="created_bys"
                placeholder="所有创建人"
                noLabel
              />
            )}
          </Form>

          <Guard point={GuardPoint['pe.prompts.create']}>
            <Button icon={<IconCozPlus />} onClick={() => createModal.open()}>
              创建 Prompt
            </Button>
          </Guard>
        </div>
      }
    >
      <TableWithPagination
        heightFull
        service={service}
        tableProps={{
          columns: newColumns,
          sticky: { top: 0 },
          onRow: row => ({
            onClick: () => {
              navigate(`pe/prompts/${row.id}`);
            },
          }),
          onChange: ({ sorter, extra }) => {
            if (extra?.changeType === 'sorter' && sorter) {
              const arr = [
                'prompt_basic.created_at',
                'prompt_basic.latest_committed_at',
              ];
              if (arr.includes(sorter.dataIndex) && sorter.sortOrder) {
                const orderBy =
                  sorter.dataIndex === 'prompt_basic.created_at'
                    ? StonePromptApi.ListPromptOrderBy.CreatedAt
                    : StonePromptApi.ListPromptOrderBy.CommitedAt;
                formApi.current?.setValue('order_by', orderBy);
                formApi.current?.setValue(
                  'asc',
                  sorter.sortOrder !== 'descend',
                );
              } else {
                formApi.current?.setValue('order_by', undefined);
                formApi.current?.setValue('asc', undefined);
              }
            }
          },
        }}
        empty={
          debouncedFilterRecord?.key_word ? (
            <EmptyState
              size="full_screen"
              icon={<IconCozIllusEmpty />}
              title="未能找到相关结果"
              description="请尝试其他关键词或修改筛选项"
            />
          ) : (
            <EmptyState
              size="full_screen"
              icon={<IconCozIllusAdd />}
              title="暂无 Prompt"
              description="点击右上角创建按钮进行创建"
            />
          )
        }
      />
      <PromptCreate
        visible={createModal.visible}
        onCancel={createModal.close}
        onOk={res => {
          createModal.close();
          service.refresh();
          navigate(`pe/prompts/${res.id}`);
        }}
      />
      <PromptDelete
        data={deleteModal.data}
        visible={deleteModal.visible}
        onCacnel={deleteModal.close}
        onOk={() => {
          deleteModal.close();
          service.refresh();
        }}
      />
    </PrimaryPage>
  );
}
