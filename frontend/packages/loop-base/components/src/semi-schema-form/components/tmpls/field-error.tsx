// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import {
  type FieldErrorProps,
  type FormContextType,
  type RJSFSchema,
  type StrictRJSFSchema,
  errorId,
} from '@rjsf/utils';
import { Form } from '@coze-arch/coze-design';

/** The `FieldErrorTemplate` component renders the errors local to the particular field
 *
 * @param props - The `FieldErrorProps` for the errors being rendered
 */
export default function FieldErrorTemplate<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>(props: FieldErrorProps<T, S, F>) {
  const { errors = [], idSchema } = props;
  if (errors.length === 0) {
    return null;
  }
  const id = errorId<T>(idSchema);

  return (
    <div id={id}>
      {errors.map(error => (
        <Form.ErrorMessage key={`field-${id}-error-${error}`} error={error} />
      ))}
    </div>
  );
}
