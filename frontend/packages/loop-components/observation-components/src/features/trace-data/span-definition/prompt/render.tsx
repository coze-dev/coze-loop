// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-params */
import React from 'react';

import { isEmpty, isObject, truncate } from 'lodash-es';
import { JsonViewer } from '@textea/json-viewer';
import { handleCopy as copy } from '@cozeloop/components';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Button, Collapse, Typography } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';
import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import {
  type RawMessage,
  type Span,
  TagType,
} from '@/features/trace-data/types';
import { getJsonViewConfig } from '@/features/trace-data/constants/json-view';
import { type SpanRenderConfig } from '@/features/trace-data';

import { SpanFieldRender } from '../../components/span-field-render';
import { RawContent } from '../../components/raw-content';
import type { Input, Output } from './index';

import styles from './index.module.less';

interface PromptDataRender {
  error: (
    error?: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  input: (
    input: RemoveUndefinedOrString<Input>,
    attrTos?: Span['attr_tos'],
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  output: (
    output: RemoveUndefinedOrString<Output>,
    attrTos?: Span['attr_tos'],
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  reasoningContent: (
    reasoningContent?: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  tool: (
    tool?: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
}

interface IInputRenderProps {
  input: RemoveUndefinedOrString<Input>;
  attrTos?: Span['attr_tos'];
  spanRenderConfig?: SpanRenderConfig;
  span?: Span;
}
const InputRender = ({
  input,
  attrTos,
  spanRenderConfig,
  span,
}: IInputRenderProps) => {
  const { t } = useLocale();
  return (
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
          span={span}
        />
      </Collapse.Panel>
      {!isEmpty(input.arguments) && (
        <Collapse.Panel
          className={styles['prompt-panel-content']}
          header={
            <div className="flex justify-between items-center">
              <span>{t('analytics_trace_arguments')}</span>
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
                  className={`
                      ${styles['argu-item']}
                      flex gap-2 !items-start min-w-0
                    `}
                >
                  <Typography.Text
                    className="w-[140px] coz-fg-secondary text-xs"
                    ellipsis={{
                      showTooltip: {
                        opts: {
                          theme: 'dark',
                        },
                      },
                    }}
                  >
                    {argu.key}
                  </Typography.Text>
                  <div className="flex-1 overflow-hidden">
                    {isObject(argu.value) ? (
                      <JsonViewer
                        value={argu.value}
                        {...getJsonViewConfig({
                          enabledValuesTypes: ['previousResponseId'],
                        })}
                      />
                    ) : (
                      <span className="coz-fg-primary break-all whitespace-pre-wrap leading-4">
                        {argu.value
                          ? truncate(argu.value, {
                              length: 1000,
                            })
                          : '-'}
                      </span>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </Collapse.Panel>
      )}
    </Collapse>
  );
};

export const PromptDataRender: PromptDataRender = {
  error: (error?: string, spanRenderConfig?: SpanRenderConfig, span?: Span) => (
    <RawContent
      structuredContent={error ?? ''}
      tagType={TagType.Error}
      span={span}
    />
  ),
  input: (input, attrTos, spanRenderConfig, span) => (
    <InputRender input={input} attrTos={attrTos} span={span} />
  ),
  output: (output, attrTos, spanRenderConfig, span) => (
    <SpanFieldRender
      attrTos={attrTos}
      messages={(output ?? []).reduce((acc, cur) => {
        acc.push(cur);
        return acc;
      }, [] as RawMessage[])}
      tagType={TagType.Output}
      span={span}
    />
  ),
  reasoningContent: (
    reasoningContent?: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => (
    <RawContent
      structuredContent={reasoningContent ?? ''}
      tagType={TagType.ReasoningContent}
      span={span}
    />
  ),
  tool: () => null,
};
