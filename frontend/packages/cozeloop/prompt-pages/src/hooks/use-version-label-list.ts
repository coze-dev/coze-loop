// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useInfiniteScroll } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { StonePromptApi } from '@cozeloop/api-schema';

export interface VersionLabelOption {
  value: string;
  label: string;
  promptVersion?: string;
}
export function useVersionLabelList({
  filterKey,
  promptID,
}: {
  promptID?: string;
  filterKey?: string;
}) {
  const { spaceID } = useSpace();

  return useInfiniteScroll<{
    list: VersionLabelOption[];
    cursorID: string;
    hasMore: boolean;
    versionMap: Record<string, string>;
  }>(
    async dataSource => {
      if (!promptID || !spaceID || dataSource?.hasMore === false) {
        return {
          list: [],
          cursorID: '',
          hasMore: false,
          versionMap: { ...dataSource?.versionMap },
        };
      }
      const res = await StonePromptApi.ListLabel({
        workspace_id: spaceID,
        prompt_id: promptID,
        with_prompt_version_mapping: true,
        label_key_like: filterKey,
        page_token: dataSource?.cursorID,
        page_size: 100,
      });
      const list = (res.labels || []).map(item => ({
        label: item.key || '',
        value: item.key || '',
        promptVersion: res.prompt_version_mapping?.[item.key || ''],
      }));

      return {
        list,
        cursorID: res.next_page_token || '',
        hasMore: res.has_more || false,
        versionMap: {
          ...dataSource?.versionMap,
          ...res.prompt_version_mapping,
        },
      };
    },
    {
      reloadDeps: [filterKey],
    },
  );
}
