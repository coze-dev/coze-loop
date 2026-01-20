// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useFetchUserBenefit } from './use-fetch-user-benefit';
import {
  type EnterpriseRoleType,
  type UserLevel,
  type SubscriptionDetailV2,
} from './types';

export interface BenefitConfig {
  /** 评测免费次数 */
  evaluateTotal: number;
  /** 评测使用次数 */
  evaluateUsed: number;
  /** Trace 免费额度 */
  traceTotal: number;
  /** Trace 使用额度 */
  traceUsed: number;
  /** 空间额度 */
  spaceTotal: number;
  /** 空间使用额度 */
  spaceUsed: number;
  /** 是否欠费 */
  isInArrears: boolean;
  /** 是否免费 token 用尽 */
  isQuotaUseUp: boolean;
  /** 是否套餐到期 */
  isPlanExpired: boolean;
  /** 用户等级 */
  userLevel: UserLevel;
  /** 当前用户在企业中的角色，如果是个人用户，则为空数组 */
  roles: EnterpriseRoleType[];
  /** 个人账号 */
  isPersonalAccount: boolean;
  /** 是否是企业管理员 */
  isEnterpriseAdmin: boolean;
}

export function useBenefit(): {
  data?: BenefitConfig;
  fetchUserBenefit: () => Promise<SubscriptionDetailV2>;
  loading: boolean;
} {
  const { fetchUserBenefitAsync, loading } = useFetchUserBenefit();

  return {
    data: undefined,
    fetchUserBenefit: fetchUserBenefitAsync,
    loading,
  };
}
