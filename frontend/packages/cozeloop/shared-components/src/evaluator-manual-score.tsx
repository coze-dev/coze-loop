// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { Guard, GuardPoint } from '@cozeloop/guard';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Button, Form, type FormApi, Popover } from '@coze-arch/coze-design';

export interface FormValues {
  score?: number;
  reasoning?: string;
}

export interface CustomSubmitManualScore extends FormValues {
  evaluatorRecordID: string;
}
export interface EvaluatorManualScoreProps {
  spaceID: Int64;
  evaluatorRecordID: Int64;
  visible?: boolean;
  children?: React.ReactNode;
  onVisibleChange?: (visible: boolean) => void;
  onSuccess?: () => void;
  customSubmitManualScore?: (values: CustomSubmitManualScore) => Promise<void>;
}

export function EvaluatorManualScore({
  spaceID,
  evaluatorRecordID,
  children,
  onSuccess,
  visible,
  onVisibleChange,
  customSubmitManualScore,
}: EvaluatorManualScoreProps) {
  const formRef = useRef<FormApi<FormValues>>();

  const { loading, run: summitManualScore } = useRequest(
    async (values: FormValues) => {
      await StoneEvaluationApi.UpdateEvaluatorRecord({
        workspace_id: spaceID,
        evaluator_record_id: evaluatorRecordID,
        correction: {
          score: values.score,
          explain: values.reasoning,
        },
      });
      onVisibleChange?.(false);
      onSuccess?.();
    },
    { manual: true },
  );

  const handleSubmit = (values: FormValues) => {
    if (customSubmitManualScore) {
      customSubmitManualScore({
        ...values,
        evaluatorRecordID,
      });
      onVisibleChange?.(false);
      onSuccess?.();
      return;
    }
    summitManualScore(values);
  };

  const form = (
    <Form<FormValues>
      getFormApi={formApi => (formRef.current = formApi)}
      onSubmit={handleSubmit}
    >
      <Form.InputNumber
        field="score"
        label={I18n.t('rating')}
        placeholder={I18n.t('input_score_between_0_and_1')}
        className="w-full"
        step={0.1}
        rules={[
          { required: true, message: I18n.t('the_field_required') },
          {
            validator: (_rule, value) => value >= 0 && value <= 1,
            message: I18n.t('input_number_between_0_and_1'),
          },
          {
            validator: (_rule, value) => {
              const precision = String(value).split('.')[1];
              return !precision || precision.length <= 4;
            },
            message: I18n.t('cozeloop_open_evaluate_max_four_decimal_places'),
          },
        ]}
        autoComplete="off"
      />

      <Form.TextArea
        field="reasoning"
        label={I18n.t('reason')}
        placeholder={I18n.t('please_enter_the_reason')}
        maxCount={500}
        maxLength={500}
        autoComplete="off"
      />
    </Form>
  );

  const header = (
    <div className="text-xl font-bold">{I18n.t('manual_calibration')}</div>
  );
  const footer = (
    <div className="flex items-center justify-end gap-2">
      <Button color="primary" onClick={() => onVisibleChange?.(false)}>
        {I18n.t('cancel')}
      </Button>
      <Guard point={GuardPoint['eval.experiment.edit_result']}>
        <Button loading={loading} onClick={() => formRef.current?.submitForm()}>
          {I18n.t('update')}
        </Button>
      </Guard>
    </div>
  );

  const content = (
    <div className="flex flex-col gap-1 w-full">
      {header}
      {form}
      {footer}
    </div>
  );

  return (
    <Popover
      visible={visible}
      onVisibleChange={onVisibleChange}
      trigger="click"
      content={content}
      showArrow={true}
      style={{ width: 360 }}
    >
      {children}
    </Popover>
  );
}
