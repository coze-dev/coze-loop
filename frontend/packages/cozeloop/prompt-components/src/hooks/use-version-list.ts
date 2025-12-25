// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { useInfiniteScroll } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type ListCommitResponse,
  type CommitInfo,
} from '@cozeloop/api-schema/prompt';
import { type UserInfoDetail } from '@cozeloop/api-schema/foundation';
import { StonePromptApi } from '@cozeloop/api-schema';

export const useVersionList = ({
  promptID,
  draftVersion,
}: {
  promptID?: string;
  draftVersion?: CommitInfo & { user?: UserInfoDetail };
}) => {
  const { spaceID } = useSpace();
  const { loading, data, loadMore, reload, loadingMore, mutate } =
    useInfiniteScroll<{
      list: CommitInfo[];
      cursorID: string;
      hasMore: boolean;
      versionLabelMap: ListCommitResponse['commit_version_label_mapping'];
    }>(
      async dataSource => {
        if (!promptID || !spaceID) {
          return {
            list: draftVersion ? [draftVersion] : [],
            cursorID: '',
            hasMore: false,
            versionLabelMap: { ...dataSource?.versionLabelMap },
          };
        }
        const resp = await StonePromptApi.ListCommit({
          page_token: dataSource?.cursorID,
          page_size: 10,
          prompt_id: promptID || '',
        }).catch(() => undefined);

        const versionLabelMap = {
          ...dataSource?.versionLabelMap,
          ...(resp?.commit_version_label_mapping || {}),
        };
        if (resp?.prompt_commit_infos?.length) {
          const newList = resp?.prompt_commit_infos?.map(it => {
            const user = resp.users?.find(u => u.user_id === it.committed_by);
            return { ...it, user };
          });
          if (!dataSource?.cursorID) {
            return {
              list: draftVersion ? [draftVersion, ...newList] : newList,
              cursorID: resp.next_page_token || '',
              hasMore: resp.has_more || false,
              versionLabelMap,
            };
          }
          return {
            list: newList || [],
            cursorID: resp.next_page_token || '',
            hasMore: resp.has_more || false,
            versionLabelMap,
          };
        } else {
          return {
            list: draftVersion ? [draftVersion] : [],
            cursorID: '',
            hasMore: false,
            versionLabelMap,
          };
        }
      },
      {
        manual: true,
        reloadDeps: [spaceID, promptID, draftVersion?.version],
        isNoMore: dataSource => !dataSource?.hasMore,
      },
    );

  return {
    versionListLoading: loading,
    versionListData: data,
    versionListLoadMore: loadMore,
    versionListReload: reload,
    versionListLoadingMore: loadingMore,
    versionListMutate: mutate,
  };
};
