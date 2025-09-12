// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
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
        label={I18n.t('name')}
        placeholder={I18n.t('please_input_name')}
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
        label={I18n.t('description')}
        field="desc"
        placeholder={I18n.t('please_input_description')}
        maxCount={200}
        maxLength={200}
        rules={baseInfoValidators.desc}
      />
    </>
  );
};
