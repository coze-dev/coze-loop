// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { type FocusEvent } from 'react';

import {
  ariaDescribedByIds,
  enumOptionsIndexForValue,
  enumOptionsValueForIndex,
  type FormContextType,
  type GenericObjectType,
  type RJSFSchema,
  type StrictRJSFSchema,
  type WidgetProps,
} from '@rjsf/utils';
import { Radio, type RadioGroupProps } from '@coze-arch/coze-design';

/** The `RadioWidget` is a widget for rendering a radio group.
 *  It is typically used with a string property constrained with enum options.
 *
 * @param props - The `WidgetProps` for this component
 */
export default function RadioWidget<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>({
  autofocus,
  disabled,
  formContext,
  id,
  onBlur,
  onChange,
  onFocus,
  options,
  readonly,
  value,
}: WidgetProps<T, S, F>) {
  const { readonlyAsDisabled = true } = formContext as GenericObjectType;

  const { enumOptions, enumDisabled, emptyValue } = options;

  const handleChange: RadioGroupProps['onChange'] = ({
    target: { value: nextValue },
  }) =>
    onChange(enumOptionsValueForIndex<S>(nextValue, enumOptions, emptyValue));

  const handleBlur = ({ target }: FocusEvent<HTMLInputElement>) =>
    onBlur(
      id,
      enumOptionsValueForIndex<S>(
        target && target.value,
        enumOptions,
        emptyValue,
      ),
    );

  const handleFocus = ({ target }: FocusEvent<HTMLInputElement>) =>
    onFocus(
      id,
      enumOptionsValueForIndex<S>(
        target && target.value,
        enumOptions,
        emptyValue,
      ),
    );

  const selectedIndexes = enumOptionsIndexForValue<S>(
    value,
    enumOptions,
  ) as string;

  // Antd's typescript definitions do not contain the following props that are actually necessary and, if provided,
  // they are used, so hacking them in via by spreading `extraProps` on the component to avoid typescript errors
  const extraProps = {
    onBlur: !readonly ? handleBlur : undefined,
    onFocus: !readonly ? handleFocus : undefined,
  };

  return (
    <Radio.Group
      disabled={disabled || (readonlyAsDisabled && readonly)}
      id={id}
      name={id}
      onChange={!readonly ? handleChange : undefined}
      {...extraProps}
      value={selectedIndexes}
      aria-describedby={ariaDescribedByIds<T>(id)}
    >
      {Array.isArray(enumOptions) &&
        enumOptions.map((option, i) => (
          <Radio
            name={id}
            autoFocus={i === 0 ? autofocus : false}
            disabled={
              disabled ||
              (Array.isArray(enumDisabled) &&
                enumDisabled.indexOf(option.value) !== -1)
            }
            key={i}
            value={String(i)}
          >
            {option.label}
          </Radio>
        ))}
    </Radio.Group>
  );
}
