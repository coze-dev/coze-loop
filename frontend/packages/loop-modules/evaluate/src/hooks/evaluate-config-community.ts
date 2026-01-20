// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useLayoutEffect } from 'react';

import { useGlobalEvalConfig } from '@cozeloop/evaluate-components';
import { PlatformType } from '@cozeloop/api-schema/observation';

/** 评测社区版配置初始化 */
export function useEvaluateConfigCommunityInit() {
  const { setTracePlatformType } = useGlobalEvalConfig();
  useLayoutEffect(() => {
    setTracePlatformType({
      // TODO: 这里等观测枚举添加完后修改为评测专用平台类型
      traceEvalTargetPlatformType: PlatformType.EvaluationTarget,
      traceEvaluatorPlatformType: PlatformType.Evaluator,
    });
  }, []);
}
