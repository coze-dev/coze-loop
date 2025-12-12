/* eslint-disable complexity */
import { useInfiniteScroll } from 'ahooks';
import { DEFAULT_PAGE_SIZE } from '@cozeloop/components';
import {
  type ListCommitResponse,
  type CommitInfo,
} from '@cozeloop/api-schema/prompt';
import { type UserInfoDetail } from '@cozeloop/api-schema/foundation';
import { StonePromptApi } from '@cozeloop/api-schema';

export type VersionItem = CommitInfo & { user?: UserInfoDetail };

interface VersionListParams {
  spaceID?: string;
  promptID?: string;
  draftVersion?: VersionItem;
  withCommitDetail?: boolean;
}

export const useVersionList = ({
  spaceID,
  promptID,
  draftVersion,
  withCommitDetail,
}: VersionListParams) =>
  useInfiniteScroll<{
    list: VersionItem[];
    cursorID: string;
    hasMore: boolean;
    versionLabelMap: ListCommitResponse['commit_version_label_mapping'];
    parentReferencesMap?: ListCommitResponse['parent_references_mapping'];
    promptCommitMap?: ListCommitResponse['prompt_commit_detail_mapping'];
  }>(
    async dataSource => {
      const isFirstPage = !dataSource?.cursorID;

      if (!promptID || !spaceID) {
        return {
          list: isFirstPage && draftVersion ? [draftVersion] : [],
          cursorID: '',
          hasMore: false,
          versionLabelMap: { ...dataSource?.versionLabelMap },
          parentReferencesMap: { ...dataSource?.parentReferencesMap },
          promptCommitMap: { ...dataSource?.promptCommitMap },
        };
      }

      const resp = await StonePromptApi.ListCommit({
        page_token: dataSource?.cursorID,
        page_size: DEFAULT_PAGE_SIZE,
        prompt_id: promptID,
        with_commit_detail: withCommitDetail,
      }).catch(() => undefined);

      const apiList =
        resp?.prompt_commit_infos?.map(it => {
          const user = resp.users?.find(u => u.user_id === it.committed_by);
          return { ...it, user };
        }) || [];

      const list =
        isFirstPage && draftVersion ? [draftVersion, ...apiList] : apiList;

      return {
        list,
        cursorID: resp?.next_page_token || '',
        hasMore: resp?.has_more || false,
        versionLabelMap: {
          ...dataSource?.versionLabelMap,
          ...(resp?.commit_version_label_mapping || {}),
        },
        parentReferencesMap: {
          ...dataSource?.parentReferencesMap,
          ...(resp?.parent_references_mapping || {}),
        },
        promptCommitMap: {
          ...dataSource?.promptCommitMap,
          ...(resp?.prompt_commit_detail_mapping || {}),
        },
      };
    },
    {
      manual: true,
      reloadDeps: [spaceID, promptID, draftVersion?.version],
      isNoMore: dataSource => !dataSource?.hasMore,
    },
  );
