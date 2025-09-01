import { useRef } from 'react';

import { useRequest } from 'ahooks';
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
        label="评分"
        placeholder="请输入0-1的分值"
        className="w-full"
        step={0.1}
        rules={[
          { required: true, message: '该字段必填' },
          {
            validator: (_rule, value) => value >= 0 && value <= 1,
            message: '请输入0～1区间内的数字',
          },
          {
            validator: (_rule, value) => {
              const precision = String(value).split('.')[1];
              return !precision || precision.length <= 4;
            },
            message: '最多保留小数点后四位',
          },
        ]}
        autoComplete="off"
      />
      <Form.TextArea
        field="reasoning"
        label="原因"
        placeholder="请输入原因"
        maxCount={500}
        maxLength={500}
        autoComplete="off"
      />
    </Form>
  );
  const header = <div className="text-xl font-bold">人工校准</div>;
  const footer = (
    <div className="flex items-center justify-end gap-2">
      <Button color="primary" onClick={() => onVisibleChange?.(false)}>
        取消
      </Button>
      <Guard point={GuardPoint['eval.experiment.edit_result']}>
        <Button loading={loading} onClick={() => formRef.current?.submitForm()}>
          更新
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
