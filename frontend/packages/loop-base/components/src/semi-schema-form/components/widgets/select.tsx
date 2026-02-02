// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useMemo } from 'react';

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
import {
  type SelectProps,
  type OptionProps,
  Select,
} from '@coze-arch/coze-design';

const SELECT_STYLE = {
  width: '100%',
};

/** The `SelectWidget` is a widget for rendering dropdowns.
 *  It is typically used with string properties constrained with enum options.
 *
 * @param props - The `WidgetProps` for this component
 */
export default function SelectWidget<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>({
  autofocus,
  disabled,
  formContext = {} as unknown as F,
  id,
  multiple,
  onBlur,
  onChange,
  onFocus,
  options,
  placeholder,
  readonly,
  value,
}: WidgetProps<T, S, F>) {
  const { readonlyAsDisabled = true } = formContext as GenericObjectType;

  const { enumOptions, enumDisabled, emptyValue } = options;

  const handleChange: SelectProps['onChange'] = nextValue => {
    if (typeof nextValue === 'object' && !Array.isArray(nextValue)) {
      throw new Error('Object type value is not supported');
    } else {
      onChange(
        typeof nextValue === 'undefined'
          ? undefined
          : enumOptionsValueForIndex<S>(nextValue, enumOptions, emptyValue),
      );
    }
  };

  const handleBlur = () =>
    onBlur(id, enumOptionsValueForIndex<S>(value, enumOptions, emptyValue));

  const handleFocus = () =>
    onFocus(id, enumOptionsValueForIndex<S>(value, enumOptions, emptyValue));

  const selectedIndexes = enumOptionsIndexForValue<S>(
    value,
    enumOptions,
    multiple,
  );

  const selectOptions: OptionProps[] | undefined = useMemo(() => {
    if (Array.isArray(enumOptions)) {
      const opts: OptionProps[] = enumOptions.map(
        ({ value: optionValue, label: optionLabel }, index) => ({
          disabled:
            Array.isArray(enumDisabled) &&
            enumDisabled.indexOf(optionValue) !== -1,
          key: String(index),
          value: String(index),
          label: optionLabel,
        }),
      );

      return opts;
    }
    return undefined;
  }, [enumDisabled, enumOptions]);

  return (
    <Select
      autoFocus={autofocus}
      disabled={disabled || (readonlyAsDisabled && readonly)}
      id={id}
      multiple={multiple}
      onBlur={!readonly ? handleBlur : undefined}
      onChange={!readonly ? handleChange : undefined}
      onFocus={!readonly ? handleFocus : undefined}
      placeholder={placeholder}
      style={SELECT_STYLE}
      value={selectedIndexes}
      aria-describedby={ariaDescribedByIds<T>(id)}
      optionList={selectOptions}
    />
  );
}
