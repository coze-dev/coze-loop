// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/** UserInfoDetail 用户详细信息，包含姓名、头像等 */
export interface UserInfoDetail {
  /** 唯一名称 */
  name?: string,
  /** 用户昵称 */
  nick_name?: string,
  /** 用户头像url */
  avatar_url?: string,
  /** 用户邮箱 */
  email?: string,
  /** 手机号 */
  mobile?: string,
  /** 用户在租户内的唯一标识 */
  user_id?: string,
}
export enum UserStatus {
  active = "active",
  deactivated = "deactivated",
  offboarded = "offboarded",
}