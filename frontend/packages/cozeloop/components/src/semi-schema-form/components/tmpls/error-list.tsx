// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import {
  type ErrorListProps,
  type FormContextType,
  type RJSFSchema,
  type StrictRJSFSchema,
  TranslatableString,
} from '@rjsf/utils';
import { IconCozWarningCircle } from '@coze-arch/coze-design/icons';
import { Banner, List, Space } from '@coze-arch/coze-design';

/** The `ErrorList` component is the template that renders the all the errors associated with the fields in the `Form`
 *
 * @param props - The `ErrorListProps` for this component
 */
export default function ErrorList<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>({ errors, registry }: ErrorListProps<T, S, F>) {
  const { translateString } = registry;
  const renderErrors = () => (
    <List className="list-group" size="small">
      {errors.map((error, index) => (
        <List.Item key={index}>
          <Space>
            <IconCozWarningCircle />
            {error.stack}
          </Space>
        </List.Item>
      ))}
    </List>
  );

  return (
    <Banner
      type="danger"
      className="panel panel-danger errors"
      title={translateString(TranslatableString.ErrorsLabel)}
      description={renderErrors()}
    />
  );
}
