import { Routes, Route, Navigate } from 'react-router-dom';

import {
  TagsListPage,
  TagsDetailPage,
  TagsCreatePage,
} from '@cozeloop/tag-components';
// 当前模块路由前缀
const TAG_MODULE_BASE_PATH = 'tag';
// 标签列表路由路径，用于跳转和拼接 标签详情 / 创建标签 路由路径
const tagListPagePath = `${TAG_MODULE_BASE_PATH}/tag`;

const App = () => (
  <div className="text-sm h-full overflow-hidden">
    <Routes>
      <Route index element={<Navigate to="tag" replace />} />
      {/* path 均为标签模块-标签管理路由，tagBasePath 为标签模块-标签管理路由前缀，所以实际完整路由为 tag/tag */}
      <Route
        path="tag"
        element={<TagsListPage tagListPagePath={tagListPagePath} />}
      />
      <Route
        path="tag/:tagId"
        element={<TagsDetailPage tagListPagePath={tagListPagePath} />}
      />
      <Route
        path="tag/create"
        element={<TagsCreatePage tagListPagePath={tagListPagePath} />}
      />
    </Routes>
  </div>
);

export default App;
