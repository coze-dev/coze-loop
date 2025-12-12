import React from 'react';

import { isEmpty } from 'lodash-es';
import { handleCopy as copy } from '@cozeloop/components';
import { IconCozCopy, IconCozDocumentFill } from '@coze-arch/coze-design/icons';
import {
  Collapse,
  Tag,
  Tooltip,
  Typography,
  Button,
} from '@coze-arch/coze-design';

import { beautifyJson } from '@/shared/utils/json';
import { useLocale } from '@/i18n';
import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import { TagType, type Span } from '@/features/trace-data/types';
import { type SpanRenderConfig } from '@/features/trace-data';

import { RawContent } from '../../components/raw-content';
import type { RetrieverInputSchema, RetrieverOutputSchema } from './schema';
import type { RetrieverInputData, RetrieverOutputData } from './index';

import styles from './index.module.less';

interface RetrieverDataRender {
  input: (
    input: RemoveUndefinedOrString<RetrieverInputData>,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  output: (
    output: RemoveUndefinedOrString<RetrieverOutputData>,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  error: (
    error?: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
}

// Retriever Input 组件
const RetrieverInputContent = (
  input: RetrieverInputSchema | string,
  spanRenderConfig?: SpanRenderConfig,
  span?: Span,
) => (
  <RawContent
    structuredContent={input as string}
    tagType={TagType.Input}
    span={span}
  />
);

const RetrieverOutputContent = (
  output: RetrieverOutputSchema | string,
  spanRenderConfig?: SpanRenderConfig,
  span?: Span,
) => {
  const { t } = useLocale();
  const handleCopy = (data: object) => {
    const str = beautifyJson(data);
    copy(str);
  };

  if (typeof output === 'string' || !output || isEmpty(output?.documents)) {
    return (
      <RawContent
        structuredContent={output as string}
        tagType={TagType.Output}
        span={span}
      />
    );
  }

  const { documents } = output;

  return (
    <Collapse className={styles['function-collapse']}>
      {documents.map((raw, index) => (
        <Collapse.Panel
          className={styles['function-panel-content']}
          header={
            <div className="flex w-full max-w-full items-center justify-between">
              <Typography.Text
                ellipsis={{ showTooltip: true }}
                className="!font-mono"
              >
                <span className="flex items-center gap-x-1">
                  <span
                    className="w-[16px] h-[16px] bg-brand inline-flex items-center justify-center text-white font-semibold text-[10px]"
                    style={{
                      borderRadius: '4px',
                    }}
                  >
                    <IconCozDocumentFill className="!w-[8px] !h-[8px]" />
                  </span>

                  <span>{raw?.index}</span>
                </span>
              </Typography.Text>

              <div className="flex items-center gap-x-1">
                <Tag color="primary" size="mini">
                  {raw?.score?.toFixed(2)}
                </Tag>
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
            </div>
          }
          itemKey={`${index}`}
          key={raw.id}
        >
          <div className="flex flex-col px-3">
            <p className="text-xs">{raw?.content}</p>
          </div>
        </Collapse.Panel>
      ))}
    </Collapse>
  );
};

export const RetrieverDataRender: RetrieverDataRender = {
  input: (input, spanRenderConfig, span) =>
    RetrieverInputContent(input, spanRenderConfig, span),
  output: (output, spanRenderConfig, span) =>
    RetrieverOutputContent(output, spanRenderConfig, span),
  error: (error?: string, spanRenderConfig?: SpanRenderConfig, span?: Span) => (
    <RawContent
      structuredContent={error ?? ''}
      tagType={TagType.Error}
      span={span}
    />
  ),
};
