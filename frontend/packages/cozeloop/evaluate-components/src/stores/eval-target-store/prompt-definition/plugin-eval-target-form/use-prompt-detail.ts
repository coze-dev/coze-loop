// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type prompt as promptDomain } from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';

import { useGlobalEvalConfig } from '@/stores/eval-global-config';

const usePromptDetail = ({
  promptId,
  version,
}: {
  promptId: string;
  version: string;
}) => {
  const [promptDetail, setPromptDetail] = useState<
    promptDomain.Prompt | undefined
  >(undefined);
  const [loading, setLoading] = useState(false);
  const { spaceID } = useSpace();
  const { customGetEvalTargetDetail: customFetchEvalTargetDetail } =
    useGlobalEvalConfig();

  const promptDetailService = useRequest(
    async () => {
      try {
        if (!promptId || !version) {
          setPromptDetail(undefined);
          return;
        }
        setLoading(true);

        // 有自定义请求数据，就用自定义的
        if (customFetchEvalTargetDetail) {
          const res = await customFetchEvalTargetDetail({
            promptID: promptId,
            version,
            spaceID,
          });
          setPromptDetail(res);
        } else {
          // 默认走开源
          const { prompt } = await StonePromptApi.GetPrompt({
            prompt_id: promptId,
            with_commit: true,
            commit_version: version,
            with_draft: true,
          });
          setPromptDetail(prompt);
        }

        setLoading(false);
        return prompt;
      } catch (e) {
        console.error('获取评测对象遇到错误', e);
      } finally {
        setLoading(false);
      }
    },
    {
      manual: true,
    },
  );

  useEffect(() => {
    promptDetailService.run();
  }, [version]);

  return {
    promptDetail,
    loading,
  };
};

export default usePromptDetail;
