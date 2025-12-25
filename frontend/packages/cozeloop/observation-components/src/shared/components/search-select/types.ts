// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type OptionProps, type SelectProps } from '@coze-arch/coze-design';

export interface BaseSelectProps extends SelectProps {
  loadOptionByIds?: (
    ids?: string | number | unknown[] | Record<string, unknown> | undefined,
  ) => Promise<(OptionProps & Record<string, unknown>)[]>;
  /** 是否显示刷新按钮 */
  showRefreshBtn?: boolean;
  /** 点击刷新按钮的回调 */
  onClickRefresh?: () => void;
}
