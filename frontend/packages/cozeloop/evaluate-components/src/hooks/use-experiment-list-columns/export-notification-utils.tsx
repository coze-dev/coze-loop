// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/**
 * 下载文件的工具函数
 */
export const downloadExptExportFile = (url: string, filename?: string) => {
  const link = document.createElement('a');
  link.href = url;
  if (filename) {
    link.download = filename;
  }
  link.target = '_blank';
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
};
