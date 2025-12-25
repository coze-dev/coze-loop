// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */

import React from 'react';

import {
  type ArrayFieldTemplateItemType,
  type FormContextType,
  type RJSFSchema,
  type StrictRJSFSchema,
} from '@rjsf/utils';
import { ButtonGroup, Col, Row } from '@coze-arch/coze-design';

const BTN_GRP_STYLE = {
  width: '100%',
};

const BTN_STYLE = {
  width: 'calc(100% / 4)',
};

/** The `ArrayFieldItemTemplate` component is the template used to render an items of an array.
 *
 * @param props - The `ArrayFieldTemplateItemType` props for the component
 */
export default function ArrayFieldItemTemplate<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>(props: ArrayFieldTemplateItemType<T, S, F>) {
  const {
    children,
    disabled,
    hasCopy,
    hasMoveDown,
    hasMoveUp,
    hasRemove,
    hasToolbar,
    index,
    onCopyIndexClick,
    onDropIndexClick,
    onReorderClick,
    readonly,
    registry,
    uiSchema,
  } = props;
  const { CopyButton, MoveDownButton, MoveUpButton, RemoveButton } =
    registry.templates.ButtonTemplates;
  const { rowGutter = 24, toolbarAlign = 'top' } = registry.formContext;

  return (
    <Row align={toolbarAlign} key={`array-item-${index}`} gutter={rowGutter}>
      <Col span={18}>{children}</Col>

      {hasToolbar ? (
        <Col span={6}>
          <ButtonGroup style={BTN_GRP_STYLE}>
            {hasMoveUp || hasMoveDown ? (
              <MoveUpButton
                disabled={disabled || readonly || !hasMoveUp}
                onClick={onReorderClick(index, index - 1)}
                style={BTN_STYLE}
                uiSchema={uiSchema}
                registry={registry}
              />
            ) : null}
            {hasMoveUp || hasMoveDown ? (
              <MoveDownButton
                disabled={disabled || readonly || !hasMoveDown}
                onClick={onReorderClick(index, index + 1)}
                style={BTN_STYLE}
                uiSchema={uiSchema}
                registry={registry}
              />
            ) : null}
            {hasCopy ? (
              <CopyButton
                disabled={disabled || readonly}
                onClick={onCopyIndexClick(index)}
                style={BTN_STYLE}
                uiSchema={uiSchema}
                registry={registry}
              />
            ) : null}
            {hasRemove ? (
              <RemoveButton
                disabled={disabled || readonly}
                onClick={onDropIndexClick(index)}
                style={BTN_STYLE}
                uiSchema={uiSchema}
                registry={registry}
              />
            ) : null}
          </ButtonGroup>
        </Col>
      ) : null}
    </Row>
  );
}
