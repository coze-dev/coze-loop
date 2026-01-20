// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import {
  Input,
  TextArea,
  type TextAreaProps,
  Toast,
  type InputProps,
} from '@coze-arch/coze-design';

export function InputLimitLengthHOC(limitLength: number) {
  return (inputProps: InputProps) => {
    const { onChange, ...rest } = inputProps;
    return (
      <Input
        placeholder={`${I18n.t('evaluate_please_input_max_limitLength_chars', { limitLength })}`}
        {...rest}
        onChange={(val, e) => {
          let newVal = val;
          if (val && val.length > limitLength) {
            newVal = val?.slice(0, limitLength);
            Toast.warning(
              `${I18n.t('evaluate_input_content_max_limitLength_truncated', { limitLength })}`,
            );
          }
          onChange?.(newVal, e);
        }}
      />
    );
  };
}

export function TextAreaLimitLengthHOC(limitLength: number) {
  return (textareaProps: TextAreaProps) => {
    const { onChange, ...rest } = textareaProps;
    return (
      <TextArea
        rows={1}
        placeholder={`${I18n.t('evaluate_please_input_max_limitLength_chars', { limitLength })}`}
        {...rest}
        onChange={(val, e) => {
          let newVal = val;
          if (val && val.length > limitLength) {
            newVal = val?.slice(0, limitLength);
            Toast.warning(
              `${I18n.t('evaluate_input_content_max_limitLength_truncated', { limitLength })}`,
            );
          }
          onChange?.(newVal, e);
        }}
      />
    );
  };
}

export function IDSearchInput(inputProps: InputProps) {
  const { onChange, ...rest } = inputProps;
  const limitLength = 19;
  return (
    <Input
      placeholder={`${I18n.t('please_enter_id_limit_length', { limitLength })}`}
      {...rest}
      onChange={(val, e) => {
        onChange?.(val, e);
      }}
      onBlur={e => {
        const val = e.target.value;
        if (val && val.length !== limitLength) {
          Toast.warning(
            `${I18n.t('evaluate_invalid_id_must_be_limitLength', { limitLength })}`,
          );
        }
      }}
    />
  );
}
