import { PrimaryPage } from '@cozeloop/components';

import { TagsList } from '../components/tags-list';

interface TagsListPageProps {
  /**
   * 标签列表路由路径，用于跳转和拼接 标签详情 / 创建标签 路由路径
   */
  tagListPagePath?: string;
}

export const TagsListPage = ({ tagListPagePath }: TagsListPageProps) => (
  <PrimaryPage pageTitle="标签管理">
    <TagsList tagListPagePath={tagListPagePath} />
  </PrimaryPage>
);
