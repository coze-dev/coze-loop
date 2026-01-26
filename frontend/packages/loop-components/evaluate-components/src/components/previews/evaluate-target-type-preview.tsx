// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type EvalTargetType } from '@cozeloop/api-schema/evaluation';
import { Tag } from '@coze-arch/coze-design';

import { useEvalTargetDefinition } from '@/stores/eval-target-store';

export function EvaluateTargetTypePreview({
  type,
}: {
  type: EvalTargetType | undefined;
}) {
  const { getEvalTargetDefinition } = useEvalTargetDefinition();

  const typeOption = getEvalTargetDefinition(type ?? '');
  if (typeOption) {
    return (
      <Tag size="small" color={typeOption.targetInfo?.tagColor}>
        {typeOption.name}
      </Tag>
    );
  }
  return '-';
}
