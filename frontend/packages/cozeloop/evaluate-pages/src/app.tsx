// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Routes, Route, Navigate } from 'react-router-dom';

import {
  // 评测集
  DatasetDetailPage,
  DatasetCreatePage,
  DatasetListPage,
  // 评估器
  EvaluatorListPage,
  EvaluatorCreatePage,
  EvaluatorDetailPage,
  // 实验
  ExperimentListPage,
  ExperimentDetailPage,
  ExperimentCreatePage,
  ExperimentContrastPage,
  useEvaluateConfigCommunityInit,
} from '@cozeloop/evaluate';

const App = () => {
  /** 初始化社区版评测配置 */
  useEvaluateConfigCommunityInit();

  return (
    <div className="text-sm h-full overflow-hidden">
      <Routes>
        <Route path="" element={<Navigate to="experiments" replace />} />
        {/* 评测集 */}
        <Route path="datasets" element={<DatasetListPage />} />
        <Route path="datasets/create" element={<DatasetCreatePage />} />
        <Route path="datasets/:id" element={<DatasetDetailPage />} />
        {/* 评估器 */}
        <Route path="evaluators" element={<EvaluatorListPage />} />
        <Route
          path="evaluators/create/:id?"
          element={<EvaluatorCreatePage />}
        />
        <Route path="evaluators/:id" element={<EvaluatorDetailPage />} />
        {/* 实验 */}
        <Route path="experiments" element={<Navigate to="list" replace />} />
        <Route path="experiments/list" element={<ExperimentListPage />} />
        <Route path="experiments/create" element={<ExperimentCreatePage />} />
        <Route
          path="experiments/:experimentID"
          element={<ExperimentDetailPage />}
        />
        <Route
          path="experiments/contrast"
          element={<ExperimentContrastPage />}
        />
      </Routes>
    </div>
  );
};

export default App;
