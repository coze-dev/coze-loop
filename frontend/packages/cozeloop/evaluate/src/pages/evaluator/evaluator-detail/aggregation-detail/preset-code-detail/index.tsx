// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useParams } from 'react-router-dom';
import { useEffect, useState } from 'react';

import { EvaluatorDetailPlaceholder } from '@cozeloop/evaluate-components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  EvaluatorBoxType,
  type Evaluator,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Skeleton } from '@coze-arch/coze-design';

import { PresetBlackCodeDetail } from './preset-black-code-detail';

export default function PresetCodeDetail() {
  const { evaluatorId } = useParams<{ evaluatorId: string }>();
  const { spaceID } = useSpace();
  const [evaluator, setEvaluator] = useState<Evaluator | undefined>();

  useEffect(() => {
    if (evaluatorId && spaceID) {
      StoneEvaluationApi.GetEvaluator({
        evaluator_id: evaluatorId,
        workspace_id: spaceID,
      })
        .then(res => {
          setEvaluator(res.evaluator);
        })
        .catch(err => {
          console.error('Failed to fetch evaluator:', err);
        });
    }
  }, [evaluatorId, spaceID]);

  const isBlackBox = evaluator?.box_type === EvaluatorBoxType.Black;

  if (!evaluator) {
    return (
      <Skeleton
        placeholder={EvaluatorDetailPlaceholder}
        loading={true}
        active
      />
    );
  }

  return (
    <div className="h-full w-full">
      {isBlackBox ? (
        <PresetBlackCodeDetail evaluator={evaluator} />
      ) : (
        <div>Not a black box code evaluator.</div>
      )}
    </div>
  );
}
