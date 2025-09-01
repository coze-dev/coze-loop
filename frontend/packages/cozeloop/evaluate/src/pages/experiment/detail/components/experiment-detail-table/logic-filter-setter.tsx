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
        placeholder={`请输入，最长${limitLength}字符`}
        {...rest}
        onChange={(val, e) => {
          let newVal = val;
          if (val && val.length > limitLength) {
            newVal = val?.slice(0, limitLength);
            Toast.warning(
              `输入内容最长${limitLength}字符，超出部分已被自动截断`,
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
        placeholder={`请输入，最长${limitLength}字符`}
        {...rest}
        onChange={(val, e) => {
          let newVal = val;
          if (val && val.length > limitLength) {
            newVal = val?.slice(0, limitLength);
            Toast.warning(
              `输入内容最长${limitLength}字符，超出部分已被自动截断`,
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
      placeholder={`请输入${limitLength}位ID`}
      {...rest}
      onChange={(val, e) => {
        onChange?.(val, e);
      }}
      onBlur={e => {
        const val = e.target.value;
        if (val && val.length !== limitLength) {
          Toast.warning(`ID不合法，必须是${limitLength}位`);
        }
      }}
    />
  );
}
