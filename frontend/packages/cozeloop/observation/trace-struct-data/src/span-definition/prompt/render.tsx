// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { isEmpty } from 'lodash-es';
import { I18n } from '@cozeloop/i18n-adapter';
import { handleCopy as copy } from '@cozeloop/components';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Button, Collapse, Typography } from '@coze-arch/coze-design';

import { type RemoveUndefinedOrString } from '../../types/utils';
import { type RawMessage, type Span, TagType } from '../../types';
import { SpanFieldRender } from '../../components/span-field-render';
import { RawContent } from '../../components/raw-content';
import type { Input, Output } from './index';
import { ArgumentValueRender } from './argument-value-render';

import styles from './index.module.less';

interface PromptDataRender {
  error: (error?: string) => React.ReactNode;
  input: (
    input: RemoveUndefinedOrString<Input>,
    attrTos?: Span['attr_tos'],
  ) => React.ReactNode;
  output: (
    output: RemoveUndefinedOrString<Output>,
    attrTos?: Span['attr_tos'],
  ) => React.ReactNode;
  reasoningContent: (reasoningContent?: string) => React.ReactNode;
  tool: (tool?: string) => React.ReactNode;
}

export const PromptDataRender: PromptDataRender = {
  error: (error?: string) => (
    <RawContent structuredContent={error ?? ''} tagType={TagType.Error} />
  ),
  input: (input, attrTos) => (
    <Collapse
      className={styles['prompt-collapse']}
      defaultActiveKey={['1', '2']}
    >
      <Collapse.Panel
        className={styles['prompt-panel-content']}
        header={'Prompt Templates'}
        itemKey="1"
      >
        <SpanFieldRender
          attrTos={attrTos}
          messages={input.templates as RawMessage[]}
          tagType={TagType.Input}
        />
      </Collapse.Panel>
      {!isEmpty(input.arguments) && (
        <Collapse.Panel
          className={styles['prompt-panel-content']}
          header={
            <div className="flex justify-between items-center">
              <span>{I18n.t('analytics_trace_arguments')}</span>
              <Button
                size="small"
                color="secondary"
                onClick={e => {
                  e.stopPropagation();
                  copy(JSON.stringify(input.arguments));
                }}
                icon={
                  <IconCozCopy className="!flex items-center justify-center h-4 w-4 !text-[#6B6B75]" />
                }
              />
            </div>
          }
          itemKey="2"
        >
          <div className={`leading-3 ${styles['argu-container']}`}>
            {input.arguments?.map((argu, ind: number) => (
              <div key={ind} className={styles['argu-item-container']}>
                <div
                  className={`${styles['argu-item']} flex gap-2 !items-start min-w-0`}
                >
                  <Typography.Text
                    className="w-[140px] coz-fg-secondary text-xs"
                    ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
                  >
                    {argu.key}
                  </Typography.Text>
                  <div className="flex-1 overflow-hidden">
                    <ArgumentValueRender promptArgument={argu} />
                  </div>
                </div>
              </div>
            ))}
          </div>
        </Collapse.Panel>
      )}
    </Collapse>
  ),
  output: (output, attrTos) => (
    <SpanFieldRender
      attrTos={attrTos}
      messages={(output ?? []).reduce((acc, cur) => {
        acc.push(cur);
        return acc;
      }, [] as RawMessage[])}
      tagType={TagType.Output}
    />
  ),
  reasoningContent: (reasoningContent?: string) => (
    <RawContent
      structuredContent={reasoningContent ?? ''}
      tagType={TagType.ReasoningContent}
    />
  ),
  tool: () => null,
} as const;
