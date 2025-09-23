// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
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
        label={I18n.t('experiment_name')}
        placeholder={I18n.t('please_input', { field: '' })}
        maxLength={50}
        rules={[
          { required: true, message: I18n.t('the_field_required') },
          { validator: sourceNameRuleValidator },
        ]}
      />
      <FormTextArea
        field="desc"
        label={I18n.t('experiment_description')}
        placeholder={I18n.t('please_input', { field: '' })}
        maxCount={200}
        maxLength={200}
      />
    </Form>
  );

  return (
    <Modal
      visible={visible}
      title={I18n.t('edit_experiment')}
      okText={I18n.t('confirm')}
      cancelText={I18n.t('cancel')}
      okButtonProps={{ loading }}
      onOk={() => formRef.current?.submitForm()}
      onCancel={onClose}
      width={600}
    >
      {form}
    </Modal>
  );
}
