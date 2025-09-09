// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { TraceDetailPanel } from '@cozeloop/observation-component-adapter';
import { useGlobalEvalConfig } from '@cozeloop/evaluate-components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type Experiment } from '@cozeloop/api-schema/evaluation';

export function TraceTargetTraceDetailPanel({
  traceID,
  spanID,
  experiment,
  onClose,
}: {
  traceID: Int64 | undefined;
  spanID: Int64 | undefined;
  experiment?: Experiment;
  onClose: () => void;
}) {
  const { spaceID, space } = useSpace();
  const { traceOnlineEvalPlatformType } = useGlobalEvalConfig();
  return (
    <TraceDetailPanel
      spaceID={spaceID}
      spaceName={space?.name ?? ''}
      searchType="trace_id"
      // 在线评测不用传入platformType
      // platformType={undefined}
      // platformType={traceEvalTargetPlatformType as string}
      platformType={traceOnlineEvalPlatformType as string}
      id={traceID?.toString() ?? ''}
      defaultSpanID={spanID?.toString()}
      // 开始时间取实验开始时间的前一天，结束时间取实验结束时间
      startTime={
        experiment?.start_time
          ? `${Number(experiment.start_time) - 24 * 3600}000`
          : undefined
      }
      endTime={experiment?.end_time ? `${experiment.end_time}000` : undefined}
      moduleName="evaluation"
      defaultActiveTabKey="feedback"
      visible={true}
      onClose={onClose}
    />
  );
}
