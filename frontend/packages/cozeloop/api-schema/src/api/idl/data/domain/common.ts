// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/** 用户信息 */
export interface UserInfo {
  /** 姓名 */
  name?: string,
  /** 英文名称 */
  en_name?: string,
  /** 用户头像url */
  avatar_url?: string,
  /** 72 * 72 头像 */
  avatar_thumb?: string,
  /** 用户应用内唯一标识 */
  open_id?: string,
  /** 用户应用开发商内唯一标识 */
  union_id?: string,
  /** 用户在租户内的唯一标识 */
  user_id?: string,
  /** 用户邮箱 */
  email?: string,
}
/** 基础信息 */
export interface BaseInfo {
  created_by?: UserInfo,
  updated_by?: UserInfo,
  created_at?: string,
  updated_at?: string,
}