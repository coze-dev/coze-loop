// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import {
  getTimePickerPresets,
  TRACE_FREE_PRESETS_LIST,
} from '@/features/trace-list/constants/time';

export function useTraceTimeRangeOptions() {
  const tracePresetsList = TRACE_FREE_PRESETS_LIST;

  const options = useMemo(() => {
    const ranges = tracePresetsList.map(item => ({
      value: item,
    }));
    return ranges.map(preset => ({
      label: (
        <div className="pr-1">{getTimePickerPresets()[preset.value].text}</div>
      ),
      value: preset.value,
    }));
  }, [tracePresetsList]);

  return options;
}
