// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

{
  /* start_aigc */
}
import { useState, useCallback, useEffect, useMemo } from 'react';

import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  EvaluatorType,
  type EvaluatorContent,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Modal, Button, Space, type Form } from '@coze-arch/coze-design';

import {
  CodeEvaluatorLanguageFE,
  codeEvaluatorLanguageMapReverse,
} from '@/constants';
import {
  type IFormValues,
  type BaseFuncExecutorValue,
} from '@/components/evaluator-code/types';
import { BaseFuncExecutor } from '@/components/evaluator-code/editor-group/func-executor';
import { CodeValidationStatus } from '@/components/code-validation-status';
import { I18n } from '@cozeloop/i18n-adapter';

interface SubmitCheckModalProps {
  visible: boolean;
  onCancel: () => void;
  onSubmit: () => void;
  formRef: React.RefObject<Form<IFormValues>>;
  onChange: (value: BaseFuncExecutorValue) => void;
}

const SubmitCheckModal = ({
  visible,
  onCancel,
  onSubmit,
  formRef,
  onChange,
}: SubmitCheckModalProps) => {
  const { spaceID } = useSpace();
  const [localFuncExecutor, setLocalFuncExecutor] =
    useState<BaseFuncExecutorValue>({});
  const [validationResult, setValidationResult] = useState<{
    valid?: boolean;
    error_message?: string;
  } | null>(null);
  const [isChanged, setIsChanged] = useState(false);

  // 代码验证服务
  const validationService = useRequest(
    async () => {
      const { code, language = CodeEvaluatorLanguageFE.Javascript } =
        localFuncExecutor || {};

      const evaluatorContent: EvaluatorContent = {
        code_evaluator: {
          code_content: code,
          language_type: codeEvaluatorLanguageMapReverse[language],
        },
      };

      const res = await StoneEvaluationApi.ValidateEvaluator({
        workspace_id: spaceID,
        evaluator_content: evaluatorContent,
        evaluator_type: EvaluatorType.Code,
      });

      return res;
    },
    {
      manual: true,
      onSuccess: res => {
        setValidationResult(res);
        setIsChanged(false);
      },
      onError: error => {
        setValidationResult({
          valid: false,
          error_message: `验证失败: ${error.message}`,
        });
      },
    },
  );

  // 处理代码变更
  const handleCodeChange = (newValue: BaseFuncExecutorValue) => {
    setLocalFuncExecutor(newValue);
    onChange(newValue);
    setIsChanged(true);
  };

  // 处理检查按钮点击
  const handleCheck = useCallback(() => {
    validationService.run();
  }, [validationService]);

  // 处理创建按钮点击
  const handleCreate = useCallback(() => {
    onSubmit();
  }, [onSubmit]);

  const submitDisabled = useMemo(
    () => !validationResult?.valid || validationService.loading || isChanged,
    [validationResult, validationService.loading, isChanged],
  );

  // 重置验证结果当弹窗打开时
  useEffect(() => {
    // 重新打开弹窗时, 初始化
    if (visible) {
      const formVs = formRef.current?.formApi.getValues();
      const { funcExecutor } = formVs?.config || {};

      setValidationResult(null);
      setLocalFuncExecutor(funcExecutor as BaseFuncExecutorValue);
    }
  }, [visible]);

  return (
    <Modal
      title={I18n.t('evaluate_pre_submit_code_check')}
      visible={visible}
      onCancel={onCancel}
      width={800}
      hasScroll={false}
      footer={
        <Space spacing={8}>
          <Button
            color="primary"
            onClick={handleCheck}
            loading={validationService.loading}
          >
            {validationResult
              ? I18n.t('evaluate_recheck')
              : I18n.t('evaluate_check')}
          </Button>
          <Button
            type="primary"
            onClick={handleCreate}
            disabled={submitDisabled}
          >
            提交
          </Button>
        </Space>
      }
    >
      <div className="min-h-[560px] flex flex-col">
        {/* 代码编辑器部分 */}
        <div
          className="flex-1 rounded-lg"
          style={{ border: '1px solid var(--coz-stroke-primary)' }}
        >
          <BaseFuncExecutor
            value={localFuncExecutor}
            onChange={handleCodeChange}
            disabled={false}
            editorHeight={
              validationResult || validationService.loading ? '504px' : '600px'
            }
          />
        </div>

        <CodeValidationStatus
          validationResult={validationResult}
          loading={validationService.loading}
        />
      </div>
    </Modal>
  );
};

export default SubmitCheckModal;
{
  /* end_aigc */
}
