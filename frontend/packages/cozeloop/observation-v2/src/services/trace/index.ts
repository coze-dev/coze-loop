/* eslint-disable max-lines-per-function */
/* eslint-disable @typescript-eslint/naming-convention */
/* eslint-disable @typescript-eslint/no-explicit-any */
import dayjs from 'dayjs';
import axios from 'axios';

import { type FetchSpansFn } from '@/types/trace-list';
import { type GetTraceDetailDataFn } from '@/trace-list-with-detail-panel';
import { useReport } from '@/shared/hooks/use-report';
import { SDK_INTERNAL_EVENTS } from '@/shared/constants';
import { type WorkspaceConfig } from '@/features/trace-list/types';
import { useConfigContext } from '@/config-provider';

export const TRACE_DETAIL_URL = '/open-api/observability/traces/search';
export const TRACE_LIST_URL = '/open-api/observability/spans/search';
export const TRACE_ADVANCE_INFO_URL = '/open-api/observability/traces/list';

interface ListSpansParams {
  workspace_id?: string;
  start_time: string;
  end_time: string;
  filters?: Record<string, any>[];
  page_size?: number;
  order_bys?: any[];
  page_token?: string;
  platform_type?: string;
  span_list_type?: string;
}

interface SearchTraceOApiRequest {
  workspace_id?: number;
  logid?: string;
  trace_id?: string;
  start_time: string; // ms
  end_time: string; // ms
  limit?: number;
  platform_type?: string;
}

/** workspaceConfig 不传入是，默认读取 ConfigContext 中的 workspaceConfig */
export const useTraceService = (workspaceConfig?: WorkspaceConfig) => {
  const context = useConfigContext();
  const {
    workspaceId = '',
    domain = '',
    token = '',
  } = workspaceConfig ?? context.workspaceConfig ?? {};
  const report = useReport();
  const axiosInstance = axios.create({
    baseURL: domain,
    headers: {
      Authorization: token,
      'Agw-Js-Conv': 'str',
    },
  });
  axiosInstance.interceptors.response.use(response => response.data);
  return {
    getTraceDetail: (async (params: SearchTraceOApiRequest) => {
      report(SDK_INTERNAL_EVENTS.sdk_trace_detail_fetch);
      return axiosInstance
        .post(`${TRACE_DETAIL_URL}`, {
          ...params,
          workspace_id: workspaceId,
          limit: 1000,
          start_time:
            params.start_time ??
            dayjs().subtract(7, 'days').valueOf().toString(),
          end_time: params.end_time ?? dayjs().valueOf().toString(),
        })
        .then(res => ({
          // @ts-expect-error type
          code: res.code,
          // @ts-expect-error type
          msg: res.msg,
          spans: res.data?.spans ?? [],
          traces_advance_info: res.data?.traces_advance_info ?? {},
        }))
        .catch(err => {
          console.error('getTraceDetail', err);
          return {
            code: err.code,
            msg: err.message,
            spans: [],
            traces_advance_info: {},
          };
        });
    }) as unknown as GetTraceDetailDataFn,
    getTraceList: (async (params: ListSpansParams) => {
      report(SDK_INTERNAL_EVENTS.sdk_trace_list_fetch);
      return axiosInstance
        .post(TRACE_LIST_URL, {
          ...params,
          workspace_id: workspaceId,
        })
        .then(res => ({
          // @ts-expect-error type
          code: res.code,
          // @ts-expect-error type
          msg: res.msg,
          spans: res.data?.spans ?? [],
          has_more: res.data?.has_more ?? false,
          next_page_token: res.data?.next_page_token ?? '',
        }))
        .catch(err => {
          console.error('getTraceList', err);
          return {
            code: err.code,
            msg: err.message,
            spans: [],
            has_more: false,
            next_page_token: '',
          };
        });
    }) as unknown as FetchSpansFn,
    _getTracesAdvanceInfo: (params: {
      traces: { trace_id: string; start_time: string }[];
      platform_type: string;
      workspace_id: string | number;
    }) => {
      const traceIds = params.traces.map(item => item.trace_id);
      const startTime = Math.min(
        ...params.traces.map(item => Number(item.start_time)),
      );
      const endTime = Math.max(
        ...params.traces.map(item => Number(item.start_time)),
      );

      return axiosInstance
        .post(`${TRACE_ADVANCE_INFO_URL}`, {
          trace_ids: traceIds,
          platform_type: params.platform_type,
          workspace_id: params.workspace_id || workspaceId,
          start_time: startTime.toString(),
          end_time: endTime.toString(),
        })
        .then(res => ({
          traces_advance_info:
            res.data?.traces?.map(
              (item: {
                trace_id: string;
                tokens?: { input_token: number; output_token: number };
              }) => ({
                trace_id: item.trace_id,
                tokens: {
                  input: item.tokens?.input_token ?? 0,
                  output: item.tokens?.output_token ?? 0,
                },
              }),
            ) ?? [],
        }))
        .catch(err => {
          console.error('_getTracesAdvanceInfo', err);
          return {
            code: err.code,
            msg: err.message,
            traces_advance_info: [],
          };
        });
    },
  };
};
