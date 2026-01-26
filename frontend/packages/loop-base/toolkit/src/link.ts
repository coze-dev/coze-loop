// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/**
 * 打开 tab，自动拼接字节云前缀
 * @param path 跳转的pathname，不需要domain
 */
export function openWindow(path: string) {
  window.open(getFullURL(path), '_blank');
}

/**
 * 重新在当前窗口加载新的url，自动拼接字节云前缀
 * @param path 跳转的pathname，不需要domain
 */
export function relaunchWindow(path: string) {
  window.open(getFullURL(path), '_self');
}

export function getFullURL(path: string) {
  const pathWithoutDomain = path.replace(window.location.origin, '');

  const url = `${window.location.origin}/${pathWithoutDomain.replace('/', '')}`;
  return url;
}
