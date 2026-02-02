// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState, useCallback, useRef, useEffect } from 'react';

import { omit } from 'lodash-es';
import { type SentinelFormRef } from '@cozeloop/components';

import type { CreateExperimentValues } from '@/types/experiment/experiment-create';

import { useInitialData } from './use-initial-data';

export interface UseFormDataOptions {
  spaceID: string;
  copyExperimentID?: string;
  evaluationSetID?: string;
  evaluationSetVersionID?: string;
  initialData: CreateExperimentValues;
}

export const useFormData = ({
  initialData,
  spaceID,
  copyExperimentID,
  evaluationSetID,
  evaluationSetVersionID,
}: UseFormDataOptions) => {
  // 非表单数据, 但包含表单数据, 用于渲染, 能力上与表单数据隔离
  const [formData, setFormData] = useState<CreateExperimentValues>(
    (initialData || {}) as CreateExperimentValues,
  );

  const formRef = useRef<SentinelFormRef<CreateExperimentValues>>(null);

  const [isDirty, setIsDirty] = useState(false);

  const { loading, initValue } = useInitialData({
    spaceID,
    copyExperimentID,
    evaluationSetID,
    evaluationSetVersionID,
    setValue: (newData: Partial<CreateExperimentValues>) => {
      // 渲染数据存放全量
      setFormData(newData as CreateExperimentValues);
      // 仅获取表单所需的字段
      const formValues = omit(newData, [
        'evaluationSetVersionDetail',
        'evalTargetVersionDetail',
        'evaluationSetDetail',
      ]);
      formRef.current?.formApi?.setValues(formValues, {
        isOverride: true,
      });
    },
  });

  useEffect(() => {
    setFormData({ ...formData, workspace_id: spaceID });
  }, [spaceID]);

  const updateFormData = (newData: Partial<CreateExperimentValues>) => {
    setFormData(prev => {
      const updated = { ...prev, ...newData };
      return updated;
    });
    setIsDirty(true);
  };

  const resetFormData = useCallback(() => {
    setFormData(initialData);
    setIsDirty(false);
  }, [initialData]);

  return {
    initLoading: loading,
    formData,
    setFormData,
    isDirty,
    updateFormData,
    resetFormData,
    formRef,
    initValue,
  };
};
