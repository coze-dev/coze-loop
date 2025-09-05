// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/naming-convention */
import { I18n } from '@cozeloop/i18n-adapter';
import { PrimaryPage } from '@cozeloop/components';

import { useTraceTimeRangeOptions } from '@/hooks/use-trace-time-range-options';
import { TraceProvider } from '@/contexts/trace-context';
import { SPAN_TAB_OPTION_LIST } from '@/consts/filter';

import { type InitValue } from './utils/url';
import { type ConvertSpan } from './typings/span';
import { type SizedColumn } from './typings/index';
import { usePageStay } from './hooks/use-page-stay';
import { useFetchMetaInfo } from './hooks/use-fetch-meta-info';
import { useColumns } from './hooks/use-column';
import { PLATFORM_ENUM_OPTION_LIST } from './consts/filter';
import { COLUMN_RECORD, SPAN_COLUMNS } from './consts';
import { useFetchTraces } from './components/queries/table/hooks/use-fetch-traces';
import { Queries } from './components/queries';
import { MANUAL_FEEDBACK } from './components/logic-expr/const';
import {
  LeftManualExpr,
  type LeftManualExprProps,
} from './components/left-manual-expr';
import { PromptSelect } from './components/filter-bar/prompt-select';
import { QueryFilterBar } from './components/filter-bar';
import { CozeLoopTraceBanner } from './components/banner';

const getTooltipContent = () => ({
  all_span: I18n.t('all_span_tip'),
  root_span: I18n.t('root_span_tip'),
  llm_span: I18n.t('llm_span_tip'),
});

const TraceListApp = () => {
  usePageStay();

  const datePickerOptions = useTraceTimeRangeOptions();
  const { selectedColumns, cols, onColumnsChange, defaultColumns } = useColumns(
    {
      columnsList: SPAN_COLUMNS,
      columnConfig: COLUMN_RECORD as unknown as Record<
        string,
        SizedColumn<ConvertSpan>
      >,
      storageOptions: {
        enabled: true,
        key: 'trace-selected-columns-open',
      },
    },
  );
  useFetchMetaInfo();

  const { spans, noMore, loading, loadMore, loadingMore, traceListCode } =
    useFetchTraces();

  return (
    <div className="h-full max-h-full w-full flex-1 max-w-full overflow-hidden !min-w-[980px] flex flex-col">
      <CozeLoopTraceBanner />
      <PrimaryPage
        pageTitle="Trace"
        filterSlot={
          <QueryFilterBar
            datePickerOptions={datePickerOptions}
            columns={cols}
            defaultColumns={defaultColumns}
            onColumnsChange={onColumnsChange}
            platformEnumOptionList={PLATFORM_ENUM_OPTION_LIST}
            spanListTypeEnumOptionList={SPAN_TAB_OPTION_LIST}
            tooltipContent={getTooltipContent()}
            customLeftRenderMap={{
              [MANUAL_FEEDBACK]: v => (
                <LeftManualExpr {...(v as unknown as LeftManualExprProps)} />
              ),
            }}
            customRightRenderMap={{
              prompt_key: v => <PromptSelect {...v} />,
            }}
          />
        }
        className="!pb-0"
      >
        <Queries
          moduleName="analytics_trace_list"
          selectedColumns={selectedColumns}
          columns={cols}
          spans={spans}
          noMore={noMore}
          loading={loading}
          loadMore={loadMore}
          loadingMore={loadingMore}
          traceListCode={traceListCode}
        />
      </PrimaryPage>
    </div>
  );
};

const initPlatformConfig: InitValue = {
  value: ['cozeloop', 'prompt'],
  defaultValue: 'cozeloop',
  format: 'string',
};

const initSpanListTypeConfig: InitValue = {
  value: ['root_span', 'all_span', 'llm_span'],
  defaultValue: 'root_span',
  format: 'string',
};

export default () => (
  <TraceProvider
    spanListType={initSpanListTypeConfig}
    platformType={initPlatformConfig}
  >
    <TraceListApp />
  </TraceProvider>
);
