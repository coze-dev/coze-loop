// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { usePagination } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { DEFAULT_PAGE_SIZE, TableWithPagination } from '@cozeloop/components';
import {
  PromptType,
  type ListPromptRequest,
  type Prompt,
} from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import { IconCozIllusEmpty } from '@coze-arch/coze-design/illustrations';
import {
  type ColumnProps,
  EmptyState,
  type TableProps,
  Image,
  Typography,
} from '@coze-arch/coze-design';

import snippetEmpty from '@/assets/snippet-empty.webp';
import promptEmpty from '@/assets/prompt-empty.webp';

export interface PromptTableProps {
  spaceID: string;
  refreshFlag?: number;
  columns?: ColumnProps<Prompt>[];
  filterRecord?: Omit<
    ListPromptRequest,
    'page_num' | 'page_size' | 'workspace_id'
  >;

  onTableRow?: NonNullable<TableProps['tableProps']>['onRow'];
  onTableChange?: NonNullable<TableProps['tableProps']>['onChange'];
  isSnippet?: boolean;
  emptyPromptCreateBtn?: React.ReactNode;
  emptySnippetCreateBtn?: React.ReactNode;
}

export function PromptTable({
  spaceID,
  columns,
  filterRecord,
  refreshFlag,
  onTableRow,
  onTableChange,
  isSnippet,
  emptyPromptCreateBtn,
  emptySnippetCreateBtn,
}: PromptTableProps) {
  const service = usePagination(
    ({ current, pageSize }) =>
      StonePromptApi.ListPrompt({
        workspace_id: spaceID,
        page_num: current,
        page_size: pageSize,
        filter_prompt_types: isSnippet
          ? [PromptType.Snippet]
          : [PromptType.Normal],
        ...filterRecord,
      })
        .then(res => {
          const newList = res.prompts?.map(it => {
            const user = res.users?.find(
              u => u.user_id === it?.prompt_basic?.created_by,
            );
            const lastUpdateUser = res.users?.find(
              u => u.user_id === it?.prompt_basic?.updated_by,
            );
            return { ...it, user, lastUpdateUser };
          });
          return {
            list: newList || [],
            total: Number(res.total || 0),
          };
        })
        .catch(() => ({
          list: [],
          total: 0,
        })),
    {
      defaultPageSize: DEFAULT_PAGE_SIZE,
      refreshDeps: [filterRecord, spaceID, refreshFlag, isSnippet],
    },
  );

  return (
    <TableWithPagination<Prompt>
      heightFull
      service={service}
      tableProps={{
        columns,
        sticky: { top: 0 },
        onRow: onTableRow,
        onChange: onTableChange,
      }}
      empty={
        filterRecord?.key_word ? (
          <EmptyState
            size="full_screen"
            icon={<IconCozIllusEmpty />}
            title={I18n.t('no_results_found')}
            description={I18n.t('try_other_keywords')}
          />
        ) : (
          <div className="flex flex-col gap-5 items-center justify-center">
            <Image
              src={isSnippet ? snippetEmpty : promptEmpty}
              width="680px"
              height="326px"
              preview={false}
            />

            <div className="flex flex-col gap-3 items-center justify-center">
              <Typography.Title heading={4}>
                {isSnippet
                  ? I18n.t('prompt_no_prompt_snippet')
                  : I18n.t('no_prompt')}
              </Typography.Title>
              <Typography.Text
                className="text-[20px] leading-[28px]"
                type="secondary"
              >
                {isSnippet ? (
                  <>
                    {I18n.t('prompt_prompt_snippet_reuse_support')}
                    <Typography.Text
                      link
                      className="text-[20px] leading-[28px] ml-1"
                    >
                      {I18n.t('prompt_view_documentation')}
                    </Typography.Text>
                  </>
                ) : (
                  I18n.t('prompt_full_process_prompt_support')
                )}
              </Typography.Text>
              {isSnippet ? emptySnippetCreateBtn : emptyPromptCreateBtn}
            </div>
          </div>
        )
      }
    />
  );
}
