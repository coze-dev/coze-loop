// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Input } from '@coze-arch/coze-design';

import { type DatasetItemProps } from '../../type';

export const IntegerDatasetItemEdit = ({
  fieldContent,
  onChange,
}: DatasetItemProps) => (
  <>
    <Input
      placeholder={I18n.t('cozeloop_open_evaluate_enter_integer')}
      className="rounded-[6px]"
      value={fieldContent?.text}
      onChange={value => {
        onChange?.({
          ...fieldContent,
          text: value,
        });
      }}
    />
  </>
);
