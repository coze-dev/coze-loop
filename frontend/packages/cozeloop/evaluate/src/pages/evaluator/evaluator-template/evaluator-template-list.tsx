// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import classNames from 'classnames';
import { useDebounceFn, useLatest, usePagination } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type EvaluatorTemplate,
  EvaluatorTagKey,
} from '@cozeloop/api-schema/evaluation';
import { IconCozEmpty } from '@coze-arch/coze-design/icons';
import {
  Button,
  CozPagination,
  Empty,
  Skeleton,
  Spin,
} from '@coze-arch/coze-design';

import {
  type EvaluatorTypeTagText,
  type ListTemplatesParams,
  type TemplateFilter,
} from './types';
import { EvaluatorTemplateNameSearchInput } from './evaluator-template-name-search';
import {
  EvaluatorTemplateFilter,
  type EvaluatorTemplateFilterProps,
} from './evaluator-template-filter';
import { EvaluatorTemplateCard } from './evaluator-template-card';

const DEFAULT_PAGE_SIZE = 12;

interface PaginationParams {
  pageSize: number;
  current: number;
}

interface EvaluatorTemplateListProps {
  /** 列数 */
  colCount?: number;
  /** 默认评估器类型 */
  defaultEvaluatorType?: EvaluatorTypeTagText;
  /** 禁用评估器类型 */
  disabledEvaluatorTypes?: EvaluatorTypeTagText[];
  /** 列表头操作 */
  listHeaderAction?: (params: {
    filters: TemplateFilter | undefined;
  }) => React.ReactNode;
  filterOptions?: Pick<
    EvaluatorTemplateFilterProps,
    'showClearAll' | 'showScenariosClear' | 'isTypeSingleSelect' | 'tagType'
  >;
  className?: string;
  /** 列表头操作 */
  getCardHeaderActions?: (template: EvaluatorTemplate) => React.ReactNode;
  listTemplates: (params: ListTemplatesParams) => Promise<{
    list: EvaluatorTemplate[];
    total: number;
  }>;
  onApply?: (evaluatorTemplate: EvaluatorTemplate) => void;
  onClickCard?: (evaluatorTemplate: EvaluatorTemplate) => void;
}

function EmptyTips({ onClearFilter }: { onClearFilter: () => void }) {
  return (
    <div className="flex items-center justify-center w-full h-full">
      <Empty
        description={
          <div className="w-[320px] flex flex-col items-center">
            <IconCozEmpty className="w-8 h-8 coz-fg-dim" />
            <div className="text-[14px] coz-fg-primary font-medium mt-1 mb-3">
              {I18n.t('evaluator_template_filter_empty_tips')}
            </div>
            <Button color="primary" onClick={onClearFilter}>
              {I18n.t('clear_filter')}
            </Button>
          </div>
        }
      />
    </div>
  );
}

/** 评估器模板列表状态管理 */
function useListStore({
  defaultEvaluatorType,
  listTemplates,
}: {
  defaultEvaluatorType?: EvaluatorTypeTagText;
  listTemplates: EvaluatorTemplateListProps['listTemplates'];
}) {
  const { spaceID } = useSpace();
  const [searchKeyword, setSearchKeyword] = useState('');
  const [filters, setFilters] = useState<TemplateFilter | undefined>(
    defaultEvaluatorType !== undefined
      ? { [EvaluatorTagKey.Category]: [defaultEvaluatorType] }
      : undefined,
  );
  const [firstLoaded, setFirstLoaded] = useState(false);
  const firstLoadRef = useLatest(firstLoaded);

  const service = usePagination(
    async (params: PaginationParams) => {
      try {
        const res = await listTemplates({
          workspace_id: spaceID,
          search_keyword: searchKeyword,
          filters,
          page_size: params?.pageSize,
          page_number: params?.current,
        });
        return res;
      } catch (err) {
        console.error(err);
        return {
          list: [],
          total: 0,
        };
      } finally {
        setFirstLoaded(true);
      }
    },
    {
      refreshDeps: [filters, searchKeyword],
      manual: true,
      defaultPageSize: DEFAULT_PAGE_SIZE,
    },
  );

  const { run: debounceQuery } = useDebounceFn(
    (params: PaginationParams) => service.run(params),
    { wait: 600 },
  );
  const templates = service.data?.list || [];

  const isEmpty = templates.length === 0 && !service.loading && firstLoaded;

  useEffect(() => {
    service.run({
      pageSize: service.pagination?.pageSize || DEFAULT_PAGE_SIZE,
      current: 1,
    });
  }, []);

  useEffect(() => {
    if (!firstLoadRef.current) {
      return;
    }
    debounceQuery({
      pageSize: service.pagination?.pageSize || DEFAULT_PAGE_SIZE,
      current: 1,
    });
  }, [filters, searchKeyword, debounceQuery]);

  return {
    service,
    templates,
    isEmpty,
    total: service.data?.total || 0,
    loading: service.loading,
    firstLoaded,
    filters,
    setFilters,
    searchKeyword,
    setSearchKeyword,
  };
}

