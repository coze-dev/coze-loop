// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type OptionProps, type SelectProps } from '@coze-arch/coze-design';

export interface BaseSelectProps extends SelectProps {
  loadOptionByIds?: (
    ids?: string | number | any[] | Record<string, any> | undefined,
  ) => Promise<(OptionProps & { [key: string]: any })[]>;
  /** 是否显示刷新按钮 */
  showRefreshBtn?: boolean;
  /** 点击刷新按钮的回调 */
  onClickRefresh?: () => void;
}
