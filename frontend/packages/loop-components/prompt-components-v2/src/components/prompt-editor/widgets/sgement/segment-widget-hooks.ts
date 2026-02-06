// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { useRequest } from 'ahooks';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';

import { convertSnippetsToMap } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';

interface ISegmentWidget {
  isActive: boolean;
  toggleActive: () => void;
  segmentInfo?: Prompt;
  ladoingSegment?: boolean;
}
export const useSegmentWidget = ({
  spaceID,
  segmengId,
  sgementVersion,
  hasSubPrompt,
}: {
  spaceID: string;
  segmengId?: Int64;
  sgementVersion?: string;
  hasSubPrompt?: boolean;
}): ISegmentWidget => {
  const { setSnippetMap } = usePromptStore.getState();
  const [isActive, setIsActive] = useState(false);
  const [ladoingSegment, setLoadingSegment] = useState(false);
  const [segmentInfo, setSgementInfo] = useState<Prompt | undefined>(undefined);

  const getPromptService = useRequest(
    () =>
      StonePromptApi.GetPrompt({
        workspace_id: spaceID,
        prompt_id: `${segmengId ?? ''}`,
        commit_version: sgementVersion,
        expand_snippet: true,
      }),
    {
      refreshDeps: [segmengId, sgementVersion, spaceID],
      ready: Boolean(
        segmengId && sgementVersion && spaceID && hasSubPrompt && isActive,
      ),
      onBefore: () => {
        setLoadingSegment(true);
      },
      onSuccess: res => {
        if (res?.prompt) {
          setSgementInfo(res.prompt);
          const subPrompts =
            res?.prompt?.prompt_commit?.detail?.prompt_template?.snippets || [];
          setSnippetMap(map => ({
            ...map,
            ...convertSnippetsToMap([res.prompt as Prompt, ...subPrompts]),
          }));
        }

        setLoadingSegment(false);
      },
      onError: () => {
        setLoadingSegment(false);
      },
    },
  );

  const toggleActive = () => {
    setIsActive(v => {
      const val = !v;
      if (val) {
        getPromptService.run();
      }
      return val;
    });
  };
  return {
    isActive,
    toggleActive,
    segmentInfo,
    ladoingSegment,
  };
};
