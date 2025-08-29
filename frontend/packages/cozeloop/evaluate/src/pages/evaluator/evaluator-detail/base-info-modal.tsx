import { useEffect, useRef } from 'react';

import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { sourceNameRuleValidator } from '@cozeloop/evaluate-components';
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
      title="编辑评估器"
      visible={visible}
      cancelText="取消"
      onCancel={onCancel}
      okText="提交"
      okButtonProps={{
        loading: saveService.loading,
      }}
      onOk={saveService.run}
    >
      <Form ref={formRef}>
        <FormInput
          label="名称"
          field="name"
          placeholder={'请输入名称'}
          required
          maxLength={50}
          trigger="blur"
          rules={[
            { required: true, message: '请输入名称' },
            { validator: sourceNameRuleValidator },
            {
              asyncValidator: async (_, value: string) => {
                if (value && value !== evaluator?.name) {
                  const { pass } = await StoneEvaluationApi.CheckEvaluatorName({
                    workspace_id: spaceID,
                    name: value,
                  });
                  if (!pass) {
                    throw new Error('名称已存在');
                  }
                }
              },
            },
          ]}
        />
        <FormTextArea
          label="描述"
          field="description"
          placeholder={'请输入描述'}
          maxCount={200}
          maxLength={200}
        />
      </Form>
    </Modal>
  );
}
