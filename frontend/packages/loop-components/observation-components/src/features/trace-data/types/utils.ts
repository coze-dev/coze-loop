// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export type RemoveUndefinedOrString<T> = T extends undefined | string | null
  ? never
  : T;

export type ValueOf<T> = T[keyof T];

export type PickValueByKey<T, K extends keyof T> = T[K];
