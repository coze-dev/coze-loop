// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createPortal } from 'react-dom';
import { useState } from 'react';

import { isEmpty } from 'lodash-es';
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { Typography } from '@coze-arch/coze-design';

import { capitalizeFirstLetter } from '../utils/letter';
import { TagType, type Span, type RawMessage } from '../types';
import { ViewAllModal } from './view-all';
import { ToolCall } from './tool-call';
import { renderPlainText } from './plain-text';
import { MessageParts } from './message-parts';

import styles from './index.module.less';

interface SpanFieldRenderProps {
  messages?: RawMessage[];
  tagType: TagType;
  attrTos: Span['attr_tos'];
}

export const SpanFieldRender = (props: SpanFieldRenderProps) => {
  const { messages, tagType, attrTos } = props;
  const [showModal, setShowModal] = useState(false);
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
        return (
          <div className="mb-4 last:mb-0" key={index}>
            {role ? (
              <div className={styles['raw-title']}>
                {capitalizeFirstLetter(
                  role.toLocaleUpperCase().replace('MESSAGE', ' '),
                )}
              </div>
            ) : null}
            <div
              className={classNames(
                styles['raw-container'],
                'styled-scrollbar',
              )}
            >
              <div className={styles['raw-content']}>
                <span
                  className={classNames(styles['view-string'], {
                    [styles.empty]:
                      !content && !tool_calls && !reasoning_content,
                  })}
                >
                  <>
                    {(!isEmpty(rawContent.tool_calls) ||
                      !isEmpty(rawContent.parts)) && (
                      <>
                        <ToolCall raw={rawContent} />
                        <MessageParts raw={rawContent} attrTos={attrTos} />
                      </>
                    )}
                    {rawContent.content
                      ? renderPlainText(rawContent.content ?? '')
                      : null}
                    {/* 全部都是空的时候渲染 -  */}
                    {isEmpty(rawContent.tool_calls) &&
                      isEmpty(rawContent.parts) &&
                      !rawContent.content &&
                      renderPlainText('')}
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
                  {I18n.t('view_all')}
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
