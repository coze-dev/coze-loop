import { TagsDetail } from '@/components/tags-detail';

interface TagsDetailPageProps {
  /**
   * 标签列表路由路径，用于跳转和拼接 标签详情 / 创建标签 路由路径
   */
  tagListPagePath?: string;
  /**
   * 标签列表参数，用于跳转和拼接 标签详情 / 创建标签 路由路径的查询参数 格式为 key1=value1&key2=value2 不需要带 ?
   */
  tagListPageQuery?: string;
}

export const TagsDetailPage = ({
  tagListPagePath,
  tagListPageQuery,
}: TagsDetailPageProps) => (
  <TagsDetail
    tagListPagePath={tagListPagePath}
    tagListPageQuery={tagListPageQuery}
  />
);
