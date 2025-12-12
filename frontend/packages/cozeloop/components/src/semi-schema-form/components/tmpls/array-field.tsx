// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import cn from 'classnames';
import {
  getTemplate,
  getUiOptions,
  type ArrayFieldTemplateProps,
  type ArrayFieldTemplateItemType,
  type FormContextType,
  type GenericObjectType,
  type RJSFSchema,
  type StrictRJSFSchema,
} from '@rjsf/utils';
import { Col, Row } from '@coze-arch/coze-design';

// const DESCRIPTION_COL_STYLE = {
//   paddingBottom: '8px',
// };

/** The `ArrayFieldTemplate` component is the template used to render all items in an array.
 *
 * @param props - The `ArrayFieldTemplateItemType` props for the component
 */
export default function ArrayFieldTemplate<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>(props: ArrayFieldTemplateProps<T, S, F>) {
  const {
    canAdd,
    className,
    disabled,
    formContext,
    idSchema,
    items,
    onAddClick,
    readonly,
    registry,
    // required,
    // schema,
    // title,
    uiSchema,
  } = props;
  const uiOptions = getUiOptions<T, S, F>(uiSchema);
  // const ArrayFieldDescriptionTemplate = getTemplate<
  //   'ArrayFieldDescriptionTemplate',
  //   T,
  //   S,
  //   F
  // >('ArrayFieldDescriptionTemplate', registry, uiOptions);
  const ArrayFieldItemTemplate = getTemplate<'ArrayFieldItemTemplate', T, S, F>(
    'ArrayFieldItemTemplate',
    registry,
    uiOptions,
  );
  // const ArrayFieldTitleTemplate = getTemplate<
  //   'ArrayFieldTitleTemplate',
  //   T,
  //   S,
  //   F
  // >('ArrayFieldTitleTemplate', registry, uiOptions);
  // Button templates are not overridden in the uiSchema
  const {
    ButtonTemplates: { AddButton },
  } = registry.templates;
  const { rowGutter = 24 } = formContext as GenericObjectType;

  return (
    <div
      className={cn('p-3 semi-card-bordered rounded-[4px]', className)}
      id={idSchema.$id}
    >
      <Row gutter={rowGutter}>
        {/* {uiOptions.title || title ? (
          <Col span={24}>
            <ArrayFieldTitleTemplate
              idSchema={idSchema}
              required={required}
              title={uiOptions.title || title}
              schema={schema}
              uiSchema={uiSchema}
              registry={registry}
            />
          </Col>
        ) : null} */}
        {/* {uiOptions.description || schema.description ? (
          <Col span={24} style={DESCRIPTION_COL_STYLE}>
            <ArrayFieldDescriptionTemplate
              description={uiOptions.description || schema.description}
              idSchema={idSchema}
              schema={schema}
              uiSchema={uiSchema}
              registry={registry}
            />
          </Col>
        ) : null} */}
        <Col className="row array-item-list" span={24}>
          {items
            ? items.map(
                ({
                  key,
                  ...itemProps
                }: ArrayFieldTemplateItemType<T, S, F>) => (
                  <ArrayFieldItemTemplate key={key} {...itemProps} />
                ),
              )
            : null}
        </Col>

        {canAdd ? (
          <Col span={24}>
            <Row gutter={rowGutter} justify="end">
              <Col span={6}>
                <AddButton
                  className="array-item-add"
                  disabled={disabled || readonly}
                  onClick={onAddClick}
                  uiSchema={uiSchema}
                  registry={registry}
                />
              </Col>
            </Row>
          </Col>
        ) : null}
      </Row>
    </div>
  );
}
