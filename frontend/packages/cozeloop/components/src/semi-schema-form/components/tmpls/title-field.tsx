// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import {
  type FormContextType,
  type TitleFieldProps,
  type RJSFSchema,
  type StrictRJSFSchema,
  getUiOptions,
} from '@rjsf/utils';
import { Typography } from '@coze-arch/coze-design';

/** The `TitleField` is the template to use to render the title of a field
 *
 * @param props - The `TitleFieldProps` for this component
 */
export default function TitleField<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>({ id, uiSchema, title }: TitleFieldProps<T, S, F>) {
  const uiOptions = getUiOptions<T, S, F>(uiSchema);

  return (
    <div id={id} className="my-1">
      <Typography.Title heading={5}>
        {uiOptions.title || title}
      </Typography.Title>
      <hr className="border-0 bg-secondary" style={{ height: '1px' }} />
    </div>
  );
}
