// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  EvaluatorBoxType,
  type Evaluator,
} from '@cozeloop/api-schema/evaluation';
import { Skeleton } from '@coze-arch/coze-design';

import { EvaluatorDetailPlaceholder } from '../evaluator-detail-placeholder';
import { PresetLLMWhiteDetail } from './preset-llm-white-detail';
import { PresetLLMBlackDetail } from './preset-llm-black-detail';
import { InfoJump } from './info-jump';

const EvaluatorInfoDetail = ({
  evaluator,
  disabled = true,
  ...restProps
}: {
  evaluator: Evaluator;
  disabled?: boolean;
}) => {
  const { builtin, box_type } = evaluator ?? {};

  // 预置 - LLM 黑盒评估器
  if (builtin && box_type === EvaluatorBoxType.Black) {
    return (
      <PresetLLMBlackDetail
        evaluator={evaluator}
        disabled={disabled}
        {...restProps}
      />
    );
  }

  // 预置 - LLM 白盒评估器
  if (builtin && box_type === EvaluatorBoxType.White) {
    return <PresetLLMWhiteDetail evaluator={evaluator} {...restProps} />;
  }

  return (
    <Skeleton placeholder={EvaluatorDetailPlaceholder} loading={true} active />
  );
};

export { EvaluatorInfoDetail, PresetLLMBlackDetail, InfoJump };
