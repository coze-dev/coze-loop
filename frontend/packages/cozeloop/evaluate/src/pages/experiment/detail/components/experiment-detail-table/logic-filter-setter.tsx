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
        placeholder={`${I18n.t('please_enter_max_limit_length', { limitLength })}`}
        {...rest}
        onChange={(val, e) => {
          let newVal = val;
          if (val && val.length > limitLength) {
            newVal = val?.slice(0, limitLength);
            Toast.warning(
              `${I18n.t('input_content_limited_truncated', { limitLength })}`,
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
        placeholder={`${I18n.t('please_enter_max_limit_length', { limitLength })}`}
        {...rest}
        onChange={(val, e) => {
          let newVal = val;
          if (val && val.length > limitLength) {
            newVal = val?.slice(0, limitLength);
            Toast.warning(
              `${I18n.t('input_content_limited_truncated', { limitLength })}`,
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
            `${I18n.t('id_invalid_must_be_limit_length', { limitLength })}`,
          );
        }
      }}
    />
  );
}
