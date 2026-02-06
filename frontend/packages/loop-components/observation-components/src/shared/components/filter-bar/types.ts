// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import type { ColumnItem } from '@cozeloop/components';
import type { DatePickerProps, OptionProps } from '@coze-arch/coze-design';

import type { LogicValue } from '@/shared/components/analytics-logic-expr/logic-expr';
import type { ConvertSpan } from '@/features/trace-list/types/span';
import type { SizedColumn } from '@/features/trace-list/types/index';
import { type CustomViewConfig } from '@/features/trace-list/contexts/trace-types';
import { type PresetRange } from '@/features/trace-list/constants/time';

export type FilterBarItem =
  | 'datePicker'
  | 'spanType'
  | 'platform'
  | 'filter'
  | 'customView'
  | 'refresh'
  | 'columnSelector';

export interface DatePickerOptions {
  label: JSX.Element;
  value: PresetRange;
  disabled?: boolean;
}

export interface QueryFilterProps {
  bottomSlot?: React.ReactNode;
  filterControlsSlot?: React.ReactNode;
  headerActionsSlot?: React.ReactNode;
  datePickerProps?: DatePickerProps;
  datePickerOptions: DatePickerOptions[];
  columns: SizedColumn<ConvertSpan>[];
  defaultColumns: SizedColumn<ConvertSpan>[];
  onColumnsChange: (newColumns: ColumnItem[]) => void;
  platformEnumOptionList: OptionProps[];
  spanListTypeEnumOptionList: OptionProps[];
  tooltipContent?: Record<string, string>;
  platformTypeConfig?: {
    defaultValue?: string;
    optionList?: OptionProps[];
    visibility?: boolean;
  };
  spanListTypeConfig?: {
    defaultValue: string;
    optionList: OptionProps[];
    visibility?: boolean;
  };
  timestamp: [number, number];
  presetTimeRange: PresetRange;
  onTimestampsChange: (timestamps: [number, number]) => void;
  onPresetTimeRangeChange: (presetTimeRange: PresetRange) => void;
  selectedSpanType: string | number;
  selectedPlatform: string | number;
  onSelectedPlatformChange: (selectedPlatform: string | number) => void;
  onSelectedSpanTypeChange: (selectedSpanType: string | number) => void;
  onRefresh: () => void;
  customViewConfig?: CustomViewConfig;
  filters?: LogicValue;
  /** 控制筛选器显示的配置数组，指定要显示的筛选器项 */
  items?: FilterBarItem[];
}
