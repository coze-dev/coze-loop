import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { FormInput, FormTextArea } from '@coze-arch/coze-design';

import { type BaseInfoValues } from '@/types/experiment/experiment-create';

import { baseInfoValidators } from '../validators/base-info';

export interface BaseInfoFormRef {
  validate?: () => Promise<BaseInfoValues>;
}

export interface BaseInfoFormProps {
  initialValues?: BaseInfoValues;
  onChange?: (values: Partial<BaseInfoValues>) => void;
}

export const BaseInfoForm = () => {
  const { spaceID } = useSpace();

  return (
    <>
      <FormInput
        field="name"
        label="名称"
        placeholder="请输入名称"
        required
        maxLength={50}
        trigger="blur"
        rules={baseInfoValidators.name.map(rule =>
          rule.asyncValidator
            ? {
                ...rule,
                asyncValidator: (_, value) =>
                  rule.asyncValidator(_, value, spaceID),
              }
            : rule,
        )}
      />
      <FormTextArea
        label="描述"
        field="desc"
        placeholder="请输入描述"
        maxCount={200}
        maxLength={200}
        rules={baseInfoValidators.desc}
      />
    </>
  );
};
