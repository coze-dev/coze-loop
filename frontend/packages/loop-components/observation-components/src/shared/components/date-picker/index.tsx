// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import React, { forwardRef, useImperativeHandle, useRef } from 'react';

import dayjs from 'dayjs';
import {
  IconCozArrowDown,
  IconCozCalendar,
} from '@coze-arch/coze-design/icons';
import { type DatePickerProps } from '@coze-arch/coze-design';
import { DatePicker, Select, InputGroup } from '@coze-arch/coze-design';

import { dayJsTimeZone } from '@/shared/utils/dayjs';
import {
  calcPresetTime,
  PresetRange,
  YEAR_DAY_COUNT,
} from '@/features/trace-list/constants/time';

import styles from './index.module.less';

export interface TimeStamp {
  startTime: number;
  endTime: number;
}

export interface PreselectedDatePickerOption {
  label: React.ReactNode;
  value: number | string;
  disabled?: boolean;
}

export interface PreselectedDatePickerProps {
  preset: PresetRange;
  timeStamp: TimeStamp;
  datePickerOptions: PreselectedDatePickerOption[];
  maxPastDateRange?: number;
  datePickerProps?: DatePickerProps;
  onPresetChange: (preset: PresetRange, presetTimeStamp?: TimeStamp) => void;
  oneTimeStampChange: (timeStamp: TimeStamp) => void;
  disabled?: boolean;
}

export interface PreselectedDatePickerRef {
  getCurrentTime: () => TimeStamp | undefined;
  closeSelect: () => void;
  closeDatePicker: () => void;
}

export const PreselectedDatePicker = forwardRef<
  PreselectedDatePickerRef,
  PreselectedDatePickerProps
>((props, ref) => {
  const startTime = props.timeStamp?.startTime;
  const endTime = props.timeStamp?.endTime;

  const amendStartTime = startTime ? Number(startTime) : undefined;
  const amendEndTime = endTime ? Number(endTime) : undefined;

  const selectRef = useRef<any>(null);

  const datePickerRef = useRef<any>(null);
  useImperativeHandle(
    ref,
    () => ({
      getCurrentTime: () => {
        const time = calcPresetTime(props.preset as PresetRange);
        return time;
      },
      closeSelect: () => {
        selectRef.current?.close?.();
      },
      closeDatePicker: () => {
        datePickerRef.current?.close?.();
      },
    }),
    [props.preset],
  );

  // 格式化时间戳显示函数
  const formatTimeDisplay = () => {
    if (!amendStartTime || !amendEndTime) {
      return '';
    }

    const startFormatted = dayJsTimeZone(amendStartTime).format(
      'YYYY-MM-DD HH:mm:ss',
    );
    const endFormatted = dayJsTimeZone(amendEndTime).format(
      'YYYY-MM-DD HH:mm:ss',
    );
    return `${startFormatted} ~ ${endFormatted}`;
  };

  // 自定义日期选择器渲染函数
  const customTriggerRender = () => (
    <div className={styles.datePickerTrigger}>
      <Select
        value={formatTimeDisplay()}
        showArrow={false}
        showClear={false}
        emptyContent={null}
        disabled={props.preset !== PresetRange.Unset || props.disabled}
        suffix={
          <IconCozArrowDown className="coz-fg-secondary !mx-2 w-[14px] h-[14px] !text-[14px]" />
        }
        prefix={
          <IconCozCalendar className="coz-fg-secondary mx-2 w-3.5 h-3.5" />
        }
      ></Select>
    </div>
  );

  const handlePresetChange = (v: unknown) => {
    const time = calcPresetTime(v as PresetRange);
    if (!time) {
      props.onPresetChange(v as PresetRange, {
        startTime: amendStartTime ?? dayjs().subtract(3, 'day').valueOf(),
        endTime: amendEndTime ?? dayjs().valueOf(),
      });
    } else {
      props.onPresetChange(v as PresetRange, {
        startTime: time.startTime ?? dayjs().subtract(3, 'day').valueOf(),
        endTime: time.endTime ?? dayjs().valueOf(),
      });
    }
  };

  return (
    <>
      {props.preset === PresetRange.Unset ? (
        <InputGroup className="box-border !flex-nowrap">
          <Select
            disabled={props.disabled}
            ref={selectRef}
            className="min-w-[136px] max-w-[136px] box-border"
            maxHeight={320}
            dropdownClassName={styles.presetTimeDropdown}
            optionList={props.datePickerOptions}
            value={props.preset}
            onChange={handlePresetChange}
          />
          <DatePicker
            ref={datePickerRef}
            className={styles.datePicker}
            disabledDate={date => {
              if (date && date.getTime() > dayjs().endOf('day').valueOf()) {
                return true;
              }
              const dayCount = dayjs().diff(dayjs(date), 'days');
              return dayCount > (props.maxPastDateRange ?? YEAR_DAY_COUNT);
            }}
            disabled={props.preset !== PresetRange.Unset || props.disabled}
            type="dateTimeRange"
            value={[amendStartTime ?? '', amendEndTime ?? '']}
            onChange={range => {
              const start = (range as Date[])?.[0];
              const end = (range as Date[])?.[1];
              props.oneTimeStampChange({
                startTime: new Date(start).valueOf(),
                endTime: new Date(end).valueOf(),
              });
            }}
            showClear={false}
            triggerRender={customTriggerRender}
            {...props.datePickerProps}
          />
        </InputGroup>
      ) : (
        <Select
          ref={selectRef}
          disabled={props.disabled}
          className="min-w-[136px] max-w-[136px] box-border"
          maxHeight={320}
          dropdownClassName={styles.presetTimeDropdown}
          optionList={props.datePickerOptions}
          value={props.preset}
          onChange={handlePresetChange}
        />
      )}
    </>
  );
});
