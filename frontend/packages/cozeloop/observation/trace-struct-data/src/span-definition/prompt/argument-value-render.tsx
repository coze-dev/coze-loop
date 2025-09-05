// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isObject, truncate } from 'lodash-es';
import { JsonViewer } from '@textea/json-viewer';

import { JSON_VIEW_CONFIG } from '../../consts/json-view';
import { MultipartRender } from '../../components/multi-part-render';
import { type MultiPartSchema } from './schema';

export enum PromptArgumentValueType {
  Text = 'text',
  Message = 'model_message',
  MessagePart = 'model_message_part',
}

export interface PromptArgument {
  key: string;
  value?: string | { content?: string | null } | MultiPartSchema[] | undefined;
  source?: string;
  value_type?: PromptArgumentValueType;
}

export function ArgumentValueRender({
  promptArgument,
}: {
  promptArgument: PromptArgument | undefined;
}) {
  if (!promptArgument) {
    return '';
  }
  const { value, value_type } = promptArgument;
  if (value_type === PromptArgumentValueType.MessagePart) {
    if (!Array.isArray(value)) {
      return <JsonViewer value={value} {...JSON_VIEW_CONFIG} />;
    }
    const multiPart = value as MultiPartSchema[];
    return <MultipartRender className="coz-fg-primary" parts={multiPart} />;
  }
  if (isObject(value)) {
    return <JsonViewer value={value} {...JSON_VIEW_CONFIG} />;
  }
  return (
    <span className="coz-fg-primary break-all whitespace-pre-wrap leading-4">
      {value
        ? truncate(value, {
            length: 1000,
          })
        : '-'}
    </span>
  );
}
