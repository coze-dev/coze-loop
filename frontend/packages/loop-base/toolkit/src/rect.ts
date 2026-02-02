// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type RefObject,
  type UIEventHandler,
  type MutableRefObject,
} from 'react';

import { debounce } from 'lodash-es';

import { isBrowser } from './is-browser';

export const getScrollTop = (el: Document | Element) => {
  if (
    el === document ||
    el === document.documentElement ||
    el === document.body
  ) {
    return Math.max(
      window.pageYOffset,
      document.documentElement.scrollTop,
      document.body.scrollTop,
    );
  }
  return (el as Element).scrollTop;
};

export const getScrollHeight = (el: Document | Element) =>
  (el as Element).scrollHeight ||
  Math.max(document.documentElement.scrollHeight, document.body.scrollHeight);

export const getClientHeight = (el: Document | Element) =>
  (el as Element).clientHeight ||
  Math.max(document.documentElement.clientHeight, document.body.clientHeight);

type TargetValue<T> = T | undefined | null;

type TargetType = HTMLElement | Element | Window | Document;

export type BasicTarget<T extends TargetType = Element> =
  | (() => TargetValue<T>)
  | TargetValue<T>
  | MutableRefObject<TargetValue<T>>;

export function getTargetElement<T extends TargetType>(
  target: BasicTarget<T>,
  defaultElement?: T,
) {
  if (!isBrowser) {
    return undefined;
  }

  if (!target) {
    return defaultElement;
  }

  let targetElement: TargetValue<T>;

  if (typeof target === 'function') {
    targetElement = target();
  } else if ('current' in target) {
    targetElement = target.current;
  } else {
    targetElement = target;
  }

  return targetElement;
}

export const scrollToBottom = (domRef: RefObject<HTMLDivElement>) => {
  if (domRef.current) {
    domRef.current.scrollTop = domRef.current.scrollHeight + 100;
  }
};

export function isScrollAtBottom(
  domRef: RefObject<HTMLDivElement>,
  offset = 0,
) {
  if (!domRef.current) {
    return false;
  }
  // div中内容的总高度
  const { scrollHeight, scrollTop, clientHeight } = domRef.current;
  return scrollTop === scrollHeight - clientHeight - offset;
}

const HEIGHT_BUFFER = 20;

// 使用WeakMap存储每个回调函数的防抖版本
const debouncedCallbacks = new WeakMap<
  () => void,
  ReturnType<typeof debounce>
>();

export const handleScrollToBottom = (
  e: Parameters<UIEventHandler>[0],
  callback: () => void,
  debounceDelay = 100,
) => {
  const { scrollTop, clientHeight, scrollHeight } = e.currentTarget;

  if (scrollTop + clientHeight + HEIGHT_BUFFER >= scrollHeight) {
    // 获取或创建防抖版本的回调函数
    let debouncedCallback = debouncedCallbacks.get(callback);
    if (!debouncedCallback) {
      debouncedCallback = debounce(callback, debounceDelay);
      debouncedCallbacks.set(callback, debouncedCallback);
    }

    debouncedCallback();
  }
};
