// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useSearchParams } from 'react-router-dom';
import { useEffect, useRef } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { useEvalTargetDefinition } from '@cozeloop/evaluate-components';
import { useBreadcrumb } from '@cozeloop/base-hooks';

import { getCurrentTime } from '../tools';

// 创建实验页面初始化的一些业务逻辑
export const useExptPageInit = () => {
  const startTimeRef = useRef<number>(0);
  const newTimeRef = useRef<number>(0);
  const [searchParams] = useSearchParams();

  const { getEvalTargetDefinitionList, getEvalTargetDefinition } =
    useEvalTargetDefinition();

  const pluginEvaluatorList = getEvalTargetDefinitionList();

  useEffect(() => {
    // 进入创建页面时打点, 并记录开始时间
    sendEvent(EVENT_NAMES.cozeloop_experiment_enter_create_page);
    const currentTime = getCurrentTime();
    startTimeRef.current = currentTime;
    newTimeRef.current = currentTime;
    return () => {
      startTimeRef.current = 0;
      newTimeRef.current = 0;
    };
  }, []);

  // 面包屑
  useBreadcrumb({
    text: I18n.t('new_experiment'),
  });

  return {
    searchParams,
    startTimeRef,
    newTimeRef,
    pluginEvaluatorList,
    getEvalTargetDefinition,
  };
};
