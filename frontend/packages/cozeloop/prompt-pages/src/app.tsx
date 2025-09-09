// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Routes, Route, Navigate } from 'react-router-dom';

import { Playground } from './playground';
import { PromptList } from './list';
import { PromptDevelop } from './develop';

const App = () => (
  <div className="text-sm h-full overflow-hidden">
    <Routes>
      <Route path="" element={<Navigate to="prompts" replace />} />
      {/* PE 列表 */}
      <Route path="prompts" element={<PromptList />} />
      <Route path="prompts/:promptID" element={<PromptDevelop />} />
      <Route path="playground" element={<Playground />} />
    </Routes>
  </div>
);

export default App;
