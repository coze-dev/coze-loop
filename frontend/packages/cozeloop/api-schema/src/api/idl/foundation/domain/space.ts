// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/** 空间类型 */
export enum SpaceType {
  Undefined = 0,
  /** 个人空间 */
  Personal = 1,
  /** 团队空间 */
  Team = 2,
  /** 官方空间 */
  Official = 3,
}
/** 空间 */
export interface Space {
  /** 空间ID */
  id: string,
  /** 空间名称 */
  name: string,
  /** 空间描述 */
  description: string,
  /** 空间类型 */
  space_type: SpaceType,
  /** 空间所有者 */
  owner_user_id: string,
  /** 创建时间 */
  create_at?: string,
  /** 更新时间 */
  update_at?: string,
  /**
   * 8-10 保留位
   * 企业ID
  */
  enterprise_id?: string,
  /** 组织ID */
  organization_id?: string,
}