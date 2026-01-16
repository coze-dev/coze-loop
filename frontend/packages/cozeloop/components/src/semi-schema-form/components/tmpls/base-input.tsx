// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import React, { type FocusEvent } from 'react';

import {
  ariaDescribedByIds,
  type BaseInputTemplateProps,
  examplesId,
  getInputProps,
  type FormContextType,
  type GenericObjectType,
  type RJSFSchema,
  type StrictRJSFSchema,
} from '@rjsf/utils';
import {
  Input,
  CozInputNumber,
  type InputNumberProps,
  type InputProps,
} from '@coze-arch/coze-design';

const INPUT_STYLE = {
  width: '100%',
};

/** The `BaseInputTemplate` is the template to use to render the basic `<input>` component for the `core` theme.
 * It is used as the template for rendering many of the <input> based widgets that differ by `type` and callbacks only.
 * It can be customized/overridden for other themes or individual implementations as needed.
 *
 * @param props - The `WidgetProps` for this template
 */
export default function BaseInputTemplate<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>(props: BaseInputTemplateProps<T, S, F>) {
  const {
    disabled,
    formContext,
    id,
    onBlur,
    onChange,
    onChangeOverride,
    onFocus,
    options,
    placeholder,
    readonly,
    schema,
    value,
    type,
  } = props;
  const {
    step,
    type: fieldType,
    ...inputProps
  } = getInputProps<T, S, F>(schema, type, options, false);

  const { readonlyAsDisabled = true } = formContext as GenericObjectType;

  const handleNumberChange: InputNumberProps['onChange'] = nextValue =>
    onChange(nextValue);

  const handleTextChange: InputProps['onChange'] = (nextValue, e) => {
    onChangeOverride
      ? onChangeOverride(e)
      : onChange(nextValue === '' ? options.emptyValue : nextValue);
  };

  const handleBlur = ({ target }: FocusEvent<HTMLInputElement>) =>
    onBlur(id, target && target.value);

  const handleFocus = ({ target }: FocusEvent<HTMLInputElement>) =>
    onFocus(id, target && target.value);

  const input =
    fieldType === 'number' || fieldType === 'integer' ? (
      <CozInputNumber
        disabled={disabled || (readonlyAsDisabled && readonly)}
        id={id}
        name={id}
        onBlur={!readonly ? handleBlur : undefined}
        onChange={!readonly ? handleNumberChange : undefined}
        onFocus={!readonly ? handleFocus : undefined}
        placeholder={placeholder}
        style={INPUT_STYLE}
        list={schema.examples ? examplesId<T>(id) : undefined}
        {...inputProps}
        step={step as number | undefined}
        value={value}
        aria-describedby={ariaDescribedByIds<T>(id, !!schema.examples)}
      />
    ) : (
      <Input
        disabled={disabled || (readonlyAsDisabled && readonly)}
        id={id}
        name={id}
        onBlur={!readonly ? handleBlur : undefined}
        onChange={!readonly ? handleTextChange : undefined}
        onFocus={!readonly ? handleFocus : undefined}
        placeholder={placeholder}
        style={INPUT_STYLE}
        list={schema.examples ? examplesId<T>(id) : undefined}
        {...inputProps}
        value={value}
        aria-describedby={ariaDescribedByIds<T>(id, !!schema.examples)}
      />
    );

  return (
    <>
      {input}
      {Array.isArray(schema.examples) && (
        <datalist id={examplesId<T>(id)}>
          {(schema.examples as string[])
            .concat(
              schema.default && !schema.examples.includes(schema.default)
                ? ([schema.default] as string[])
                : [],
            )
            .map(example => (
              <option key={example} value={example} />
            ))}
        </datalist>
      )}
    </>
  );
}
