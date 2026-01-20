// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import {
  type FieldTemplateProps,
  type FormContextType,
  type RJSFSchema,
  type StrictRJSFSchema,
} from '@rjsf/utils';
import { Form, Typography } from '@coze-arch/coze-design';

/** The `FieldTemplate` component is the template used by `SchemaField` to render any field. It renders the field
 * content, (label, description, children, errors and help) inside of a `WrapIfAdditional` component.
 *
 * @param props - The `FieldTemplateProps` for this component
 */
export default function FieldTemplate<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>(props: FieldTemplateProps<T, S, F>) {
  const {
    children,
    description,
    rawErrors,
    hidden,
    label,
    rawDescription,
    required,
  } = props;

  if (hidden) {
    return <div className="field-hidden">{children}</div>;
  }

  // check to see if there is rawDescription(string) before using description(ReactNode)
  // to prevent showing a blank description area
  const descriptionNode = rawDescription ? description : undefined;

  return (
    <Form.Slot
      label={{ text: label, required }}
      className="mb-4"
      error={{
        error: rawErrors,
        validateStatus: rawErrors?.length ? 'error' : undefined,
      }}
    >
      {children}

      <Typography.Paragraph className="text-xs w-full text-gray-500">
        {descriptionNode}
      </Typography.Paragraph>
    </Form.Slot>
  );
}
