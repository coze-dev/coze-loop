// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback } from 'react';

import { type GuardPoint, type GuardResultDataMap } from '../../types';
import { useGuardStrategy } from '../../context';

export function useGuards({ points }: { points: GuardPoint[] }) {
  const strategy = useGuardStrategy();

  // 为所有请求的点位创建数据
  const data = points.reduce<GuardResultDataMap>(
    (acc, point) => ({ ...acc, [point]: strategy.checkGuardPoint(point) }),
    // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
    {} as GuardResultDataMap,
  );

  return {
    data,
    updateData: strategy.refreshGuardData,
    loading: strategy.loading,
  };
}

export function useGuard({ point }: { point: GuardPoint }) {
  const strategy = useGuardStrategy();
  const guardData = strategy.checkGuardPoint(point);

  const updateData = useCallback(async () => {
    await strategy.refreshGuardData();
  }, [strategy, point]);

  return {
    data: guardData,
    updateData,
    loading: strategy.loading,
  };
}
