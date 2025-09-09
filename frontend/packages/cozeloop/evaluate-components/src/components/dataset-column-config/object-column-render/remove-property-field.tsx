// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type FieldTransformationConfig,
  FieldTransformationType,
} from '@cozeloop/api-schema/data';
import { Switch } from '@coze-arch/coze-design';

interface RemovePropertyFieldProps {
  value: FieldTransformationConfig[];
  onChange?: (value: FieldTransformationConfig[]) => void;
  disabled?: boolean;
  className?: string;
}

export const RemovePropertyField = ({
  value,
  onChange,
  disabled,
  className,
}: RemovePropertyFieldProps) => {
  const isCheck = value?.length > 0;
  return (
    <Switch
      checked={isCheck}
      className={className}
      size="small"
      disabled={disabled}
      onChange={checked => {
        if (checked) {
          onChange?.([
            {
              transType: FieldTransformationType.RemoveExtraFields,
              global: true,
            },
          ]);
        } else {
          onChange?.([]);
        }
      }}
    ></Switch>
  );
};
