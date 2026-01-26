// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import dayjs from 'dayjs';

const UNIX_LEN = 10;
const UNIX_LEN2 = 13;

export const formatTimestampToString = (
  timestamp?: string | number,
  format = 'YYYY-MM-DD HH:mm:ss',
) => {
  if (timestamp === undefined || timestamp === null) {
    return '-';
  }
  const strLen = `${timestamp}`.length;
  if (strLen === UNIX_LEN) {
    return dayjs.unix(Number(timestamp)).format(format);
  } else if (strLen === UNIX_LEN2) {
    return dayjs(Number(timestamp)).format(format);
  }
  return '-';
};

export const formateMsToSeconds = (ms?: number | string) => {
  if (ms === undefined || ms === null) {
    return '-';
  }
  if (Number(ms) < 100) {
    return `${ms}ms`;
  }
  return `${(Number(ms) / 1000).toFixed(2)}s`;
};

export const optionsToMap = (
  options: Array<{ label: string; value: string | number }>,
) =>
  options?.reduce(
    (prev, current) => {
      prev[current.value] = current.label;
      return prev;
    },
    {} as unknown as Record<string, string>,
  );
