// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/**
 * 获取coze 相关地址信息
 * @returns
 */
export function useCozeLocation() {
  const cozeOrigin = window.location.origin.replace('loop.', '');

  return {
    origin: cozeOrigin,
  };
}
