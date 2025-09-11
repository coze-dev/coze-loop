// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { forwardRef } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { ColumnSelector, type ColumnItem } from '@cozeloop/components';
import { PlatformType, SpanListType } from '@cozeloop/api-schema/observation';
import { tag } from '@cozeloop/api-schema/data';
import { IconCozRefresh } from '@coze-arch/coze-design/icons';
import {
  Button,
  InputGroup,
  type DatePickerProps,
} from '@coze-arch/coze-design';

import { type ConvertSpan } from '@/typings/span';
import { type SizedColumn } from '@/typings/index';
import { useTraceStore } from '@/stores/trace';
import { useCustomView } from '@/hooks/use-custom-view';
import { calcPresetTime, PresetRange } from '@/consts/time';
import {
  PreselectedDatePicker,
  type PreselectedDatePickerRef,
} from '@/components/date-picker';

import { type CustomRightRenderMap } from '../logic-expr/logic-expr';
import { MANUAL_FEEDBACK } from '../logic-expr/const';
import { LeftManualExpr, type LeftManualExprProps } from '../left-manual-expr';
import { SpanTypeSelect } from './trace-list';
import { FilterSelect } from './filter-select';
import { PlatformSelect } from './data-source';
import { CustomView, type View } from './custom-view';
import {
  CategoricalSelect,
  type CategoricalSelectProps,
} from './categorical-select';

interface QueryFilterProps {
  slot?: React.ReactNode;
  headerSlot?: React.ReactNode;
  datePickerProps?: DatePickerProps;
  datePickerOptions: DatePickerOptions[];
  columns: SizedColumn<ConvertSpan>[];
  defaultColumns: SizedColumn<ConvertSpan>[];
  onColumnsChange: (newColumns: ColumnItem[]) => void;
  platformEnumOptionList: { label: string; value: string | number }[];
  spanListTypeEnumOptionList: { label: string; value: string | number }[];
  customRightRenderMap?: CustomRightRenderMap;
  customLeftRenderMap?: CustomRightRenderMap;
  tooltipContent?: Record<string, string>;
}

interface DatePickerOptions {
  label: JSX.Element;
  value: PresetRange;
  disabled?: boolean;
}

export const QueryFilterBar = forwardRef<
  PreselectedDatePickerRef,
  QueryFilterProps
