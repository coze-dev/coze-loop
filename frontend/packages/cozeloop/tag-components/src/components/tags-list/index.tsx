import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';

import { useSearchTags } from '@/hooks/use-search-tags';

import { TagsListTable } from './table';
import { TagsListHeader } from './header';

interface TagsListProps {
  /**
   * 标签列表路由路径，用于跳转和拼接 标签详情 / 创建标签 路由路径
   */
  tagListPagePath?: string;
}

export const TagsList = ({ tagListPagePath }: TagsListProps) => {
  const navigate = useNavigateModule();
  const {
    service,
    searchValue,
    setSearchValue,
    contentTypes,
    setContentTypes,
    createdBys,
    setCreatedBys,
    setOrderBy,
  } = useSearchTags();

  return (
    <div className="flex flex-col gap-3 h-full">
      <TagsListHeader
        searchValue={searchValue}
        onSearchValueChange={value => {
          setSearchValue(value);
        }}
        contentTypes={contentTypes}
        onContentTypesChange={setContentTypes}
        createdBys={createdBys}
        onCreatedBysChange={setCreatedBys}
        onCreateTag={() => navigate(`${tagListPagePath}/create`)}
      />
      <TagsListTable
        service={service}
        setOrderBy={setOrderBy}
        tagListPagePath={tagListPagePath}
      />
    </div>
  );
};
