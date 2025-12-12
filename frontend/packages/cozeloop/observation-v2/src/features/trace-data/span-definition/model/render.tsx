/* eslint-disable max-params */
import React from 'react';

import { isEmpty } from 'lodash-es';
import { handleCopy as copy } from '@cozeloop/components';
import { type span } from '@cozeloop/api-schema/observation';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Button, Collapse, Tooltip, Typography } from '@coze-arch/coze-design';

import { beautifyJson } from '@/shared/utils/json';
import { useLocale } from '@/i18n';
import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import { type Span, TagType } from '@/features/trace-data/types';
import { type SpanRenderConfig } from '@/features/trace-data';

import { ReactComponent as IconSpanPluginTool } from '../../icons/icon-plugin-tool.svg';
import { RawContent } from '../../components/raw-content';
import { renderPlainText } from '../../components/plain-text';
import {
  type ModelNestParameterProperty,
  type Tool as ModelTool,
} from './schema';
import { ModelOutput } from './output';
import { ModelInput } from './input';
import { type Input, type Output, type Tool } from './index';

import styles from './index.module.less';

interface ModelDataRender {
  error: (
    error?: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  input: (
    input: RemoveUndefinedOrString<Input>,
    attrTos: Span['attr_tos'],
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
    context?: span.OutputSpan[] | undefined,
  ) => React.ReactNode;
  output: (
    output: RemoveUndefinedOrString<Output>,
    attrTos: Span['attr_tos'],
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
    context?: span.OutputSpan[] | undefined,
  ) => React.ReactNode;
  reasoningContent: (
    reasoningContent?: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  tool: (
    tool?: Tool,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
}

const ModelTool = (
  tool?: Tool,
  spanRenderConfig?: SpanRenderConfig,
  span?: Span,
) => {
  const { t } = useLocale();
  const handleCopy = (data: object) => {
    const str = beautifyJson(data);
    copy(str);
  };

  if (
    !tool ||
    !Array.isArray(tool) ||
    isEmpty(tool) ||
    typeof tool === 'string'
  ) {
    return (
      <RawContent
        structuredContent={tool as string}
        tagType={TagType.Functions}
        span={span}
      />
    );
  }
  return (
    <Collapse className={styles['function-collapse']}>
      {(tool as ModelTool[])?.map((raw, index) => (
        <Collapse.Panel
          className={styles['function-panel-content']}
          header={
            <div className="flex w-full max-w-full items-center justify-between">
              <Typography.Text
                ellipsis={{ showTooltip: true }}
                className="!font-mono"
              >
                <span className="flex items-center gap-x-1">
                  <span className="text-[16px] inline-flex items-center">
                    <IconSpanPluginTool
                      style={{
                        width: '16px',
                        height: '16px',
                      }}
                    />
                  </span>

                  <span>{raw?.function?.name}</span>
                </span>
              </Typography.Text>

              <Tooltip content={t('copy_tooltip_text')} theme="dark">
                <Button
                  className="!w-[24px] !h-[24px] box-border mr-1"
                  size="small"
                  color="secondary"
                  icon={
                    <IconCozCopy className="flex items-center justify-center w-[14px] h-[14px] text-[var(--coz-fg-secondary)]" />
                  }
                  onClick={e => {
                    e.stopPropagation();
                    handleCopy(raw);
                  }}
                />
              </Tooltip>
            </div>
          }
          itemKey={`${index}`}
          key={`${raw.function?.name ?? ''}${index}`}
        >
          <div className="flex flex-col px-3">
            <p className="text-xs">{raw?.function?.description}</p>
            <div>
              {Object.entries(raw?.function?.parameters?.properties || {}).map(
                ([key, value]) => (
                  <div
                    key={key}
                    className="grid grid-cols-[auto,1fr] overflow-hidden rounded-lg mt-2"
                    style={{ border: '1px solid #1D1C2314' }}
                  >
                    <div
                      className="col-span-2  px-4 py-2 font-mono text-sm"
                      style={{
                        borderBottom: '1px solid #1D1C2314',
                        backgroundColor: 'var(--semi-color-info-light-default)',
                      }}
                    >
                      {raw?.function?.parameters?.required?.includes(key) ? (
                        <span className="text-[red]">*</span>
                      ) : (
                        ''
                      )}
                      {key}
                    </div>
                    <div className="px-4 py-2 text-sm font-semibold">
                      {t('analytics_trace_type')}
                    </div>
                    <div className="px-4 py-2 text-sm">{value?.type}</div>
                    <div
                      className="border-t  px-4 py-2 text-sm font-semibold"
                      style={{ borderTop: '1px solid #1D1C2314' }}
                    >
                      {t('analytics_trace_description')}
                    </div>
                    <div
                      className="whitespace-pre-wrap px-4 py-2 text-sm"
                      style={{ borderTop: '1px solid #1D1C2314' }}
                    >
                      {value?.description}
                    </div>
                    {value.type === 'object' ? (
                      <>
                        <div
                          className="border-t  px-4 py-2 text-sm font-semibold"
                          style={{ borderTop: '1px solid #1D1C2314' }}
                        >
                          Properties
                        </div>
                        <div
                          className="whitespace-pre-wrap px-4 py-2 text-sm"
                          style={{
                            borderTop: '1px solid #1D1C2314',
                          }}
                        >
                          {renderPlainText(
                            (value as ModelNestParameterProperty).properties,
                          )}
                        </div>
                      </>
                    ) : null}
                  </div>
                ),
              )}
            </div>
          </div>
        </Collapse.Panel>
      ))}
    </Collapse>
  );
};

export const ModelDataRender: ModelDataRender = {
  error: (error?: string, spanRenderConfig?: SpanRenderConfig, span?: Span) => (
    <RawContent
      structuredContent={error ?? ''}
      tagType={TagType.Error}
      span={span}
    />
  ),
  input: (input, attrTos, spanRenderConfig, span, context) => (
    <ModelInput input={input} attrTos={attrTos} span={span} context={context} />
  ),
  output: (output, attrTos, spanRenderConfig, span, context) => (
    <ModelOutput
      output={output}
      attrTos={attrTos}
      span={span}
      context={context}
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
  tool: ModelTool,
};
