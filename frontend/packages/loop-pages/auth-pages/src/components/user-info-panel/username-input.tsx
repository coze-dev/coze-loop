// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { Form, Input, type InputProps } from '@coze-arch/coze-design';

import s from './username-input.module.less';

export const USER_NAME_MAX_LEN = 24;

interface InputWithCountProps extends InputProps {
  // 设置字数限制并显示字数统计
  getValueLength?: (value?: InputProps['value'] | string) => number;
}

export interface UsernameInputProps
  extends Omit<InputWithCountProps, 'prefix' | 'maxLength' | 'validateStatus'> {
  scene?: 'modal' | 'page';
  errorMessage?: string;
}

export function UsernameInput({
  className,
  scene = 'page',
  errorMessage,
  ...props
}: UsernameInputProps) {
  const isError = Boolean(errorMessage);
  return (
    <>
      <Input
        className={classNames(
          s.input,
          isError && s.error,
          scene === 'modal' ? s.modal : s.page,
          className,
        )}
        validateStatus={isError ? 'error' : 'default'}
        prefix="@"
        maxLength={USER_NAME_MAX_LEN}
        {...props}
      />
      <Form.ErrorMessage error={errorMessage} />
    </>
  );
}
