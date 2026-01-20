// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useSearchParams } from 'react-router-dom';

import { PresetLLMDetail } from './preset-llm-detail';
import EvaluatorDetailPage from '..';

const AggregationLLMEvaluatorDetailPage = () => {
  const [searchParams] = useSearchParams();
  const isPreEvaluator = searchParams.get('isPreEvaluator');
  if (isPreEvaluator) {
    return <PresetLLMDetail />;
  }

  return <EvaluatorDetailPage />;
};

export { AggregationLLMEvaluatorDetailPage };
