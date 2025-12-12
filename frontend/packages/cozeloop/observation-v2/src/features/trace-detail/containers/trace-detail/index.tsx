/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect } from 'react';

import { useUnmount } from 'ahooks';
import { SpanType } from '@cozeloop/api-schema/observation';

import { useReport } from '@/shared/hooks/use-report';
import { SDK_INTERNAL_EVENTS, BIZ_EVENTS } from '@/shared/constants';
import { useTraceService } from '@/services';
import { getNodeConfig } from '@/features/trace-detail/utils/span';
import { useTraceDetailControls } from '@/features/trace-detail/hooks/use-trace-detail-controls';
import {
  type TraceDetailContext,
  traceDetailContext,
} from '@/features/trace-detail/hooks/use-trace-detail-context';
import { useTeaDuration } from '@/features/trace-detail/hooks/use-tea-duration';
import { useFetchSpans } from '@/features/trace-detail/hooks/use-fetch-spans';
import { useFetchResponseApi } from '@/features/trace-detail/hooks/use-fetch-response-api';
import { INVALIDATE_CODE } from '@/features/trace-detail/constants/code';
import { useConfigContext } from '@/config-provider';

import { useResponseApiCacheStore } from '../../store/response-api-cache';
import { VerticalTraceDetail } from './vertical';
import { TraceDetailError } from './trace-error';
import { type CozeloopTraceDetailProps } from './interface';
import { HorizontalTraceDetail } from './horizontal';

export const CozeloopTraceDetail = (
  props: CozeloopTraceDetailProps & TraceDetailContext,
) => {
  const {
    defaultSelectedSpanID,
    layout,
    switchConfig,
    spanId,
    customParams,
    getTraceSpanDetailData,
    keySpanType,
    treeSelectorConfig,
    enableTraceSearch,
    enableResponseApi = false,
    traceId,
  } = props;
  const { getTraceDetail } = useTraceService();
  const responseApiCache = useResponseApiCacheStore(state => state.cache);
  const report = useReport();
  const getTraceDetailData = (props.getTraceDetailData ??
    getTraceDetail) as unknown as any;
  const { roots, spans, advanceInfo, loading, statusCode } = useFetchSpans({
    getTraceDetailData,
    spanId,
  });
  const { sendEvent } = useConfigContext();
  const responseApiService = useFetchResponseApi();

  useUnmount(() => {
    responseApiCache.clear();
  });

  // 使用新的统一状态管理 hook
  const traceDetailControls = useTraceDetailControls({
    roots,
    spans,
    getTraceSpanDetailData,
    getTraceDetailData,
    defaultSpanID: defaultSelectedSpanID,
    keySpanType,
    enableTraceSearch,
  });

  useEffect(() => {
    report(SDK_INTERNAL_EVENTS.sdk_trace_detail_view);
  }, []);

  useTeaDuration(
    BIZ_EVENTS.cozeloop_observation_trace_detail_panel_view_duration,
    {
      space_id: customParams?.spaceID ?? '',
      space_name: customParams?.spaceName ?? '',
      search_type: 'trace_id',
      platform_type: customParams?.platformType ?? '',
      module_name: customParams?.moduleName ?? '',
    },
  );

  useEffect(() => {
    sendEvent?.(BIZ_EVENTS.cozeloop_trace_detail_show, {
      space_id: customParams?.spaceID ?? '',
      space_name: customParams?.spaceName ?? '',
      search_type: 'trace_id',
      platform_type: customParams?.platformType ?? '',
      module_name: customParams?.moduleName ?? '',
    });
  }, []);

  const handleSelected = (selectedId: string) => {
    traceDetailControls.onSelect(selectedId);
    const { type, span_type } =
      spans.find(span => span.span_id === selectedId) || {};
    sendEvent?.(BIZ_EVENTS.cozeloop_click_trace_tree_node, {
      space_id: customParams?.spaceID ?? '',
      space_name: customParams?.spaceName ?? '',
      type:
        span_type ||
        getNodeConfig({
          spanTypeEnum: type ?? SpanType.Unknown,
          spanType: span_type ?? SpanType.Unknown,
        })?.typeName,
      module_name: '',
    });
  };

  useEffect(() => {
    const currentSelectSpan = spans.find(
      span => span.span_id === traceDetailControls.selectedSpanId,
    );
    const previousResponseId =
      currentSelectSpan?.system_tags?.previous_response_id;
    const cacheItem = responseApiCache.get(previousResponseId ?? '');
    if (
      !previousResponseId ||
      cacheItem?.isFetched ||
      !traceId ||
      !spans.length ||
      !enableResponseApi
    ) {
      return;
    }
    responseApiService
      .runAsync({
        trace_id: traceId,
        span_id: traceDetailControls.selectedSpanId,
        platform_type: customParams?.platformType ?? '',
        previous_response_id: previousResponseId,
        start_time: currentSelectSpan.started_at,
        workspace_id: customParams?.spaceID ?? '',
      })
      .then(res => {
        responseApiCache.set(previousResponseId, {
          data: res.spans,
          isFetched: true,
        });
      })
      .catch(() => {
        responseApiCache.set(previousResponseId, {
          isFetched: true,
          data: null,
        });
      });
  }, [
    traceDetailControls.selectedSpanId,
    traceId,
    spans,
    customParams?.platformType,
    enableResponseApi,
    responseApiCache,
  ]);

  if (INVALIDATE_CODE.includes(statusCode)) {
    return (
      <TraceDetailError
        statusCode={statusCode}
        headerConfig={props.headerConfig}
      />
    );
  }

  const commonProps = {
    ...props,
    advanceInfo,
    loading,
    onCollapseChange: traceDetailControls.changeSpanNodeCollapseStatus,
    onSelect: handleSelected,
    onToggleAll: traceDetailControls.onToggleAll,
    isAllExpanded: traceDetailControls.isAllExpanded,
    rootNodes: traceDetailControls.rootNodes,
    spans,
    searchService: traceDetailControls.searchService,
    selectedSpanService: traceDetailControls.selectedSpanService,
    responseApiService,
    selectedSpanId: traceDetailControls.selectedSpanId,
    searchFilters: traceDetailControls.searchFilters,
    // 新增搜索相关属性
    matchedSpanIds: traceDetailControls.matchedSpanIds,
    setSearchFilters: traceDetailControls.setSearchFilters,
    onClear: traceDetailControls.onClear,
    // 新增过滤非关键节点相关属性
    filterNonCritical: traceDetailControls.filterNonCritical,
    onFilterNonCriticalChange: traceDetailControls.onFilterNonCriticalChange,
    treeSelectorConfig,
    enableTraceSearch,
  };

  return (
    <traceDetailContext.Provider
      value={{
        getTraceDetailData: props.getTraceDetailData,
        customParams,
        spanDetailHeaderSlot: props.spanDetailHeaderSlot,
        extraSpanDetailTabs: props.extraSpanDetailTabs,
        platformType: props.platformType,
        jumpButtonConfig: props.jumpButtonConfig,
      }}
    >
      <div
        className="w-full h-full max-h-full overflow-hidden flex min-h-full"
        id="cozeloop-trace-detail"
      >
        {layout === 'vertical' ? (
          <VerticalTraceDetail {...commonProps} />
        ) : (
          <HorizontalTraceDetail {...commonProps} switchConfig={switchConfig} />
        )}
      </div>
    </traceDetailContext.Provider>
  );
};
