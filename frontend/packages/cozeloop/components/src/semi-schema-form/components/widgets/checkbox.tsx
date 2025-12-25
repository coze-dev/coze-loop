// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { type FocusEvent } from 'react';

import {
  ariaDescribedByIds,
  // labelValue,
  type FormContextType,
  type RJSFSchema,
  type StrictRJSFSchema,
  type WidgetProps,
  type GenericObjectType,
} from '@rjsf/utils';
import { Checkbox, type CheckboxProps } from '@coze-arch/coze-design';

/** The `CheckBoxWidget` is a widget for rendering boolean properties.
 *  It is typically used to represent a boolean.
 *
 * @param props - The `WidgetProps` for this component
 */
export default function CheckboxWidget<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>(props: WidgetProps<T, S, F>) {
  const {
    autofocus,
    disabled,
    formContext,
    id,
    // label,
    // hideLabel,
    onBlur,
    onChange,
    onFocus,
    readonly,
    value,
  } = props;
  const { readonlyAsDisabled = true } = formContext as GenericObjectType;

  const handleChange: CheckboxProps['onChange'] = ({ target }) =>
    onChange(target.checked);

  const handleBlur = ({ target }: FocusEvent<HTMLInputElement>) =>
    onBlur(id, target && target.checked);

  const handleFocus = ({ target }: FocusEvent<HTMLInputElement>) =>
    onFocus(id, target && target.checked);

  // Antd's typescript definitions do not contain the following props that are actually necessary and, if provided,
  // they are used, so hacking them in via by spreading `extraProps` on the component to avoid typescript errors
  const extraProps = {
    onBlur: !readonly ? handleBlur : undefined,
    onFocus: !readonly ? handleFocus : undefined,
  };
  return (
    <Checkbox
      autoFocus={autofocus}
      checked={typeof value === 'undefined' ? false : value}
      disabled={disabled || (readonlyAsDisabled && readonly)}
      id={id}
      onChange={!readonly ? handleChange : undefined}
      {...extraProps}
      aria-describedby={ariaDescribedByIds<T>(id)}
    >
      {/* {labelValue(label, hideLabel, '')} */}
    </Checkbox>
  );
}
