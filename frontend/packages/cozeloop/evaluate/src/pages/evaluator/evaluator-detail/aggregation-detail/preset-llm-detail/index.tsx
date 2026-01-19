// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { EvaluatorBoxType } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

import { PresetWhiteLLMDetailPage } from './preset-white-llm-detail-page';
import { PresetBlackLLMDetailPage } from './preset-black-llm-detail-page';

// 预置评估器, 版本从query获取, 区分黑白盒
const PresetLLMDetail = () => {
  const { spaceID } = useSpace();

  // 初始化拉取预置评估器数据
  const service = useRequest(async () => {
    const queryString = window.location.search;
    const urlParams = new URLSearchParams(queryString);
    const versionID = urlParams.get('version');

    return StoneEvaluationApi.GetEvaluatorVersion({
      workspace_id: spaceID,
      evaluator_version_id: versionID || '',
      builtin: true,
    }).then(res => res.evaluator);
  });
  const evaluator = service.data;

  const handleClickDebugBtn = () => {
    sendEvent(EVENT_NAMES.cozeloop_pre_evaluator_test, {
      pre_evaluator_card_name: evaluator?.name,
    });
  };

  // 预置 - llm 黑盒
  if (evaluator?.box_type === EvaluatorBoxType.Black) {
    return (
      <PresetBlackLLMDetailPage
        evaluator={evaluator}
        onClickDebugBtn={handleClickDebugBtn}
      />
    );
  }

  // 预置 - llm 白盒
  return (
    <PresetWhiteLLMDetailPage
      evaluator={evaluator}
      onClickDebugBtn={handleClickDebugBtn}
    />
  );
};

export { PresetLLMDetail };
