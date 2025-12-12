import { useEffect, useState } from 'react';

import { type LRUCache } from 'lru-cache';

import { safeJsonParse } from '@/shared/utils/json';
import {
  type SpanDefinition,
  type Span,
  type TagType,
} from '@/features/trace-data/types';
import { structDataListKeys } from '@/features/trace-data/constants';

import { type ResponseCacheItem } from '../trace-detail/store/response-api-cache';
import { ResponseApiCacheProvider } from './stores/response-api-cache';
import {
  BUILT_IN_SPAN_DEFINITIONS,
  defaultSpanDefinition,
} from './span-definition';
import { TypeEnum } from './components/span-content-container';
import { RawContent, SpanContentContainer } from './components';

export interface SpanRenderConfig {
  enableCopy?: (span: Span, type: TagType) => boolean;
}

const config = {
  moduleName: 'trace',
  point: 'span',
};
export interface TraceDetailLayoutProps {
  span: Span;
  customSpanDefinition?: SpanDefinition[];
  spanRenderConfig?: SpanRenderConfig;
  responseApiCache?: LRUCache<string, ResponseCacheItem, unknown>;
}

function getRender(key: string, spanDefinition: SpanDefinition) {
  if (key === 'input') {
    return spanDefinition.renderInput;
  }
  if (key === 'output') {
    return spanDefinition.renderOutput;
  }
  if (key === 'tool') {
    return spanDefinition.renderTool;
  }
  if (key === 'reasoningContent') {
    return spanDefinition.renderReasoningContent;
  }
  if (key === 'error') {
    return spanDefinition.renderError;
  }
  return () => <></>;
}
const spanDefinitionMap = new Map<string, SpanDefinition>();

export const TraceStructData = (props: TraceDetailLayoutProps) => {
  const { span, customSpanDefinition, spanRenderConfig, responseApiCache } =
    props;
  const [spanDefinitionInitialized, setSpanDefinitionInitialized] =
    useState(false);

  useEffect(() => {
    (
      [
        ...BUILT_IN_SPAN_DEFINITIONS,
        ...(customSpanDefinition ?? []),
      ] as SpanDefinition[]
    ).forEach(spanDefinition => {
      if (spanDefinitionMap.has(spanDefinition.name)) {
        console.warn(
          `spanDefinition ${spanDefinition.name} already exists, it will be overwritten`,
        );
      }
      spanDefinitionMap.set(spanDefinition.name, spanDefinition);
    });
    setSpanDefinitionInitialized(true);
  }, [customSpanDefinition]);

  const { span_type } = span;

  if (!spanDefinitionInitialized) {
    return null;
  }

  const targetSpanDefinition =
    spanDefinitionMap.get(span_type) ??
    (defaultSpanDefinition as SpanDefinition);

  const previousResponseId = span.system_tags?.previous_response_id;
  if (previousResponseId) {
    // 注入额外的上下文信息
    const contextSpan = responseApiCache?.get(previousResponseId);
    targetSpanDefinition?.setContext?.(contextSpan?.data ?? []);
  } else {
    targetSpanDefinition?.setContext?.([]);
  }

  const parseResults = targetSpanDefinition.parseSpanContent(span);
  const isEncryptedData = span?.system_tags?.is_encryption_data === 'true';

  return (
    <ResponseApiCacheProvider responseApiCache={responseApiCache}>
      <div>
        {structDataListKeys.map(({ key, title }) => {
          const result = parseResults[key];

          if (result.isEmpty) {
            return null;
          }
          return (
            <SpanContentContainer
              key={key}
              content={result.originalContent ?? ''}
              title={title}
              span={span}
              copyConfig={config}
              tagType={result.tagType}
              children={(renderType, content) => {
                if (
                  !result.isValidate ||
                  renderType === TypeEnum.JSON ||
                  isEncryptedData
                ) {
                  return (
                    <RawContent
                      structuredContent={content}
                      attrTos={span.attr_tos}
                      tagType={result.tagType}
                      span={span}
                    />
                  );
                }

                const structuredContent =
                  typeof result.content === 'string'
                    ? safeJsonParse(result.content)
                    : result.content;
                return getRender(key, targetSpanDefinition)(
                  span,
                  structuredContent,
                  spanRenderConfig,
                );
              }}
            />
          );
        })}
      </div>
    </ResponseApiCacheProvider>
  );
};

export { SpanContentContainer, RawContent } from './components';
export { getSpanContentField } from '@/features/trace-data/utils/span';
