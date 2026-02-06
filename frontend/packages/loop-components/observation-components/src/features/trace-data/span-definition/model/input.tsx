// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Fragment } from 'react';

import { isEmpty } from 'lodash-es';
import { type span } from '@cozeloop/api-schema/observation';
import { Collapse } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';
import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import {
  type RawMessage,
  TagType,
  type Span,
} from '@/features/trace-data/types';
import { RawContent, type SpanRenderConfig } from '@/features/trace-data';

import {
  getInputAndTools,
  getOutputAndReasoningContent,
  type Input,
} from '../model';
import { SpanFieldRender } from '../../components/span-field-render';

import styles from './index.module.less';

interface Props {
  input: RemoveUndefinedOrString<Input>;
  attrTos: Span['attr_tos'];
  spanRenderConfig?: SpanRenderConfig;
  span?: Span;
  context?: span.OutputSpan[] | undefined;
}

export const ModelInput = (props: Props) => {
  const { input, attrTos, span, context } = props;
  const { t } = useLocale();

  // 当 context 为空时，直接渲染原始 input 状态
  if (isEmpty(context)) {
    return (
      <SpanFieldRender
        attrTos={attrTos}
        messages={input}
        tagType={TagType.Input}
        span={span}
      />
    );
  }

  const responseApiContext: [
    ReturnType<typeof getInputAndTools>,
    ReturnType<typeof getOutputAndReasoningContent>,
    string | undefined,
  ][] =
    context?.map(item => [
      getInputAndTools(item.input),
      getOutputAndReasoningContent(item.output),
      item.system_tags?.response_id,
    ]) ?? [];

  return (
    <Collapse
      className={styles['response-api-collapse']}
      defaultActiveKey={['context', 'original']}
    >
      <Collapse.Panel itemKey="context" header={t('context_from_response_id')}>
        <div className="flex flex-col gap-y-4">
          {responseApiContext.map((item, index) => {
            const [
              inputAndToolsItem,
              outputAndReasoningContentItem,
              responseId,
            ] = item;
            const { input: inputContent } = inputAndToolsItem;
            const { output: outputContent } = outputAndReasoningContentItem;

            return (
              <Fragment key={index}>
                <>
                  {inputContent.isValidate ? (
                    <SpanFieldRender
                      attrTos={attrTos}
                      messages={(inputContent.content as RawMessage[]).map(
                        message => ({
                          ...message,
                          response_id: responseId,
                        }),
                      )}
                      tagType={inputContent.tagType}
                      span={span}
                    />
                  ) : (
                    <RawContent
                      structuredContent={inputContent.originalContent}
                      tagType={TagType.Input}
                      span={span}
                    />
                  )}
                </>
                <>
                  {outputContent.isValidate &&
                  typeof outputContent.content !== 'string' ? (
                    <SpanFieldRender
                      attrTos={attrTos}
                      messages={outputContent.content.choices.reduce(
                        (acc, cur) => {
                          acc.push({
                            ...(cur.message ?? {}),
                            response_id: responseId,
                          });
                          return acc;
                        },
                        [] as RawMessage[],
                      )}
                      tagType={TagType.Output}
                      span={span}
                    />
                  ) : (
                    <RawContent
                      structuredContent={outputContent.originalContent}
                      tagType={TagType.Output}
                      span={span}
                    />
                  )}
                </>
              </Fragment>
            );
          })}
        </div>
      </Collapse.Panel>
      <Collapse.Panel itemKey="original" header={t('current_context')}>
        <SpanFieldRender
          attrTos={attrTos}
          messages={input}
          tagType={TagType.Input}
          span={span}
        />
      </Collapse.Panel>
    </Collapse>
  );
};
