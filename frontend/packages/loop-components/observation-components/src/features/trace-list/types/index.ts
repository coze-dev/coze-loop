// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type ColumnProps } from '@coze-arch/coze-design';

import { type QueryPropertyEnum } from '@/features/trace-list/constants';

export interface SizedColumn<RecordType extends Record<string, any> = any>
  extends ColumnProps<RecordType> {
  width?: number;
  checked?: boolean;
  key?: string;
  disabled?: boolean;
  value?: string;
  displayName?: string;
}

export interface PersistentFilter {
  type: QueryPropertyEnum;
  value: string | string[];
}
