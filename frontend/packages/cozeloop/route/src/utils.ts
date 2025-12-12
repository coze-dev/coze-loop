// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export function paddingPath(path: string) {
  if (path.startsWith('/')) {
    return path;
  }
  return `/${path}`;
}

export function getPath(path: string, baseURL: string) {
  if (!baseURL) {
    return path;
  }
  if (path.startsWith(baseURL)) {
    console.warn(`You can directly use ${path.replace(`${baseURL}/`, '')}`);
    return path;
  }
  return `${baseURL}${paddingPath(path)}`;
}
