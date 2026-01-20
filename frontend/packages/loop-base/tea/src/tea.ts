// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
const globalTeaConfig = {
  started: false,
};

const isDev = process.env.NODE_ENV === 'development';

export function sendEvent<TEventName = string>(
  event: TEventName,
  params?: unknown,
) {
  isDev && console.info(event, params);
}

export const init = params => isDev && console.info('Init 🍵 with', params);

export const config = params => isDev && console.info('Config 🍵 with', params);

export const start = () => {
  globalTeaConfig.started = true;
  isDev && console.info('Start 🍵');
};

export const stop = () => {
  isDev && console.info('Stop 🍵');
  globalTeaConfig.started = false;
};

export const isStarted = () => globalTeaConfig.started;
