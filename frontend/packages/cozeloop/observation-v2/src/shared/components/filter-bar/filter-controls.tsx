import type { OptionProps } from '@coze-arch/coze-design';

import type { LogicValue } from '@/shared/components/analytics-logic-expr/logic-expr';
import {
  MAX_DAY_COUNT,
  type PresetRange,
} from '@/features/trace-list/constants/time';

import {
  PreselectedDatePicker,
  type PreselectedDatePickerRef,
} from '../date-picker';
import type {
  FilterBarItem,
  DatePickerOptions,
  QueryFilterProps,
} from './types';
import { SpanTypeSelect } from './span-list-type-select';
import { PlatformSelect } from './platform-type-select';
import type { View } from './custom-view';

interface FilterControlsProps {
  isItemEnabled: (item: FilterBarItem) => boolean;
  datePickerRef?: React.Ref<PreselectedDatePickerRef>;
  presetTimeRange: PresetRange;
  timestamp: [number, number];
  datePickerOptions: DatePickerOptions[];
  onTimestampsChange: (timestamps: [number, number]) => void;
  onPresetTimeRangeChange: (presetTimeRange: PresetRange) => void;
  datePickerProps?: QueryFilterProps['datePickerProps'];
  spanListTypeConfig?: QueryFilterProps['spanListTypeConfig'];
  selectedSpanType: string | number;
  filters?: LogicValue;
  selectedPlatform: string | number;
  spanListTypeEnumOptionList: OptionProps[];
  onSelectedSpanTypeChange: (selectedSpanType: string | number) => void;
  setActiveViewKey: (key: string | null) => void;
  platformTypeConfig?: QueryFilterProps['platformTypeConfig'];
  platformEnumOptionList: OptionProps[];
  onSelectedPlatformChange: (selectedPlatform: string | number) => void;
  filterControlsSlot?: React.ReactNode;
  viewList: View[];
  activeViewKey: string | number | null;
  onApplyFilters: () => void;
  onSaveToCustomView: (viewId: string) => void;
  onSaveToCurrentView: (viewId: string) => void;
}

export const FilterControls: React.FC<FilterControlsProps> = ({
  isItemEnabled,
  datePickerRef,
  presetTimeRange,
  timestamp,
  datePickerOptions,
  onTimestampsChange,
  onPresetTimeRangeChange,
  datePickerProps,
  spanListTypeConfig,
  selectedSpanType,
  selectedPlatform,
  spanListTypeEnumOptionList,
  onSelectedSpanTypeChange,
  setActiveViewKey,
  platformTypeConfig,
  platformEnumOptionList,
  onSelectedPlatformChange,
  filterControlsSlot,
  viewList,
  activeViewKey,
  onApplyFilters,
}) => {
  const [startTime, endTime] = timestamp;

  const handleTimeStampChange = ({
    start,
    end,
  }: {
    start: number;
    end: number;
  }) => {
    onTimestampsChange([start, end]);
  };

  return (
    <div className="flex  gap-x-2 gap-y-2 items-center flex-nowrap">
      {isItemEnabled('datePicker') && (
        <div className="box-border">
          <PreselectedDatePicker
            ref={datePickerRef}
            preset={presetTimeRange}
            timeStamp={{
              startTime,
              endTime,
            }}
            datePickerOptions={datePickerOptions}
            maxPastDateRange={MAX_DAY_COUNT}
            onPresetChange={(preset, timeStamp) => {
              if (timeStamp) {
                handleTimeStampChange({
                  start: timeStamp.startTime,
                  end: timeStamp.endTime,
                });
              }
              onPresetTimeRangeChange(preset);
            }}
            oneTimeStampChange={timeStamp => {
              handleTimeStampChange({
                start: timeStamp.startTime,
                end: timeStamp.endTime,
              });
            }}
            datePickerProps={datePickerProps}
          />
        </div>
      )}

      <div className="box-border flex gap-x-2">
        {isItemEnabled('spanType') &&
        (spanListTypeConfig?.visibility ?? true) ? (
          <SpanTypeSelect
            value={selectedSpanType}
            optionList={spanListTypeEnumOptionList}
            onChange={e => {
              onSelectedSpanTypeChange(e);
              setActiveViewKey(null);
            }}
          />
        ) : null}
        {isItemEnabled('platform') &&
        (platformTypeConfig?.visibility ?? false) ? (
          <PlatformSelect
            value={selectedPlatform}
            optionList={platformEnumOptionList}
            onChange={e => {
              onSelectedPlatformChange(e);
              setActiveViewKey(null);
            }}
          />
        ) : null}
      </div>

      {filterControlsSlot}

      {/* {isItemEnabled('filter') && (
        <FilterSelect
          viewList={viewList}
          activeViewKey={activeViewKey}
          onApplyFilters={onApplyFilters}
        />
      )} */}
    </div>
  );
};
