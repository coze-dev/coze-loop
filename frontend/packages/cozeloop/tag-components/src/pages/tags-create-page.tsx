// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { useBreadcrumb } from '@cozeloop/base-hooks';
import { IconCozLongArrowUp } from '@coze-arch/coze-design/icons';
import { Button, Spin, Toast } from '@coze-arch/coze-design';

import { useGetTagSpec } from '@/hooks/use-get-tag-spec';
import { useCreateTag } from '@/hooks/use-create-tag';
import {
  TagsForm,
  type TagFormRef,
  type FormValues,
} from '@/components/tags-form';

interface TagsCreatePageProps {
  onCreateSuccess?: () => void;
  /**
   * 标签列表路由路径，用于跳转和拼接 标签详情 / 创建标签 路由路径
   */
  tagListPagePath?: string;
  /**
   * 标签列表参数，用于跳转和拼接 标签详情 / 创建标签 路由路径的查询参数 格式为 key1=value1&key2=value2 不需要带 ?
   */
  tagListPageQuery?: string;
}

export const TagsCreatePage = ({
  onCreateSuccess,
  tagListPagePath,
  tagListPageQuery,
}: TagsCreatePageProps) => {
  const navigate = useNavigateModule();
  const tagFormRef = useRef<TagFormRef>(null);
  const service = useCreateTag();
  useBreadcrumb({
    text: I18n.t('create_tag'),
  });

  const { data: tagSpec, loading: tagSpecLoading } = useGetTagSpec();

  const handleSubmit = (values: FormValues) => {
    service.runAsync(values).then(() => {
      onCreateSuccess?.();
      navigate(
        `${tagListPagePath}${tagListPageQuery ? `?${tagListPageQuery}` : ''}`,
      );
      Toast.success(I18n.t('create_success'));
    });
  };

  if (tagSpecLoading) {
    return (
      <div className="w-full h-full flex items-center justify-center">
        <Spin />
      </div>
    );
  }

  return (
    <div className="h-full max-h-full overflow-hidden flex flex-col ">
      <div className="w-full h-[56px] flex items-center box-border py-4 px-6 gap-x-2">
        <div
          className="-rotate-90 cursor-pointer"
          onClick={() =>
            navigate(
              `${tagListPagePath}${tagListPageQuery ? `?${tagListPageQuery}` : ''}`,
            )
          }
        >
          <IconCozLongArrowUp className="w-5 h-5" />
        </div>
        <div className="text-[20px] font-medium leading-5 text-[var(--coz-fg-plus)]">
          {I18n.t('create_tag')}
        </div>
      </div>
      <div className="pt-4 px-[52px] flex justify-center max-w-full flex-1 overflow-auto styled-scroll">
        <div className="w-[800px]">
          <TagsForm
            entry="crete-tag"
            onSubmit={handleSubmit}
            ref={tagFormRef}
            maxTags={tagSpec?.max_total}
          />
        </div>
      </div>
      <div className="flex items-center justify-end w-[800px] mx-auto py-6">
        <Button color="brand" onClick={() => tagFormRef.current?.submit()}>
          {I18n.t('create')}
        </Button>
      </div>
    </div>
  );
};
