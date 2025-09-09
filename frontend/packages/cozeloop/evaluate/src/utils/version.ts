// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const compareVersions = (version1: string, version2: string): number => {
  const v1Parts = version1.split('.').map(Number);
  const v2Parts = version2.split('.').map(Number);
  const maxLength = Math.max(v1Parts.length, v2Parts.length);

  for (let i = 0; i < maxLength; i++) {
    const num1 = v1Parts[i] || 0;
    const num2 = v2Parts[i] || 0;

    if (num1 > num2) {
      return 1;
    } else if (num1 < num2) {
      return -1;
    }
  }

  return 0;
};

/**
 * 将版本号加 1，从最右侧部分开始加。
 * @param version 输入的版本号字符串，格式为 x.y.z
 * @returns 加 1 后的版本号字符串
 */
export const incrementVersion = (version: string): string => {
  const parts = version.split('.').map(Number);
  for (let i = parts.length - 1; i >= 0; i--) {
    parts[i]++;
    return parts.join('.');
  }
  // 如果版本号为空，返回 1
  return '0.0.1';
};
