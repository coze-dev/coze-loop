// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { Tag } from '@coze-arch/coze-design';

import { formatTime } from '@/shared/utils/time';
import { useCustomComponents } from '@/features/trace-list/hooks/use-custom-components';

import { CustomTableTooltip } from '../table-cell-text';

interface TimeTagProps {
  latency?: number | string; // in seconds
}

const FAST_LATENCY = 50000;
const SLOW_LATENCY = 600000;

export const LatencyTag: React.FC<TimeTagProps> = ({ latency }) => {
  const { LatencyIcon } = useCustomComponents();
  const [bgClassName, textClassName] = useMemo(() => {
    if (Number(latency) > SLOW_LATENCY) {
      return ['!bg-[rgba(255,235,233,1)]', '!text-[rgba(208,41,47,1)]'];
    } else if (Number(latency) > FAST_LATENCY) {
      return ['!bg-[rgba(251,238,225,1)]', '!text-[rgba(160,95,1,1)]'];
    } else {
      return ['!bg-[rgba(230,247,237,1)]', '!text-[rgba(0,129,92,1)]'];
    }
  }, [latency]);

  if (!latency) {
    return (
      <div className="flex w-full items-center justify-start pr-[6px]">-</div>
    );
  }

  return (
    <Tag
      size="small"
      className={`m-w-full border-box ${bgClassName}`}
      prefixIcon={<LatencyIcon className={`${textClassName} !w-3 !h-3`} />}
    >
      <CustomTableTooltip textClassName={textClassName}>
        {formatTime(Number(latency))}
      </CustomTableTooltip>
    </Tag>
  );
};
