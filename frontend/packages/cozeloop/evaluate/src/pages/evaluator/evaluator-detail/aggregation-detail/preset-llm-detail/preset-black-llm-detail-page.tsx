// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { EvaluatorInfoDetail } from '@cozeloop/evaluate-components';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';

import { BlackDetailHeader } from '@/components/evaluator-ecosystem/black-detail-header';

interface PresetBlackLLMDetailProps {
  evaluator?: Evaluator;
  onClickDebugBtn?: (evaluator: Evaluator) => void;
}

export function PresetBlackLLMDetailPage({
  evaluator,
  onClickDebugBtn,
}: PresetBlackLLMDetailProps) {
  return (
    <div className="h-full w-full flex flex-col">
      <BlackDetailHeader
        evaluator={evaluator}
        onClickDebugBtn={onClickDebugBtn}
      />
      <div className="overflow-y-auto">
        <div className="w-full max-w-[800px] mx-auto mt-6">
          <EvaluatorInfoDetail evaluator={evaluator as Evaluator} />
        </div>
        <div className="h-[120px]" />
      </div>
    </div>
  );
}
