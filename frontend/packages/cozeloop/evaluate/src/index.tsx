// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
// 评估器
export { default as EvaluatorListPage } from './pages/evaluator/evaluator-list';
export { default as EvaluatorDetailPage } from './pages/evaluator/evaluator-detail';
export { default as EvaluatorCreatePage } from './pages/evaluator/evaluator-create';

// 评测集
export { DatasetListPage } from '@cozeloop/evaluate-components';
export { CreateDatasetPage as DatasetCreatePage } from '@cozeloop/evaluate-components';
export { default as DatasetDetailPage } from './pages/dataset/detail';

// 实验
export { default as ExperimentListPage } from './pages/experiment/list';
export { default as ExperimentDetailPage } from './pages/experiment/detail';
export { default as ExperimentContrastPage } from './pages/experiment/contrast';
export { default as ExperimentCreatePage } from './pages/experiment/create';

export { useEvaluateConfigCommunityInit } from './hooks/evaluate-config-community';
