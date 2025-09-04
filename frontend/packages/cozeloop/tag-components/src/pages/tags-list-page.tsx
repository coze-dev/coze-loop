import { I18n } from '@cozeloop/i18n-adapter';
import { PrimaryPage } from '@cozeloop/components';

import { TagsList } from '../components/tags-list';

interface TagsListPageProps {
  /**
   * 标签列表路由路径，用于跳转和拼接 标签详情 / 创建标签 路由路径
   */
  tagListPagePath?: string;
}

export const TagsListPage = ({ tagListPagePath }: TagsListPageProps) => (
  <PrimaryPage pageTitle={I18n.t('tag_management')}>
    <TagsList tagListPagePath={tagListPagePath} />
  </PrimaryPage>
);
