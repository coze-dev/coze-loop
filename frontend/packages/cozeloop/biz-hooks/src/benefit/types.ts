// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/naming-convention -- skip */
export enum EnterpriseRoleType {
  super_admin = 'SuperAdmin',
  admin = 'Admin',
  member = 'Member',
  guest = 'Guest',
}

export enum UserLevel {
  /** 免费版。 */
  Free = 0,
  /** 海外 PremiumLite */
  PremiumLite = 10,
  /** Premium */
  Premium = 15,
  PremiumPlus = 20,
  /** 国内 V1火山专业版 */
  V1ProInstance = 100,
  /** 个人旗舰版 */
  ProPersonal = 110,
  /** 团队版 */
  Team = 120,
  /** 企业版 */
  Enterprise = 130,
}

export interface SubscriptionDetailV2 {
  /** 用户基本信息 */
  user_basic_info?: {
    user_level: UserLevel;
  };
  benefit_type_infos?: Record<number, number>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- skip
  resource_packages?: Array<any>;
}
