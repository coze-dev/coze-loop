// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback, type ReactNode } from 'react';

import { useDebounceFn, useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';
import {
  type RenderSelectedItemFn,
  type SelectProps,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

const MAX_RENDER_TAG = 100;
type TagInfo = tag.TagInfo;

const genTagOption = (tagInfo: TagInfo) => ({
  value: tagInfo.tag_key_id,
  label: (
    <div className="w-full max-w-full min-w-0 pr-2 flex items-center gap-x-1">
      <Typography.Text className="max-w-full" ellipsis={{ showTooltip: true }}>
        {tagInfo.tag_key_name}
      </Typography.Text>
      {tagInfo.status === tag.TagStatus.Inactive ? (
        <Tag color="primary">{I18n.t('disable')}</Tag>
      ) : null}
    </div>
  ),
  ...tagInfo,
});

export function DatasetTagsFilterSelect(props: SelectProps) {
  const { spaceID } = useSpace();

  const service = useRequest(async (text?: string) => {
    const res = await DataApi.SearchTags({
      workspace_id: spaceID,
      tag_key_name_like: text || undefined,
      domain_types: [tag.TagDomainType.Evaluation],
      page_size: MAX_RENDER_TAG,
    });
    return res.tagInfos?.filter(item => item.tag_key_id).map(genTagOption);
  });

  const handleSearch = useDebounceFn(service.run, {
    wait: 500,
  });

  const fetchOptionsByIds = useCallback(
    async value => {
      if (!value) {
        return [];
      }
      const tagIDs = Array.isArray(value) ? value : [value];
      const res = await DataApi.BatchGetTags({
        workspace_id: spaceID,
        tag_key_ids: tagIDs.map(String),
      });
      return (
        res.tag_info_list?.filter(item => item.tag_key_id).map(genTagOption) ||
        []
      );
    },
    [spaceID],
  );

  const renderSelectedItem = useCallback(
    (optionNode?: Record<string, unknown>, multipleProps?: unknown) => {
      if (multipleProps) {
        return {
          isRenderInTag: true,
          content: (
            <Typography.Text
              className="max-w-[100px]"
              ellipsis={{ showTooltip: true }}
            >
              <>{optionNode?.tag_key_name || optionNode?.value}</>
            </Typography.Text>
          ),
        };
      }
      return (optionNode?.label || optionNode?.value) as ReactNode;
    },
    [],
  );

  return (
    <BaseSearchSelect
      {...props}
      filter
      remote
      multiple
      showClear
      maxTagCount={2}
      placeholder={I18n.t('tag_tag_name')}
      loading={service.loading}
      renderSelectedItem={renderSelectedItem as RenderSelectedItemFn}
      optionList={service.data}
      loadOptionByIds={fetchOptionsByIds}
      showRefreshBtn={true}
      onSearch={handleSearch.run}
      onClickRefresh={() => service.run()}
    />
  );
}
