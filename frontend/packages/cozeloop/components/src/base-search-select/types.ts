// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type OptionProps, type SelectProps } from '@coze-arch/coze-design';

export type LoadOptionByIds = (
  ids: string[] | number[],
) => Promise<(OptionProps & { [key: string]: any })[]>;

export interface BaseSelectProps extends SelectProps {
  /** 是否禁用缓存选项 */
  disabledCacheOptions?: boolean;
  loadOptionByIds?: LoadOptionByIds;
  /** 是否显示刷新按钮 */
  showRefreshBtn?: boolean;
  /** 点击刷新按钮的回调 */
  onClickRefresh?: () => void;
}
