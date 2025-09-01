import { debounce } from 'lodash-es';
import cls from 'classnames';
import { GuardPoint, useGuard, GuardActionType } from '@cozeloop/guard';
import { UserSelect } from '@cozeloop/biz-components-adapter';
import { type tag } from '@cozeloop/api-schema/data';
import { IconCozPlus, IconCozMagnifier } from '@coze-arch/coze-design/icons';
import { Button, Select, Search } from '@coze-arch/coze-design';

import { TAG_TYPE_OPTIONS } from '@/const';

const DEBOUNCE_TIME = 300;
interface TagListHeaderProps {
  searchValue?: string;
  onSearchValueChange?: (value: string) => void;
  contentTypes?: tag.TagContentType[];
  onContentTypesChange?: (value: tag.TagContentType[]) => void;
  createdBys?: string[];
  onCreatedBysChange?: (value: string[]) => void;
  onCreateTag?: () => void;
  className?: string;
}

export const TagsListHeader = (props: TagListHeaderProps) => {
  const {
    searchValue,
    onSearchValueChange,
    contentTypes,
    onContentTypesChange,
    createdBys,
    onCreatedBysChange,
    onCreateTag,
    className,
  } = props;

  const guard = useGuard({
    point: GuardPoint['data.tag.create'],
  });
  return (
    <div className={cls('flex items-center justify-between', className)}>
      <div className="flex items-center gap-x-2">
        {/* Search组件存在clear按钮时，会有样式闪烁的问题，所以外面套一层div */}
        <div className="w-60">
          <Search
            className="box-border !w-full !mr-0 !pr-0"
            placeholder="请输入标签名称"
            value={searchValue}
            onSearch={debounce(e => {
              onSearchValueChange?.(e as string);
            }, DEBOUNCE_TIME)}
            prefix={<IconCozMagnifier />}
            showClear
            autoComplete="off"
          />
        </div>
        <Select
          className="box-border w-[180px]"
          placeholder="请输入标签类型"
          optionList={TAG_TYPE_OPTIONS}
          multiple
          value={contentTypes}
          onChange={debounce(value => {
            onContentTypesChange?.(value as tag.TagContentType[]);
          }, DEBOUNCE_TIME)}
          maxTagCount={2}
          showClear
        />
        <UserSelect
          placeholder="请输入创建人"
          value={createdBys}
          onChange={debounce(value => {
            onCreatedBysChange?.(value as string[]);
          }, DEBOUNCE_TIME)}
          maxTagCount={2}
          multiple
        />
      </div>
      <Button
        color="brand"
        icon={<IconCozPlus />}
        onClick={() => onCreateTag?.()}
        disabled={guard.data.type === GuardActionType.READONLY}
      >
        新建标签
      </Button>
    </div>
  );
};
