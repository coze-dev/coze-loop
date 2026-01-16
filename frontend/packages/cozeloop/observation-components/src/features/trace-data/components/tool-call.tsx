// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isEmpty } from 'lodash-es';
import cls from 'classnames';

import { safeJsonParse } from '@/shared/utils/json';
import {
  type Part,
  type Span,
  type RawMessage,
} from '@/features/trace-data/types';
import { ReactComponent as IconSpanPluginTool } from '@/features/trace-data/icons/icon-plugin-tool.svg';

import { renderJsonContent, renderPlainText } from './plain-text';
import { MessageParts } from './message-parts';

import styles from './index.module.less';

interface ToolCallProps {
  raw: RawMessage;
  attrTos?: Span['attr_tos'];
}
export const ToolCall = (props: ToolCallProps) => {
  const { raw, attrTos } = props;

  if (isEmpty(raw.tool_calls)) {
    return null;
  }
  return (
    <div className="flex gap-2 flex-col">
      {raw.tool_calls?.map((tool, ind) => {
        const query = safeJsonParse(
          typeof tool?.function?.arguments === 'string'
            ? tool?.function?.arguments
            : JSON.stringify(tool?.function?.arguments),
        );
        return (
          <div key={ind} className="flex gap-2 flex-col">
            <div className={cls(styles['tool-title'], 'font-mono')}>
              <IconSpanPluginTool style={{ width: '16px', height: '16px' }} />
              {tool?.function?.name || '-'}
            </div>
            {raw.role === 'tool' && raw.content ? (
              Array.isArray(raw.content) ? (
                <MessageParts
                  raw={{
                    parts: raw.content as Part[],
                    role: raw.role,
                  }}
                  attrTos={attrTos}
                />
              ) : (
                renderPlainText(raw.content)
              )
            ) : (
              renderJsonContent(query)
            )}
          </div>
        );
      })}
    </div>
  );
};
