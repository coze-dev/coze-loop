// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { type FocusEvent } from 'react';

import {
  ariaDescribedByIds,
  type FormContextType,
  type GenericObjectType,
  type RJSFSchema,
  type StrictRJSFSchema,
  type WidgetProps,
} from '@rjsf/utils';
import { TextArea, type TextAreaProps } from '@coze-arch/coze-design';

const INPUT_STYLE = {
  width: '100%',
};

/** The `TextareaWidget` is a widget for rendering input fields as textarea.
 *
 * @param props - The `WidgetProps` for this component
 */
export default function TextareaWidget<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>({
  disabled,
  formContext,
  id,
  onBlur,
  onChange,
  onFocus,
  options,
  placeholder,
  readonly,
  value,
}: WidgetProps<T, S, F>) {
  const { readonlyAsDisabled = true } = formContext as GenericObjectType;

  const handleChange: TextAreaProps['onChange'] = nextValue =>
    onChange(nextValue === '' ? options.emptyValue : nextValue);

  const handleBlur = ({ target }: FocusEvent<HTMLTextAreaElement>) =>
    onBlur(id, target && target.value);

  const handleFocus = ({ target }: FocusEvent<HTMLTextAreaElement>) =>
    onFocus(id, target && target.value);

  return (
    <TextArea
      disabled={disabled || (readonlyAsDisabled && readonly)}
      id={id}
      name={id}
      onBlur={!readonly ? handleBlur : undefined}
      onChange={!readonly ? handleChange : undefined}
      onFocus={!readonly ? handleFocus : undefined}
      placeholder={placeholder}
      rows={options.rows || 4}
      style={INPUT_STYLE}
      value={value}
      aria-describedby={ariaDescribedByIds<T>(id)}
    />
  );
}
