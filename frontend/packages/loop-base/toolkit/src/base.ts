// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const SLEEP_TIME = 600;

export async function sleep(timer = 2000) {
  return new Promise<void>(resolve => {
    setTimeout(() => resolve(), timer);
  });
}

export const exhaustiveCheck = (_v: never) => undefined;
