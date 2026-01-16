// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useMemo, useRef, useState } from 'react';

import { isEmpty, uniq } from 'lodash-es';
import { SideSheet, Tag, Collapse, EmptyState } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';
import { useResponseApiCacheStore } from '@/features/trace-data/stores/response-api-cache';

import { TraceItem } from './trace-item';
import { CustomAnchor } from './custom-anchor';

import styles from './index.module.less';

interface AnchorPanelProps {
  visible: boolean;
  onClose: () => void;
  previousResponseId: string;
}

const AnchorPanel: React.FC<AnchorPanelProps> = ({
  visible,
  onClose,
  previousResponseId,
}) => {
  const { t } = useLocale();
  const responseApiCache = useResponseApiCacheStore(
    state => state.responseApiCache,
  );

  const traceData = useMemo(() => {
    if (!responseApiCache || !previousResponseId) {
      return [];
    }
    const cacheItem = responseApiCache.get(previousResponseId);
    if (!cacheItem?.data) {
      return [];
    }
    return cacheItem.data.map((item, index) => ({
      id: item.system_tags?.response_id ?? '',
      round: index + 1,
      input: item.input,
      output: item.output,
    }));
  }, [previousResponseId, responseApiCache]);

  const contentRef = useRef<HTMLDivElement>(null);
  const sectionRefs = useRef<Record<string, HTMLElement | null>>({});

  // 生成锚点配置
  const anchorItems = traceData.map(item => ({
    anchorId: `anchor-${item.id}`,
    title: t('round', { round: item.round }),
    id: item.id,
  }));

  const [activeKey, setActiveKey] = useState<string[]>(
    anchorItems.map(item => item.id),
  );

  return (
    <SideSheet
      keepDOM={false}
      visible={visible}
      onCancel={onClose}
      bodyStyle={{
        padding: 0,
      }}
      title={
        <div className="flex items-center gap-2">
          <div className="text-[16px] font-semibold coz-fg-plus">
            {t('response_api_full_text')}
          </div>
          <Tag color="grey">JSON</Tag>
        </div>
      }
      width={1000}
    >
      {isEmpty(traceData) ? (
        <EmptyState description={t('no_context_available')} />
      ) : (
        <div className="flex h-full border-[0] border-t border-solid border-[var(--coz-stroke-primary)]">
          <div className="w-48 border-r border-gray-200 p-4 overflow-y-auto">
            <CustomAnchor
              defaultAnchor={`#${anchorItems[0]?.anchorId ?? ''}`}
              getContainer={() => contentRef.current}
              scrollMotion
              className={styles.anchor}
              onBeforeChange={event => {
                const targetId = event.replace('#anchor-', '');
                setActiveKey(prev => uniq([...prev, targetId]));
              }}
            >
              {anchorItems.map(item => (
                <CustomAnchor.Link
                  key={item.anchorId}
                  href={`#${item.anchorId}`}
                  title={item.title}
                />
              ))}
            </CustomAnchor>
          </div>

          <Collapse
            className="w-full max-h-full overflow-hidden border-0 border-l border-solid border-[var(--coz-stroke-primary)]"
            activeKey={activeKey}
            onChange={key => setActiveKey(key as string[])}
            motion={false}
          >
            <div ref={contentRef} className="h-full overflow-y-auto">
              {traceData.map(item => (
                <TraceItem
                  key={item.id}
                  id={item.id}
                  round={item.round}
                  input={item.input}
                  output={item.output}
                  sectionRefs={sectionRefs}
                />
              ))}
            </div>
          </Collapse>
        </div>
      )}
    </SideSheet>
  );
};

export { AnchorPanel };