export function EvaluatorTemplateList({
  colCount = 3,
  listHeaderAction,
  defaultEvaluatorType,
  disabledEvaluatorTypes,
  filterOptions,
  listTemplates,
  className,
  getCardHeaderActions,
  onClickCard,
}: EvaluatorTemplateListProps) {
  const {
    service,
    templates,
    isEmpty,
    filters,
    setFilters,
    searchKeyword,
    setSearchKeyword,
  } = useListStore({
    defaultEvaluatorType,
    listTemplates,
  });
  const cardWidth = `calc((100% - ${colCount - 1} * 0.75rem) / ${colCount})`;

  const skeletonPlaceholder = (
    <div className="flex flex-wrap gap-3">
      {new Array(12).fill(1).map((_, index) => (
        <Skeleton.Image key={index} style={{ width: cardWidth, height: 194 }} />
      ))}
    </div>
  );

  return (
    <div className={classNames('h-full flex overflow-hidden', className)}>
      <div className="overflow-auto w-[320px] h-full border-0 border-r-[1px] border-solid border-[var(--coz-stroke-primary)]">
        <EvaluatorTemplateFilter
          {...filterOptions}
          disabledEvaluatorTypes={disabledEvaluatorTypes}
          className="h-full"
          filter={filters}
          onFilterChange={setFilters}
        />
      </div>
      <div className="flex-1 flex flex-col gap-4 py-4 h-full overflow-hidden pr-0">
        <div className="flex items-center gap-[10px] px-6 pr-6">
          <EvaluatorTemplateNameSearchInput
            className="flex-1"
            value={searchKeyword}
            onChange={setSearchKeyword}
          />
          {listHeaderAction?.({ filters })}
        </div>
        <Spin
          wrapperClassName="flex-1 overflow-hidden"
          childStyle={{
            padding: '0 24px',
            height: '100%',
            overflow: 'auto',
          }}
          spinning={service.loading}
        >
          <Skeleton
            placeholder={skeletonPlaceholder}
            // 骨架屏仅在加载状态且列表为空时显示
            loading={service.loading && templates.length === 0}
            active={true}
          >
            {!isEmpty ? (
              <div className="flex flex-wrap gap-3">
                {templates?.map(evaluatorTemplate => (
                  <EvaluatorTemplateCard
                    key={evaluatorTemplate.id}
                    evaluatorTemplate={evaluatorTemplate}
                    style={{ width: cardWidth }}
                    getCardHeaderActions={getCardHeaderActions}
                    onClick={onClickCard}
                  />
                ))}
              </div>
            ) : (
              <EmptyTips
                onClearFilter={() => {
                  setFilters(undefined);
                  setSearchKeyword('');
                }}
              />
            )}
          </Skeleton>
        </Spin>
        {!isEmpty && (
          <div className="flex justify-end items-center">
            <span>
              {I18n.t('total_{num}_items', {
                num: service.data?.total || 0,
              })}
            </span>
            <CozPagination
              {...service.pagination}
              showTotal={false}
              showSizeChanger={false}
            />
          </div>
        )}
      </div>
    </div>
  );
}
