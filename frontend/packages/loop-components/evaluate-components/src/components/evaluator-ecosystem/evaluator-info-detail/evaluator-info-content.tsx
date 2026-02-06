// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Evaluator } from '@cozeloop/api-schema/evaluation';

export const EvaluatorInfoContent = ({
  evaluator,
}: {
  evaluator: Evaluator;
}) => (
  <div>
    <div className="text-[13px] mt-2 mb-2 font-normal text-[rgba(32,41,69,0.62)]">
      {evaluator.description}
    </div>
  </div>
);
