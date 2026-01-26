// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useCallback, useMemo, useRef, useState } from 'react';

import { useDebounce } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { PrimaryPage } from '@cozeloop/components';
import { StonePromptApi } from '@cozeloop/api-schema';
import { IconCozArrowDown, IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  Form,
  type FormApi,
  Button,
  withField,
  Search,
  Menu,
  Tooltip,
} from '@coze-arch/coze-design';

import { PromptTable, type PromptTableProps } from '../prompt-table';
import { promptDisplayColumns } from './column';

// 通过TypeScript工具类型提取changeInfo参数的类型
type TableChangeInfo = Parameters<
  NonNullable<PromptTableProps['onTableChange']>
>[0];

import styles from './index.module.less';
export type PromptTabKey = 'prompts' | 'snippet';
interface PromptListProps extends PromptTableProps {
  pageTitle?: string;
  extraSearchFormItems?: React.ReactNode;
  customCreateBtn?: React.ReactNode;
  hideSnippet?: boolean;
  defaultTabKey?: PromptTabKey;
  snippetColumns?: PromptTableProps['columns'];
  emptyPromptCreateBtn?: React.ReactNode;
  emptySnippetCreateBtn?: React.ReactNode;
  onCreatePromptClick?: () => void;
  onCreateSegmentClick?: () => void;
  onTabChange?: (key: PromptTabKey) => void;
}

type PromptSearchProps = NonNullable<PromptTableProps['filterRecord']>;

const FormSearch = withField(Search);

export function PromptList({
  pageTitle = I18n.t('prompt_development'),
  extraSearchFormItems,
  onCreatePromptClick,
  columns: propsColumns,
  snippetColumns: propsSnippetColumns,
  customCreateBtn,
  onTableChange: propsOnTableChange,
  hideSnippet,
  onCreateSegmentClick,
  defaultTabKey = 'prompts',
  onTabChange,
  emptyPromptCreateBtn,
  emptySnippetCreateBtn,
  ...rest
}: PromptListProps) {
  const formApi = useRef<FormApi<PromptSearchProps>>();
  const [filterRecord, setFilterRecord] = useState<PromptSearchProps>();
  const debouncedFilterRecord = useDebounce(filterRecord, { wait: 300 });
  const [tabKey, setTabKey] = useState<PromptTabKey>(defaultTabKey);

  const onFilterValueChange = (allValues?: PromptSearchProps) => {
    setFilterRecord({ ...allValues });
  };

  const onTabarChange = (key: PromptTabKey) => {
    setTabKey(key);
    onTabChange?.(key);
  };

  const isSnippet = tabKey === 'snippet';

  const columns =
    (isSnippet ? propsSnippetColumns : propsColumns) || promptDisplayColumns;

  const onTableChange = useCallback(({ sorter, extra }: TableChangeInfo) => {
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
        formApi.current?.setValue('asc', sorter.sortOrder !== 'descend');
      } else {
        formApi.current?.setValue('order_by', undefined);
        formApi.current?.setValue('asc', undefined);
      }
    }
  }, []);

  const tabbarDom = useMemo(
    () => (
      <div className="flex gap-2">
        <Button
          color={tabKey === 'prompts' ? 'highlight' : 'secondary'}
          onClick={() => onTabarChange('prompts')}
        >
          Prompt
        </Button>
        <Button
          color={isSnippet ? 'highlight' : 'secondary'}
          onClick={() => onTabarChange('snippet')}
        >
          {I18n.t('prompt_prompt_snippet')}
        </Button>
      </div>
    ),

    [tabKey],
  );

  const filterForm = useMemo(
    () => (
      <Form<PromptSearchProps>
        className={styles['prompt-form']}
        onValueChange={onFilterValueChange}
        getFormApi={api => (formApi.current = api)}
      >
        <FormSearch
          field="key_word"
          placeholder={I18n.t('search_prompt_key_or_prompt_name')}
          width={360}
          noLabel
        />

        {extraSearchFormItems}
      </Form>
    ),

    [],
  );

  return (
    <PrimaryPage
      pageTitle={pageTitle}
      filterSlot={
        <div className="flex align-center justify-between" data-btm="c20080">
          {hideSnippet ? filterForm : tabbarDom}
          {customCreateBtn ?? (
            <Menu
              className="w-[145px]"
              render={
                <Menu.SubMenu mode="menu">
                  <Menu.Item
                    itemKey={I18n.t('prompt_blank_prompt')}
                    onClick={onCreatePromptClick}
                    data-btm="d75966"
                  >
                    {I18n.t('prompt_blank_prompt')}
                  </Menu.Item>
                  {onCreateSegmentClick ? (
                    <Tooltip
                      content={I18n.t('prompt_prompt_snippet_nesting_support')}
                      position="left"
                    >
                      <Menu.Item
                        itemKey={I18n.t('prompt_prompt_snippet')}
                        onClick={onCreateSegmentClick}
                      >
                        {I18n.t('prompt_prompt_snippet')}
                      </Menu.Item>
                    </Tooltip>
                  ) : null}
                </Menu.SubMenu>
              }
            >
              <Button icon={<IconCozPlus />}>
                {I18n.t('create_prompt')}
                <IconCozArrowDown className="ml-1" />
              </Button>
            </Menu>
          )}
        </div>
      }
    >
      <div className="w-full h-full overflow-hidden flex flex-1 flex-col">
        {!hideSnippet ? <div className="pb-3">{filterForm}</div> : null}
        <PromptTable
          key={tabKey}
          {...rest}
          columns={columns}
          filterRecord={debouncedFilterRecord}
          onTableChange={propsOnTableChange || onTableChange}
          isSnippet={isSnippet}
          emptyPromptCreateBtn={
            emptyPromptCreateBtn ?? (
              <Button onClick={onCreatePromptClick} icon={<IconCozPlus />}>
                {I18n.t('create_prompt')}
              </Button>
            )
          }
          emptySnippetCreateBtn={
            emptySnippetCreateBtn ?? (
              <Button onClick={onCreateSegmentClick} icon={<IconCozPlus />}>
                {I18n.t('prompt_create_prompt_snippet')}
              </Button>
            )
          }
        />
      </div>
    </PrimaryPage>
  );
}
