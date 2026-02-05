// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/use-error-in-catch */
import { type UIEventHandler } from 'react';

import { uniqueId } from 'lodash-es';

export const safeParseJson = <T>(
  jsonString?: string,
  fallback?: T,
): T | undefined => {
  try {
    if (jsonString) {
      return JSON.parse(jsonString) as T;
    }
  } catch (e) {
    return fallback;
  }
};

const HEIGHT_BUFFER = 20;
export const handleScrollToBottom = (
  e: Parameters<UIEventHandler>[0],
  callback: () => void,
) => {
  const { scrollTop, clientHeight, scrollHeight } = e.currentTarget;
  if (scrollTop + clientHeight + HEIGHT_BUFFER >= scrollHeight) {
    callback();
  }
};

export function sleep(timer = 600) {
  return new Promise<void>(resolve => {
    setTimeout(() => resolve(), timer);
  });
}
export const messageId = () => {
  const date = new Date();
  return date.getTime() + uniqueId();
};

const thousand = 1000;
export const convertNumberToThousands = (num?: Int64) => {
  if (!num) {
    return 0;
  }
  const number = Number(num);
  if (number < thousand) {
    return number;
  } else {
    return `${(number / thousand).toFixed(1)} K`;
  }
};

export function recordToArray(
  record: Record<string, string>,
): { label: string; value?: string }[] {
  return Object.entries(record).map(([key, value]) => ({
    label: key,
    value, // 将值转换为字符串，如果值为 undefined 则不设置
  }));
}
