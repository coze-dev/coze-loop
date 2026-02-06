// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { createPortal } from 'react-dom';
import React, { useDeferredValue, useState, useMemo } from 'react';

import { isObject } from 'lodash-es';
import cls from 'classnames';
import { JsonViewer } from '@textea/json-viewer';
import { IconCozLock } from '@coze-arch/coze-design/icons';
import { Typography, Tag } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';
import { useTraceDetailContext } from '@/features/trace-detail/hooks/use-trace-detail-context';
import { type Span, TagType } from '@/features/trace-data/types';
import { useFullScreen } from '@/features/trace-data/hooks/use-srcoll-view';
import {
  type BuildInValueTypes,
  getJsonViewConfig,
} from '@/features/trace-data/constants/json-view';
import { useConfigContext } from '@/config-provider';

import { VirtualText } from './virtual-text';
import { ViewAllModal } from './view-all';

import styles from './index.module.less';

const MAX_STRUCT_LENGTH = 10000;
interface RawContentProps {
  structuredContent: string | object;
  tagType?: TagType;
  className?: string;
  attrTos?: Span['attr_tos'];
  span?: Span;
}

export const RawContent: React.FC<
  RawContentProps & {
    enabledValuesTypes?: BuildInValueTypes[];
  }
> = ({
  structuredContent,
  tagType,
  className,
  attrTos,
  span,
  enabledValuesTypes,
}) => {
  const { isFullScreen } = useFullScreen();
  const [showModal, setShowModal] = useState(false);
  const { t } = useLocale();

  const handleViewAll = () => {
    setShowModal(true);
  };
  const { customParams } = useTraceDetailContext();
  const { spanRenderConfig } = customParams ?? {};

  const enableCopy = spanRenderConfig?.enableCopy
    ? spanRenderConfig.enableCopy(span as Span, tagType ?? '')
    : true;

  const showViewAllButton =
    (tagType === TagType.Input && attrTos?.input_data_url) ||
    (tagType === TagType.Output && attrTos?.output_data_url);

  // 检查是否为加密数据
  const isEncryptedData = span?.system_tags?.is_encryption_data === 'true';
  return (
    <div
      className={cls(styles['view-content'], 'styled-scrollbar', className)}
      style={
        isFullScreen
          ? {
              maxHeight: 'calc(100vh - 300px)',
            }
          : {}
      }
    >
      <div>
        {isEncryptedData ? (
          <EncryptedDataDisplay />
        ) : isObject(structuredContent) ? (
          <DeferredJSONViewer
            structuredContent={structuredContent}
            enableCopy={enableCopy}
            enabledValuesTypes={enabledValuesTypes}
          />
        ) : (
          <span
            className={cls(styles['view-string'], {
              [styles.empty]: !structuredContent,
              '!text-[#ff441e]': tagType === TagType.Error,
              'use-select-none': !enableCopy,
            })}
          >
            {((structuredContent as string)?.length > MAX_STRUCT_LENGTH ? (
              <VirtualText text={structuredContent as string} />
            ) : (
              (structuredContent as string)
            )) || '-'}
          </span>
        )}
      </div>
      {showViewAllButton ? (
        <div className="inline-flex justify-end w-full pb-2">
          <Typography.Text
            className="!text-[rgb(var(--coze-up-brand-9))] text-xs leading-4 font-medium cursor-pointer"
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
};

// 加密数据显示组件
const EncryptedDataDisplay: React.FC = () => {
  const { bizId } = useConfigContext();
  const { t } = useLocale();
  return (
    <Tag color="primary" size="mini" className="font-medium coz-fg-secondary">
      <IconCozLock className="text-gray-500 w-4 h-4" />
      <Typography.Text className="text-gray-600 flex-1">
        {t('current_data_encrypted')} {t('details_available')}：
      </Typography.Text>
      {bizId === 'cozeloop' || bizId === 'fornax' ? (
        <>
          <Typography.Text
            size="small"
            color="primary"
            className="text-[var(--coz-fg-hglt)]"
            link={{
              target: '_blank',
              href: 'https://bytedance.larkoffice.com/wiki/EKxzwqVNzifybMkB2UZcgqjRnHg',
            }}
          >
            {t('fornax_data_security_guide')}
          </Typography.Text>
        </>
      ) : null}
    </Tag>
  );
};

function DeferredJSONViewer({
  structuredContent,
  enableCopy,
  enabledValuesTypes,
}: {
  structuredContent: string | object;
  enableCopy: boolean;
  enabledValuesTypes?: BuildInValueTypes[];
}) {
  const deferredData = useDeferredValue(structuredContent);

  const contentLength = useMemo(() => {
    try {
      return JSON.stringify(structuredContent)?.length;
    } catch (error) {
      console.error('error', error);
      return 0;
    }
  }, [structuredContent]);

  return (
    <JsonViewer
      value={deferredData}
      {...getJsonViewConfig({
        enabledValuesTypes: enabledValuesTypes ?? ['previousResponseId'],
      })}
      defaultInspectDepth={contentLength > MAX_STRUCT_LENGTH ? 1 : 5}
      className={cls({
        [styles['cozeloop-json-viewer']]: !enableCopy,
      })}
    />
  );
}
