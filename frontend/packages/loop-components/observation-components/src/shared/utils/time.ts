// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  DEFAULT_FRACTION_DIGITS,
  SECOND_MS,
  MINUTE_MS,
  HOUR_MS,
  DAY_MS,
} from '@/shared/constants';

export function formatTime(time: number) {
  if (time < SECOND_MS) {
    return `${time}ms`;
  } else if (time < MINUTE_MS) {
    return `${(time / SECOND_MS).toFixed(DEFAULT_FRACTION_DIGITS)}s`;
  } else if (time < HOUR_MS) {
    return `${(time / MINUTE_MS).toFixed(DEFAULT_FRACTION_DIGITS)}min`;
  } else if (time < DAY_MS) {
    return `${(time / HOUR_MS).toFixed(DEFAULT_FRACTION_DIGITS)}h`;
  } else {
    return `${(time / DAY_MS).toFixed(DEFAULT_FRACTION_DIGITS)}d`;
  }
}
