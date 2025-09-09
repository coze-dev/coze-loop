// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Input } from '@coze-arch/coze-design';

import { type DatasetItemProps } from '../../type';
export const FloatDatasetItemEdit = ({
  fieldContent,
  onChange,
}: DatasetItemProps) => (
  <Input
    placeholder={I18n.t('input_float_with_precision')}
    className="rounded-[6px]"
    value={fieldContent?.text}
    onChange={value => {
      onChange?.({
        ...fieldContent,
        text: value,
      });
    }}
  />
);