>((props, datePickerRef) => {
  const {
    datePickerProps,
    slot,
    headerSlot,
    datePickerOptions,
    columns,
    onColumnsChange,
    platformEnumOptionList,
    defaultColumns,
    spanListTypeEnumOptionList,
    tooltipContent,
  } = props;
  const {
    setActiveViewKey,
    updateViewList,
    viewList,
    visibleViewIds,
    viewNames,
    lastVisibleIds,
    setVisibleViewIds,
    activeViewKey,
    setAutoSelectedViewId,
  } = useCustomView();
  const {
    timestamps: [startTime, endTime],
    presetTimeRange,
    setTimestamps,
    setPresetTimeRange,
    selectedSpanType,
    selectedPlatform,
    setSelectedPlatform,
    setSelectedSpanType,
    updateRefreshFlag,
    applyFilters,
  } = useTraceStore();

  const handleTimeStampChange = ({
    start,
    end,
  }: {
    start: number;
    end: number;
  }) => {
    setTimestamps([start, end]);
  };

  const handleRefresh = () => {
    if (presetTimeRange === PresetRange.Unset) {
      return;
    }
    const time = calcPresetTime(presetTimeRange);
    if (time) {
      handleTimeStampChange({
        start: time.startTime,
        end: time.endTime,
      });
    }
    updateRefreshFlag();
  };

  const customRightRenderMap: CustomRightRenderMap = {
    [tag.TagContentType.Categorical]: v => (
      <CategoricalSelect {...(v as CategoricalSelectProps)} />
    ),
    [tag.TagContentType.Boolean]: v => (
      <CategoricalSelect {...(v as CategoricalSelectProps)} />
    ),
    ...(props.customRightRenderMap ?? {}),
  };
  const customLeftRenderMap: CustomRightRenderMap = {
    [MANUAL_FEEDBACK]: v => <LeftManualExpr {...(v as LeftManualExprProps)} />,
    ...(props.customLeftRenderMap ?? {}),
  };

  return (
    <div className="flex flex-col gap-2 box-border max-w-full">
      <div className="flex gap-2 items-start justify-between flex-wrap">
        <div className="flex  gap-x-2 gap-y-2 items-center flex-nowrap">
          <FilterSelect
            customRightRenderMap={customRightRenderMap}
            customLeftRenderMap={customLeftRenderMap}
            platformEnumOptionList={platformEnumOptionList}
            spanListTypeEnumOptionList={spanListTypeEnumOptionList}
            viewList={viewList as unknown as View[]}
            activeViewKey={activeViewKey}
            onApplyFilters={() => {
              setActiveViewKey(null);
            }}
            onSaveToCustomView={viewId => {
              setAutoSelectedViewId(viewId);
              updateViewList();
            }}
            onSaveToCurrentView={viewId => {
              setActiveViewKey(viewId);
              updateViewList();
            }}
          />
          {/** 时间选择器 */}
          <div className="box-border">
            <PreselectedDatePicker
              ref={datePickerRef}
              preset={presetTimeRange}
              timeStamp={{
                startTime,
                endTime,
              }}
              datePickerOptions={datePickerOptions}
              onPresetChange={(preset, timeStamp) => {
                if (timeStamp) {
                  handleTimeStampChange({
                    start: timeStamp.startTime,
                    end: timeStamp.endTime,
                  });
                }
                setPresetTimeRange(preset);
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
          <div className="box-border">
            <InputGroup className="box-border">
              <SpanTypeSelect
                value={selectedSpanType}
                applyFilters={applyFilters}
                selectedPlatform={selectedPlatform}
                spanListTypeEnumOptionList={spanListTypeEnumOptionList}
                onChange={e => {
                  setSelectedSpanType(e);
                  setActiveViewKey(null);
                }}
                tooltipContent={tooltipContent}
              />
              <PlatformSelect
                value={selectedPlatform}
                optionList={platformEnumOptionList}
                onChange={e => {
                  setSelectedPlatform(e);
                  setActiveViewKey(null);
                }}
              />
            </InputGroup>
          </div>
        </div>
        <div className="flex items-center gap-x-2 flex-nowrap justify-between flex-1">
          <div className="flex items-center gap-x-2">
            <CustomView
              customRightRenderMap={customRightRenderMap}
              customLeftRenderMap={customLeftRenderMap}
              platformEnumOptionList={platformEnumOptionList}
              spanListTypeEnumOptionList={spanListTypeEnumOptionList}
              activeViewKey={activeViewKey}
              onSelectView={view => {
                setSelectedPlatform(
                  view?.platform_type ?? PlatformType.Cozeloop,
                );
                setSelectedSpanType(
                  view?.spanList_type ?? SpanListType.RootSpan,
                );

                if (!view) {
                  setActiveViewKey(null);
                  return;
                }
                setActiveViewKey(view.id.toString());
              }}
              viewList={viewList as unknown as View[]}
              visibleViewIds={visibleViewIds}
              onTriggerViewVisible={view => {
                if (visibleViewIds.includes(view.id)) {
                  setVisibleViewIds(
                    visibleViewIds.filter(id => id !== view.id),
                  );
                  lastVisibleIds.current = visibleViewIds.filter(
                    id => id !== view.id,
                  );
                } else {
                  setVisibleViewIds([...visibleViewIds, view.id]);
                  lastVisibleIds.current = [...visibleViewIds, view.id];
                }
              }}
              viewNames={viewNames}
              onDelteView={() => {
                updateViewList();
              }}
              onUpdateView={() => {
                updateViewList();
              }}
            />
            <Button
              className="!w-[32px] !h-[32px]"
              icon={<IconCozRefresh />}
              onClick={handleRefresh}
              color="primary"
            />
          </div>
          <div className="flex gap-1">
            <ColumnSelector
              columns={columns as ColumnItem[]}
              onChange={onColumnsChange}
              buttonText={I18n.t('column_management')}
              resetButtonText={I18n.t('reset_to_default')}
              defaultColumns={defaultColumns as ColumnItem[]}
            />
            {headerSlot}
          </div>
        </div>
      </div>
      {slot}
    </div>
  );
});
