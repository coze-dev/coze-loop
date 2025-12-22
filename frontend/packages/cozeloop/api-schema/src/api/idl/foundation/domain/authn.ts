// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export interface PersonalAccessToken {
  id: string,
  name: string,
  /** unix，秒 */
  created_at: string,
  /** unix，秒 */
  updated_at: string,
  /** unix，秒，-1 表示未使用 */
  last_used_at: string,
  /** unix，秒 */
  expire_at: string,
}