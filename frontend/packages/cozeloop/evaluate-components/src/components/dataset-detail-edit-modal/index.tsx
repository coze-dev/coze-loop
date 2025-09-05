import { useRef, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { Guard, GuardPoint } from '@cozeloop/guard';
import { EditIconButton } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  Form,
  type FormApi,
  FormInput,
  FormTextArea,
  Modal,
  Toast,
} from '@coze-arch/coze-design';

import { sourceNameRuleValidator } from '../../utils/source-name-rule';
interface FormValues {
  name?: string;
  description?: string;
}
export const DatasetDetailEditModal = ({
  datasetDetail,
  onSuccess,
  visible: visibleProp,
  showTrigger = true,
  onCancel,
}: {
  datasetDetail?: EvaluationSet;
  onSuccess: () => void;
  visible?: boolean;
  showTrigger?: boolean;
  onCancel?: () => void;
}) => {
  const { spaceID } = useSpace();
  const [visible, setVisible] = useState(visibleProp);
  const onSubmit = async (formValues: FormValues) => {
    await StoneEvaluationApi.UpdateEvaluationSet({
      name: formValues?.name,
      description: formValues?.description || '',
      evaluation_set_id: datasetDetail?.id as string,
      workspace_id: spaceID,
    });
    Toast.success(I18n.t('prompt_block_update_success'));
    onSuccess();
    setVisible(false);
  };
  const formRef = useRef<FormApi<FormValues>>();
  return (
    <>
      {showTrigger ? (
        <Guard point={GuardPoint['eval.dataset.edit_meta']}>
          <EditIconButton onClick={() => setVisible(true)} />
        </Guard>
      ) : null}
      <Modal
        visible={visible}
        onCancel={() => {
          setVisible(false);
          onCancel?.();
        }}
        title={I18n.t('edit_evaluation_set')}
        onOk={() => {
          formRef?.current?.submitForm();
        }}
        okText={I18n.t('save')}
        cancelText={I18n.t('cancel')}
      >
        <Form<FormValues>
          getFormApi={formApi => {
            formRef.current = formApi;
          }}
          onSubmit={onSubmit}
          initValues={{
            name: datasetDetail?.name,
            description: datasetDetail?.description,
          }}
          layout="vertical"
        >
          <FormInput
            field="name"
            label={I18n.t('evaluation_set_name')}
            maxLength={50}
            autoComplete="off"
            rules={[
              {
                required: true,
                message: I18n.t('enter_evaluation_name'),
              },
              {
                validator: sourceNameRuleValidator,
              },
            ]}
          />
          <FormTextArea
            field="description"
            label={I18n.t('evaluation_set_description')}
            maxCount={200}
            maxLength={200}
          />
        </Form>
      </Modal>
    </>
  );
};
