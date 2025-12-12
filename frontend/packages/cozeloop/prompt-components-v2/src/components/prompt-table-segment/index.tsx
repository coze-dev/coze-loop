import { usePagination } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { DEFAULT_PAGE_SIZE, TableWithPagination } from '@cozeloop/components';
import {
  PromptType,
  type ListPromptRequest,
  type Prompt,
} from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import {
  IconCozIllusAdd,
  IconCozIllusEmpty,
} from '@coze-arch/coze-design/illustrations';
import {
  type ColumnProps,
  EmptyState,
  type TableProps,
} from '@coze-arch/coze-design';

export interface PromptTableSegmentProps {
  spaceID: string;
  refreshFlag?: number;
  columns?: ColumnProps<Prompt>[];
  filterRecord?: Omit<
    ListPromptRequest,
    'page_num' | 'page_size' | 'workspace_id'
  >;

  onTableRow?: NonNullable<TableProps['tableProps']>['onRow'];
  onTableChange?: NonNullable<TableProps['tableProps']>['onChange'];
}

export function PromptTableSegment({
  spaceID,
  columns,
  filterRecord,
  refreshFlag,
  onTableRow,
  onTableChange,
}: PromptTableSegmentProps) {
  const service = usePagination(
    ({ current, pageSize }) =>
      StonePromptApi.ListPrompt({
        workspace_id: spaceID,
        page_num: current,
        page_size: pageSize,
        filter_prompt_types: [PromptType.Snippet],
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
      refreshDeps: [filterRecord, spaceID, refreshFlag],
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
          <EmptyState
            size="full_screen"
            icon={<IconCozIllusAdd />}
            title={I18n.t('no_prompt')}
            description={I18n.t('click_to_create')}
          />
        )
      }
    />
  );
}
