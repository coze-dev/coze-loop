// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Radio } from '@coze-arch/coze-design';

import { saftJsonParse } from '../../util';
import { type DatasetItemProps } from '../../type';

export const BoolDatasetItemEdit = ({
  fieldContent,
  onChange,
}: DatasetItemProps) => {
  const value = saftJsonParse(fieldContent?.text);
  const isTrue = value === true;
  const isFalse = value === false;
  const handleChange = (newValue: boolean) => {
    onChange?.({
      ...fieldContent,
      text: JSON.stringify(newValue),
    });
  };
  return (
    <div className="flex items-center gap-6">
      <Radio checked={isTrue} onChange={() => handleChange(true)}>
        true
      </Radio>
      <Radio checked={isFalse} onChange={() => handleChange(false)}>
        false
      </Radio>
    </div>
  );
};
