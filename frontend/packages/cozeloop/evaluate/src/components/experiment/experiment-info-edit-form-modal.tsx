import { useRef } from 'react';

import { useRequest } from 'ahooks';
import { sourceNameRuleValidator } from '@cozeloop/evaluate-components';
import { type Experiment } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  Form,
  type FormApi,
  Modal,
  FormTextArea,
  FormInput,
} from '@coze-arch/coze-design';

interface FormValues {
  name?: string;
  desc?: string;
}

export default function ExperimentInfoEditFormModal({
  spaceID,
  experiment,
  visible,
  onSuccess,
  onClose,
}: {
  spaceID: string;
  experiment: Experiment | undefined;
  visible?: boolean;
  onClose?: () => void;
  onSuccess?: () => void;
}) {
  const formRef = useRef<FormApi<FormValues>>();

  const { loading, runAsync } = useRequest(
    async (values: FormValues) => {
      await StoneEvaluationApi.UpdateExperiment({
        ...values,
        workspace_id: spaceID,
        expt_id: experiment?.id ?? '',
      });
    },
    { manual: true },
  );

  const handleSubmit = async (values: FormValues) => {
    await runAsync(values);
    onSuccess?.();
    onClose?.();
  };

  const form = (
    <Form<FormValues>
      getFormApi={formApi => (formRef.current = formApi)}
      initValues={{ name: experiment?.name, desc: experiment?.desc }}
      onSubmit={handleSubmit}
    >
      <FormInput
        field="name"
        label="实验名称"
        placeholder="请输入"
        maxLength={50}
        rules={[
          { required: true, message: '该字段必填' },
          { validator: sourceNameRuleValidator },
        ]}
      />
      <FormTextArea
        field="desc"
        label="实验描述"
        placeholder="请输入"
        maxCount={200}
        maxLength={200}
      />
    </Form>
  );

  return (
    <Modal
      visible={visible}
      title="编辑实验"
      okText="确定"
      cancelText="取消"
      okButtonProps={{ loading }}
      onOk={() => formRef.current?.submitForm()}
      onCancel={onClose}
      width={600}
    >
      {form}
    </Modal>
  );
}
