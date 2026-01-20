// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { EvaluatorInfoDetail } from '@cozeloop/evaluate-components';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';

import { WhiteDetailHeader } from '@/components/evaluator-ecosystem/white-llm-header';

interface PresetWhiteLLMDetailProps {
  evaluator?: Evaluator;
  onClickDebugBtn?: (evaluator: Evaluator) => void;
}

export function PresetWhiteLLMDetailPage({
  evaluator,
  onClickDebugBtn,
}: PresetWhiteLLMDetailProps) {
  return (
    <div className="h-full w-full flex flex-col">
      <WhiteDetailHeader
        evaluator={evaluator}
        onClickDebugBtn={onClickDebugBtn}
      />
      <div className="overflow-y-auto">
        <div className="w-full max-w-[800px] mx-auto mt-6">
          <EvaluatorInfoDetail evaluator={evaluator as Evaluator} />
        </div>
      </div>
      <div className="h-[120px]" />
    </div>
  );
}
