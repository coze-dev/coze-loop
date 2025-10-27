// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import {
  AggregatorType,
  type Evaluator,
  type EvaluatorAggregateResult,
} from '@cozeloop/api-schema/evaluation';
import { Popover } from '@coze-arch/coze-design';

import { type CustomSubmitManualScore } from '@/components/experiments/evaluator-manual-score';

import { EvaluatorNameScoreTag } from '../../components/experiments/evaluator-name-score';
import { AutoOverflowList } from '../../components/common';

// 实验评估器的聚合结果
function ExperimentEvaluatorAggregatorScore({
  evaluators,
  spaceID,
  evaluatorAggregateResult = [],
  customSubmitManualScore,
}: {
  evaluators: Evaluator[] | undefined;
  spaceID: Int64;
  // 评估器聚合结果
  evaluatorAggregateResult: EvaluatorAggregateResult[] | undefined;
  customSubmitManualScore?: (values: CustomSubmitManualScore) => Promise<void>;
}) {
  const evaluatorDataMap = useMemo(
    () =>
      evaluatorAggregateResult.reduce(
        (acc, cur) => {
          acc[cur.evaluator_version_id] = cur;
          return acc;
        },
        {} as unknown as Record<Int64, EvaluatorAggregateResult>,
      ),
    [evaluatorAggregateResult],
  );
  return (
    <AutoOverflowList<Evaluator>
      itemKey={'current_version.id'}
      items={evaluators ?? []}
      itemRender={({ item: evaluator, inOverflowPopover }) => {
        const evaluatorAggregateData =
          evaluatorDataMap[evaluator.current_version?.id ?? ''];
        // 平均分
        const averageScore = evaluatorAggregateData?.aggregator_results?.find(
          e => e.aggregator_type === AggregatorType.Average,
        )?.data?.value;

        const { name, current_version, evaluator_id } = evaluator ?? {};
        const { version, id: versionId } = current_version ?? {};
        const evaluatorResult = { score: averageScore };
        if (inOverflowPopover) {
          return (
            <EvaluatorNameScoreTag
              name={name}
              evaluatorResult={evaluatorResult}
              version={version}
              evaluatorID={evaluator_id}
              evaluatorVersionID={versionId}
              spaceID={spaceID}
              enableLinkJump={true}
              defaultShowAction={true}
              border={false}
              customSubmitManualScore={customSubmitManualScore}
            />
          );
        }
        return (
          <Popover
            position="top"
            stopPropagation
            content={
              <div className="p-1" style={{ color: 'var(--coz-fg-secondary)' }}>
                <EvaluatorNameScoreTag
                  name={name}
                  evaluatorResult={evaluatorResult}
                  version={version}
                  evaluatorID={evaluator_id}
                  evaluatorVersionID={versionId}
                  spaceID={spaceID}
                  enableLinkJump={true}
                  defaultShowAction={true}
                  border={false}
                  customSubmitManualScore={customSubmitManualScore}
                />
              </div>
            }
          >
            <div onClick={e => e.stopPropagation()}>
              <EvaluatorNameScoreTag
                name={name}
                evaluatorResult={evaluatorResult}
                version={version}
                border={true}
                showVersion={true}
                evaluatorID={evaluator_id}
                evaluatorVersionID={versionId}
                spaceID={spaceID}
                customSubmitManualScore={customSubmitManualScore}
              />
            </div>
          </Popover>
        );
      }}
    />
  );
}

export default ExperimentEvaluatorAggregatorScore;
