// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isObject } from 'lodash-es';
import classNames from 'classnames';
import { JsonViewer, type JsonViewerProps } from '@textea/json-viewer';

import { JSON_VIEW_CONFIG } from '../consts/json-view';

import styles from './index.module.less';

export const PlantText = ({ content }: { content: string | null }) => (
  <span className={classNames(styles['view-string'], {})}>
    {content || '-'}
  </span>
);

export const renderPlainText = (
  content: string | object | null,
  config?: Partial<JsonViewerProps>,
) =>
  isObject(content) ? (
    <JsonViewer {...JSON_VIEW_CONFIG} {...(config ?? {})} value={content} />
  ) : (
    <PlantText content={content} />
  );

export const renderJsonContent = (
  content: string | object | null,
  config?: Partial<JsonViewerProps>,
) =>
  isObject(content) ? (
    <JsonViewer {...JSON_VIEW_CONFIG} {...(config ?? {})} value={content} />
  ) : (
    <PlantText content={content} />
  );
