// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import {
  ariaDescribedByIds,
  rangeSpec,
  type FormContextType,
  type RJSFSchema,
  type StrictRJSFSchema,
  type WidgetProps,
  type GenericObjectType,
} from '@rjsf/utils';
import { Slider, type SliderProps } from '@coze-arch/coze-design';

/** The `RangeWidget` component uses the `BaseInputTemplate` changing the type to `range` and wrapping the result
 * in a div, with the value along side it.
 *
 * @param props - The `WidgetProps` for this component
 */
export default function RangeWidget<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>(props: WidgetProps<T, S, F>) {
  const {
    autofocus,
    disabled,
    formContext,
    id,
    onBlur,
    onChange,
    onFocus,
    options,
    placeholder,
    readonly,
    schema,
    value,
  } = props;
  const { readonlyAsDisabled = true } = formContext as GenericObjectType;

  const { min, max, step } = rangeSpec(schema);

  const emptyValue = options.emptyValue || '';

  const handleChange: SliderProps['onChange'] = nextValue =>
    onChange(typeof nextValue === 'undefined' ? emptyValue : nextValue);

  const handleBlur = () => onBlur(id, value);

  const handleFocus = () => onFocus(id, value);

  // Antd's typescript definitions do not contain the following props that are actually necessary and, if provided,
  // they are used, so hacking them in via by spreading `extraProps` on the component to avoid typescript errors
  const extraProps = {
    placeholder,
    onBlur: !readonly ? handleBlur : undefined,
    onFocus: !readonly ? handleFocus : undefined,
  };

  return (
    <Slider
      autoFocus={autofocus}
      disabled={disabled || (readonlyAsDisabled && readonly)}
      id={id}
      max={max}
      min={min}
      onChange={!readonly ? handleChange : undefined}
      range={false}
      step={step}
      value={value}
      {...extraProps}
      aria-describedby={ariaDescribedByIds<T>(id)}
    />
  );
}
