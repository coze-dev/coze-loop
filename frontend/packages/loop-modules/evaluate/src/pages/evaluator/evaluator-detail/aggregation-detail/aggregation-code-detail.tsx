// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
// 实现组件：判断query是否有isPreEvaluator参数，有则表示是预置Code评估器，否则表示是普通Code评估器
import { useLocation } from 'react-router-dom';

import CodeDetail from '../code-detail';
import PresetCodeDetail from './preset-code-detail';

export function AggregationCodeDetail() {
  const location = useLocation();
  const isPreEvaluator =
    new URLSearchParams(location.search).get('isPreEvaluator') === 'true';

  if (isPreEvaluator) {
    return <PresetCodeDetail />;
  }

  return <CodeDetail />;
}
