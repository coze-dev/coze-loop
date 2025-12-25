// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { observationTraceAdapters } from '@cozeloop/observation-adapter';
import { useGlobalEvalConfig } from '@cozeloop/evaluate-components';
import { type Experiment } from '@cozeloop/api-schema/evaluation';

import { type ExperimentItem } from '@/types/experiment/experiment-detail';

const { TraceDetailPanel } = observationTraceAdapters;

const ONE_DAY_IN_SECONDS = 24 * 3600;
const MILLISECOND_FACTOR = '000';

export function TraceTargetTraceDetailPanel({
  item,
  traceID,
  spanID,
  experiment,
  onClose,
}: {
  item: ExperimentItem | undefined;
  traceID: Int64 | undefined;
  spanID: Int64 | undefined;
  experiment?: Experiment;
  onClose: () => void;
}) {
  const { traceOnlineEvalPlatformType } = useGlobalEvalConfig();

  const amendStartTime = useMemo(() => {
    if (!experiment?.start_time) {
      return undefined;
    }
    const startTimestamp = Number(experiment.start_time);
    if (Number.isNaN(startTimestamp)) {
      return undefined;
    }
    const result = startTimestamp - ONE_DAY_IN_SECONDS;
    return Number.isNaN(result) ? undefined : `${result}${MILLISECOND_FACTOR}`;
  }, [experiment?.start_time]);

  const amendEndTime = useMemo(() => {
    if (!experiment?.end_time) {
      return undefined;
    }
    const endTimestamp = Number(experiment.end_time);
    if (Number.isNaN(endTimestamp)) {
      return undefined;
    }
    return `${endTimestamp}${MILLISECOND_FACTOR}`;
  }, [experiment?.end_time]);

  const groupExt = item?.groupExt as Record<string, string> | undefined;

  return (
    <TraceDetailPanel
      // 在线评测不用传入platformType
      // platformType={undefined}
      // platformType={traceEvalTargetPlatformType as string}
      platformType={
        groupExt?.platform_type || (traceOnlineEvalPlatformType as string)
      }
      traceID={traceID?.toString() ?? ''}
      defaultSelectedSpanID={spanID?.toString()}
      // 开始时间取实验开始时间的前一天，结束时间取实验结束时间
      startTime={groupExt?.span_start_time || amendStartTime}
      endTime={groupExt?.span_end_time || amendEndTime}
      defaultActiveTabKey="feedback"
      visible={true}
      onClose={onClose}
    />
  );
}
