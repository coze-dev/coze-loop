// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import type { ReactElement } from 'react';

import { type useRequest } from 'ahooks';
import {
  type TraceSelectorProps,
  type CozeloopTraceDetailPanelProps,
  type TraceDetailContext,
  type CozeloopTraceDetailProps,
} from '@cozeloop/observation-components';
import type { PlatformType, span } from '@cozeloop/api-schema/observation';
import {
  type FieldSchema,
  type ArgsSchema,
} from '@cozeloop/api-schema/evaluation';

type CozeloopTraceSelectorProps = Pick<
  TraceSelectorProps,
  | 'items'
  | 'onChange'
  | 'spanListTypeOptionList'
  | 'platformTypeOptionList'
  | 'datePickerOptions'
  | 'fieldMetas'
  | 'customParams'
  | 'layoutMode'
  | 'initValues'
  | 'getFieldMetas'
  | 'disabled'
  | 'ignoreKeys'
  | 'disabledRowKeys'
  | 'disableFilterItemByFilterName'
>;

export interface TraceDetailExtraProps {
  traceID: string;
  startTime?: string;
  endTime?: string;
  env?: string;
}
interface FieldMapping {
  key: string;
  json_path: string;
}

type UseRequestReturn<TData, TParams extends unknown[]> = ReturnType<
  typeof useRequest<TData, TParams>
>;

export interface ObservationTraceAdapters {
  getAutoEvaluateFieldMappingConfig: (params: {
    evaluatorInputSchemas: ArgsSchema[];
    sampleSpan: span.OutputSpan;
    platformType: PlatformType;
    spaceID: string;
  }) => Promise<{
    success: boolean;
    fieldMapping: Record<string, string>;
  }>;

  getDataReflowFieldMappingConfig: (params: {
    evaluationSetSchema: FieldSchema[];
    sampleSpan: span.OutputSpan;
    platformType: PlatformType;
    spaceID: string;
  }) => Promise<{
    success: boolean;
    fieldMapping: Record<string, string>;
    fieldMappings?: FieldMapping[];
  }>;

  DatasetSelect: (props: {
    pageSize?: number;
    publishOption?: number;
    showSecurityLevel?: boolean;
    hiddenL4Dataset?: boolean;
    [key: string]: unknown;
  }) => ReactElement | null;

  useFetchSpanDetail: (params: {
    spans: span.OutputSpan[];
    getStartTime: (startTime: number | string) => string;
    getEndTime: (endTime: number | string, latency: number | string) => string;
    platformType?: string | number;
  }) => UseRequestReturn<span.OutputSpan[], []>;

  CozeloopTraceSelector?: (
    props: Partial<CozeloopTraceSelectorProps>,
  ) => ReactElement | null;
  CozeloopTraceSelectorInitValues: Record<string, any>;
  TraceDetailPanel: (
    props: CozeloopTraceDetailPanelProps &
      TraceDetailContext &
      TraceDetailExtraProps,
  ) => JSX.Element;
  TraceDetail: (
    props: CozeloopTraceDetailProps &
      TraceDetailContext &
      TraceDetailExtraProps,
  ) => JSX.Element;
}
