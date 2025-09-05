import { useEffect, useRef } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { sourceNameRuleValidator } from '@cozeloop/evaluate-components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { EvaluatorType, type Evaluator } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Form, FormInput, FormTextArea, Modal } from '@coze-arch/coze-design';

export type BaseInfo = Pick<Evaluator, 'name' | 'description'>;

export function BaseInfoModal({
  evaluator,
  visible,
  onCancel,
  onSubmit,
}: {
  visible: boolean;
  onCancel: () => void;
  onSubmit: (values: BaseInfo) => void;
  evaluator?: Evaluator;
}) {
  const { spaceID } = useSpace();
  const formRef = useRef<Form<BaseInfo>>(null);

  const saveService = useRequest(
    async () => {
      const values = await formRef.current?.formApi
        ?.validate()
        .catch(e => console.warn(e));
      const newMeta = {
        name: values?.name || '',
        description: values?.description || '',
      };
      if (values) {
        await StoneEvaluationApi.UpdateEvaluator({
          workspace_id: evaluator?.workspace_id || '',
          evaluator_id: evaluator?.evaluator_id || '',
          evaluator_type: evaluator?.evaluator_type || EvaluatorType.Prompt,
          ...newMeta,
        });

        onSubmit(newMeta);
        onCancel();
      }
    },
    {
      manual: true,
    },
  );

  useEffect(() => {
    if (visible) {
      formRef.current?.formApi?.setValues({
        name: evaluator?.name,
        description: evaluator?.description,
      });
    }
  }, [visible]);

  return (
    <Modal
      width={600}
      title={I18n.t('edit_evaluator')}
      visible={visible}
      cancelText={I18n.t('cancel')}
      onCancel={onCancel}
      okText={I18n.t('submit')}
      okButtonProps={{
        loading: saveService.loading,
      }}
      onOk={saveService.run}
    >
      <Form ref={formRef}>
        <FormInput
          label={I18n.t('name')}
          field="name"
          placeholder={I18n.t('please_input_name')}
          required
          maxLength={50}
          trigger="blur"
          rules={[
            { required: true, message: I18n.t('please_input_name') },
            { validator: sourceNameRuleValidator },
            {
              asyncValidator: async (_, value: string) => {
                if (value && value !== evaluator?.name) {
                  const { pass } = await StoneEvaluationApi.CheckEvaluatorName({
                    workspace_id: spaceID,
                    name: value,
                  });
                  if (!pass) {
                    throw new Error(I18n.t('name_already_exists'));
                  }
                }
              },
            },
          ]}
        />
        <FormTextArea
          label={I18n.t('description')}
          field="description"
          placeholder={I18n.t('please_input_description')}
          maxCount={200}
          maxLength={200}
        />
      </Form>
    </Modal>
  );
}
