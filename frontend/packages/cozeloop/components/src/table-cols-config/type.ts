// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ColumnProps } from '@coze-arch/coze-design';

export type ColKey = string | number;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export interface ColumnPropsPro<T extends Record<string, any> = any>
  extends ColumnProps<T> {
  /* 是否可配置,默认为true */
  configurable?: boolean;
  /* 内部使用可忽略 */
  colConfigKey?: ColKey;
}
