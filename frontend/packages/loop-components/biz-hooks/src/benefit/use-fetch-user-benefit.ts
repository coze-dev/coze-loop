// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';

import { type SubscriptionDetailV2 } from './types';

interface UseFetchUserBenefitProps {
  /** 是否手动触发请求，默认true */
  manual?: boolean;
}

/**
 * 请求当前用户权益数据
 */
export function useFetchUserBenefit({
  manual = true,
}: UseFetchUserBenefitProps = {}) {
  const result = useRequest(
    async () => Promise.resolve([] as SubscriptionDetailV2),
    { manual },
  );

  return {
    ...result,
    fetchUserBenefit: result.run,
    fetchUserBenefitAsync: result.runAsync,
  };
}
