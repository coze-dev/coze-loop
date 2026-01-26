// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export function notEmpty<T>(value: T | null | undefined): value is T {
  return value !== null && value !== undefined;
}
