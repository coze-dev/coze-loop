// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState, useEffect, useRef } from 'react';

import { useDocumentVisibility } from 'ahooks';

import { BIZ_EVENTS } from '@/shared/constants';
import { useTraceStore } from '@/features/trace-list/stores/trace';
import { useConfigContext } from '@/config-provider';

export function usePageStay() {
  const visibleState = useDocumentVisibility();
  const stayTimeRef = useRef<number>(0);
  const startTimeRef = useRef<number | null>(null);
  const [stayTime, setStayTime] = useState<number>(0);
  const { customParams } = useTraceStore();
  const { sendEvent } = useConfigContext();

  useEffect(() => {
    if (visibleState !== 'hidden') {
      startTimeRef.current = Date.now();
    } else if (startTimeRef.current) {
      stayTimeRef.current = Date.now() - startTimeRef.current;
      startTimeRef.current = null;
      setStayTime(stayTimeRef.current);

      sendEvent?.(BIZ_EVENTS.cozeloop_observation_trace_page_stay, {
        duration: stayTimeRef.current,
        space_id: customParams?.spaceID ?? '',
        space_name: customParams?.spaceName ?? '',
      });
    }

    return () => {
      if (startTimeRef.current) {
        stayTimeRef.current = Date.now() - startTimeRef.current;
        startTimeRef.current = null;
        setStayTime(stayTimeRef.current);

        sendEvent?.(BIZ_EVENTS.cozeloop_observation_trace_page_stay, {
          duration: stayTimeRef.current,
          space_id: customParams?.spaceID ?? '',
          space_name: customParams?.spaceName ?? '',
        });
      }
    };
  }, [visibleState]);

  return stayTime;
}
