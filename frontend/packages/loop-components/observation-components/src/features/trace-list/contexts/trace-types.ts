// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type FieldMeta } from '@cozeloop/api-schema/observation';

export interface LogicExprConfig {
  customRightRenderMap?: Record<string, (props: any) => React.ReactNode>;
  customLeftRenderMap?: Record<string, (props: any) => React.ReactNode>;
}

export interface CustomViewConfig {
  visibility: boolean;
}

export interface TraceContextState {
  fieldMetas?: Record<string, FieldMeta | undefined>;
  getFieldMetas?: (params: {
    platform_type: string | number;
    span_list_type: string | number;
  }) => Promise<Record<string, FieldMeta>>;
  customViewConfig?: CustomViewConfig;
  customParams?: Record<string, any>;
  disableEffect?: boolean;
}

export interface TraceContextActions {
  setFieldMetas: (e?: Record<string, FieldMeta>) => void;
}

export type TraceContextType = TraceContextState & TraceContextActions;
export interface OptionsItem {
  value: string;
  label: string;
}
export interface TraceProviderProps {
  children: React.ReactNode;
  getFieldMetas?: (params: {
    platform_type: string | number;
    span_list_type: string | number;
  }) => Promise<Record<string, FieldMeta>>;
  customViewConfig?: CustomViewConfig;
  customParams?: Record<string, any>;
  disableEffect?: boolean;
}
