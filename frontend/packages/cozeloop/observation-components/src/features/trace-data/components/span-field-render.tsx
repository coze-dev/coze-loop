// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { createPortal } from 'react-dom';
import { useState } from 'react';

import { isEmpty } from 'lodash-es';
import cls from 'classnames';
import { handleCopy } from '@cozeloop/components';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Button, Tooltip, Typography } from '@coze-arch/coze-design';

import { capitalizeFirstLetter } from '@/shared/utils/letter';
import { useLocale } from '@/i18n';
import {
  TagType,
  type Span,
  type RawMessage,
  type Part,
} from '@/features/trace-data/types';

import { type SpanRenderConfig } from '../../trace-data';
import { ViewAllModal } from './view-all';
import { ToolCall } from './tool-call';
import { renderPlainText } from './plain-text';
import { MessageParts } from './message-parts';

import styles from './index.module.less';

interface SpanFieldRenderProps {
  messages?: RawMessage[];
  tagType: TagType;
  attrTos: Span['attr_tos'];
  spanRenderConfig?: SpanRenderConfig;
  span?: Span;
}

export const SpanFieldRender = (props: SpanFieldRenderProps) => {
  const { messages, tagType, attrTos, spanRenderConfig, span } = props;
  const [showModal, setShowModal] = useState(false);
  const { t } = useLocale();

  const handleViewAll = () => {
    setShowModal(true);
  };
  const showViewAllButton =
    (tagType === TagType.Input && attrTos?.input_data_url) ||
    (tagType === TagType.Output && attrTos?.output_data_url);

  return (
    <div>
      {messages?.map((rawContent, index) => {
        const { role, content, tool_calls, reasoning_content } = rawContent;
        const enableCopy = spanRenderConfig?.enableCopy
          ? spanRenderConfig.enableCopy(span as Span, tagType)
          : true;
        return (
          <div className="mb-4 last:mb-0 group" key={index}>
            {role ? (
              <div
                className={cls(
                  styles['raw-title'],
                  'flex items-center justify-between w-full',
                )}
              >
                <span className={styles.text}>
                  {capitalizeFirstLetter(
                    role.toLocaleUpperCase().replace('MESSAGE', ' '),
                  )}
                </span>

                {rawContent.response_id ? (
                  <div className="flex items-center gap-x-1 opacity-0 group-hover:opacity-100 coz-fg-secondary text-[12px]">
                    <div>ID: {rawContent.response_id}</div>
                    <Tooltip content={t('copy_id_tooltip')}>
                      <Button
                        onClick={() => handleCopy(rawContent.response_id ?? '')}
                        icon={<IconCozCopy className="coz-fg-secondary" />}
                        size="mini"
                        type="secondary"
                        color="secondary"
                      />
                    </Tooltip>
                  </div>
                ) : null}
              </div>
            ) : null}
            <div className={cls(styles['raw-container'], 'styled-scrollbar')}>
              <div className={styles['raw-content']}>
                <span
                  className={cls(styles['view-string'], {
                    [styles.empty]:
                      !(content || isEmpty(content)) &&
                      !tool_calls &&
                      !reasoning_content,
                    'user-select-none': !enableCopy,
                  })}
                >
                  <>
                    <>
                      {(!isEmpty(rawContent.tool_calls) ||
                        !isEmpty(rawContent.parts)) && (
                        <>
                          <ToolCall raw={rawContent} attrTos={attrTos} />
                          <MessageParts raw={rawContent} attrTos={attrTos} />
                        </>
                      )}
                      {typeof rawContent.content === 'string' &&
                      (rawContent.role !== 'tool' ||
                        (isEmpty(rawContent.tool_calls) &&
                          rawContent.content)) ? (
                        renderPlainText(rawContent.content ?? '')
                      ) : Array.isArray(rawContent.content) ? (
                        <MessageParts
                          raw={{
                            parts: rawContent.content as Part[],
                            role: rawContent.role,
                          }}
                          attrTos={attrTos}
                        />
                      ) : null}
                      {/* 全部都是空的时候渲染 -  */}
                      {isEmpty(rawContent.tool_calls) &&
                        isEmpty(rawContent.parts) &&
                        !(rawContent.content || isEmpty(rawContent.content)) &&
                        renderPlainText('')}
                    </>
                  </>
                </span>
              </div>
            </div>
            {showViewAllButton ? (
              <div className="inline-flex justify-end  w-full pb-2">
                <Typography.Text
                  className="!text-brand-9 text-xs leading-4 font-medium cursor-pointer"
                  onClick={handleViewAll}
                >
                  {t('view_all')}
                </Typography.Text>
              </div>
            ) : null}

            {showModal
              ? createPortal(
                  <ViewAllModal
                    onViewAllClick={setShowModal}
                    tagType={tagType}
                    attrTos={attrTos}
                  />,
                  document.getElementById(
                    'trace-detail-side-sheet-panel',
                  ) as HTMLDivElement,
                )
              : null}
          </div>
        );
      })}
    </div>
  );
};
