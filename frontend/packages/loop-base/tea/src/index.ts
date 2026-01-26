// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export { start, init, config, stop, sendEvent, isStarted } from './tea';

const INNER_EVENT_NAMES: Record<string, string> = {};

export const EVENT_NAMES = new Proxy(INNER_EVENT_NAMES, {
  get(_target, p, _receiver) {
    return p;
  },
  // eslint-disable-next-line max-params -- skip proxy.set
  set(_target, _p, _newValue, _receiver) {
    // do nothing
    return false;
  },
});

export type ParamsTypeDefine = Record<string, Record<string, string>>;
