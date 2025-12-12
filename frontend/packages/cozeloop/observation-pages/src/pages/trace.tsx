// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/naming-convention */
import { useMemo } from 'react';

import zhCH from '@cozeloop/observation-components/zh-CN';
import enUS from '@cozeloop/observation-components/en-US';
import {
  ConfigProvider,
  fetchMetaInfo,
  CozeloopTraceListWithDetailPanel,
  PromptSelect,
  type PromptSelectProps,
  CozeLoopTraceBanner,
} from '@cozeloop/observation-components';
import { useTraceTimeRangeOptions } from '@cozeloop/observation-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { PrimaryPage } from '@cozeloop/components';
import { useSpace, useUserInfo } from '@cozeloop/biz-hooks-adapter';
import {
  type PlatformType,
  type FieldMeta,
  type span,
} from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';

const TracesPage = () => {
  const { spaceID, space: { name: spaceName } = {} } = useSpace();
  const user = useUserInfo();
  const lang = I18n.language === 'zh-CN' ? zhCH : enUS;

  const datePickerOptions = useTraceTimeRangeOptions();
  const columnsConfig = useMemo(
    () => ({
      columns: [
        // 基础列
        'status',
        'trace_id',
        'input',
        'output',
        'tokens',
        'latency',
        'latency_first_resp',
        'start_time',
        'input_tokens',
        'output_tokens',
        'span_id',
        'span_type',
        'span_name',
        'prompt_key',
        'logic_delete_date',
      ],
    }),
    [],
  );

  return (
    <div className="h-full max-h-full w-full flex-1 max-w-full overflow-hidden !min-w-[980px] flex flex-col">
      <CozeLoopTraceBanner />
      <PrimaryPage pageTitle="Trace" className="!pb-0">
        <ConfigProvider
          bizId="cozeloop"
          locale={{
            language: I18n.lang,
            locale: lang,
          }}
        >
          <CozeloopTraceListWithDetailPanel
            columnsConfig={columnsConfig}
            filterOptions={{
              platformTypeConfig: {
                visibility: true,
              },
              datePickerOptions,
            }}
            getFieldMetas={async ({ platform_type, span_list_type }) => {
              const result = await fetchMetaInfo({
                selectedPlatform: platform_type,
                selectedSpanType: span_list_type,
                spaceID,
              });

              return (result ?? {}) as unknown as Record<string, FieldMeta>;
            }}
            getTraceList={params => {
              const newParams = {
                ...params,
                filters: {
                  query_and_or: params.filters?.query_and_or ?? 'and',
                  filter_fields: [...(params.filters?.filter_fields ?? [])],
                },
              };
              return observabilityTrace.ListSpans.bind(observabilityTrace)(
                newParams,
              );
            }}
            getTraceDetailData={async ({
              trace_id,
              platform_type,
              start_time,
              end_time,
            }) => {
              const result = await observabilityTrace.GetTrace({
                trace_id: trace_id as string,
                platform_type: platform_type as unknown as PlatformType,
                start_time,
                end_time,
                workspace_id: spaceID,
              });
              return {
                spans: result.spans as unknown as span.OutputSpan[],
                traces_advance_info: result.traces_advance_info,
              };
            }}
            customParams={{
              spaceID,
              spaceName: spaceName ?? '',
              user,
              custom_view: {
                readonly: false,
              },
              customRightRenderMap: {
                prompt_key: (v: unknown) => (
                  <PromptSelect
                    {...(v as PromptSelectProps)}
                    customParams={{
                      spaceID,
                    }}
                  />
                ),
              },
            }}
          />
        </ConfigProvider>
      </PrimaryPage>
    </div>
  );
};

export { TracesPage };
