// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import {
  getSubmitButtonOptions,
  type FormContextType,
  type RJSFSchema,
  type StrictRJSFSchema,
  type SubmitButtonProps,
} from '@rjsf/utils';
import { Button } from '@coze-arch/coze-design';

/** The `SubmitButton` renders a button that represent the `Submit` action on a form
 */
export default function SubmitButton<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>({ uiSchema }: SubmitButtonProps<T, S, F>) {
  const {
    submitText,
    norender,
    props: submitButtonProps,
  } = getSubmitButtonOptions(uiSchema);
  if (norender) {
    return null;
  }
  return (
    <Button type="primary" {...submitButtonProps} htmlType="submit">
      {submitText}
    </Button>
  );
}
