// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type CozeloopTraceListProps } from '@/types/trace-list';
import {
  getBizConfig,
  getDefaultBizConfig,
} from '@/features/trace-list/biz-config';
import { useConfigContext } from '@/config-provider';

interface Params {
  customParams?: Record<string, any>;
  filterOptions?: CozeloopTraceListProps['filterOptions'];
}

export const useInitTraceListConfig = (params: Params) => {
  const { customParams, filterOptions } = params;
  const { bizId } = useConfigContext();
  const bizConfig = getBizConfig(customParams ?? {});
  const config =
    bizConfig[bizId as keyof typeof bizConfig] ?? getDefaultBizConfig();

  const { platformTypeConfig, spanListTypeConfig } = filterOptions ?? {};

  const configSpanListConfig = config.initSpanListTypeConfig;
  const configPlatformTypeConfig = config.initPlatformConfig;

  const { logicExprConfig } = config;

  const initPlatformTypeConfig = {
    ...platformTypeConfig,
    optionList: platformTypeConfig?.optionList ?? config.platformTypeOptions,
    defaultValue:
      platformTypeConfig?.defaultValue ?? configPlatformTypeConfig.defaultValue,
    visibility: platformTypeConfig?.visibility ?? true,
  };

  const initSpanListTypeConfig = {
    ...spanListTypeConfig,
    optionList: spanListTypeConfig?.optionList ?? config.spanListTypeOptions,
    defaultValue:
      spanListTypeConfig?.defaultValue ?? configSpanListConfig.defaultValue,
    visibility: spanListTypeConfig?.visibility ?? true,
  };

  const initLogicExprConfig = {
    customRightRenderMap: {
      ...(logicExprConfig?.customRightRenderMap ?? {}),
      ...(customParams?.customRightRenderMap ?? {}),
    },
    customLeftRenderMap: {
      ...(logicExprConfig?.customLeftRenderMap ?? {}),
      ...(customParams?.customLeftRenderMap ?? {}),
    },
  };

  return {
    initPlatformTypeConfig,
    initSpanListTypeConfig,
    initLogicExprConfig,
    initBizConfig: config,
  };
};
