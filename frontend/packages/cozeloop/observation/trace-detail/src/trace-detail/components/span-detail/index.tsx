// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { isEmpty, isFunction } from 'lodash-es';
import cs from 'classnames';
import {
  TraceStructData,
  SpanContentContainer,
  RawContent,
  getSpanContentField,
} from '@cozeloop/trace-struct-data';
import { I18n } from '@cozeloop/i18n-adapter';
import { PlatformType } from '@cozeloop/api-schema/observation';
import { IconCozIllusEmpty } from '@coze-arch/coze-design/illustrations';
import { Col, Empty, Row, Tabs } from '@coze-arch/coze-design';

import type { Span } from '@/trace-detail/typings/params';
import { useTraceDetailContext } from '@/trace-detail/hooks/use-trace-detail-context';

import { SpanFieldList } from '../span-detail-list';
import { SpanDetailHeader } from './span-header';
import { geSpanOverviewField } from './field';

import styles from './index.module.less';

interface SpanDetailProps {
  span: Span;
  showTags?: boolean;
  baseInfoPosition?: 'right' | 'top';
  className?: string;
  minColWidth?: number;
  maxColNum?: number;
  moduleName?: string;
}

enum TabKey {
  Run = 'run',
  Metadata = 'metadata',
}

export const SpanDetail = ({
  span,
  baseInfoPosition = 'right',
  showTags = true,
  maxColNum,
  minColWidth,
  moduleName,
  className,
}: SpanDetailProps) => {
  const { custom_tags } = span;
  const { runtime, ...otherTags } = custom_tags || {};
  const overviewFields = useMemo(() => geSpanOverviewField(span), [span]);
  const spanContentList = useMemo(() => getSpanContentField(span), [span]);
  const { extraSpanDetailTabs, defaultActiveTabKey, platformType } =
    useTraceDetailContext();

  const actualDefaultActiveTabKey = useMemo(() => {
    const targetTabs = extraSpanDetailTabs?.find(
      tab => tab.tabKey === defaultActiveTabKey,
    );
    if (
      !targetTabs ||
      (!isFunction(targetTabs.visible) && !targetTabs.visible) ||
      (isFunction(targetTabs.visible) && !targetTabs.visible(span))
    ) {
      return TabKey.Run;
    }
    return defaultActiveTabKey;
  }, [defaultActiveTabKey, extraSpanDetailTabs]);

  return (
    <div className={cs(className, styles.container)}>
      <SpanDetailHeader span={span} moduleName={moduleName} />
      <Tabs className={styles.tab} defaultActiveKey={actualDefaultActiveTabKey}>
        <Tabs.TabPane tab={I18n.t('analytics_trace_run')} itemKey={TabKey.Run}>
          <Row className={styles['tab-content']}>
            <Col span={baseInfoPosition === 'top' ? 24 : 19}>
              {spanContentList?.length > 0 ? (
                <>
                  {baseInfoPosition === 'top' && (
                    <SpanFieldList
                      fields={overviewFields}
                      span={span}
                      maxColNum={maxColNum}
                      minColWidth={minColWidth}
                      layout="horizontal"
                    />
                  )}
                  <div className="flex flex-col">
                    <TraceStructData span={span} />
                  </div>
                </>
              ) : (
                <div className="flex items-center justify-center h-full w-full mt-[150px]">
                  <Empty
                    image={
                      <IconCozIllusEmpty style={{ width: 150, height: 150 }} />
                    }
                    title={I18n.t('reported_data_not_found')}
                    description={I18n.t('report_in_sdk')}
                  />
                </div>
              )}
            </Col>
            {baseInfoPosition === 'right' ? (
              <Col span={5} className={styles['span-detail']}>
                <SpanFieldList
                  fields={overviewFields}
                  span={span}
                  layout="vertical"
                />
              </Col>
            ) : null}
          </Row>
        </Tabs.TabPane>
        {showTags ? (
          <Tabs.TabPane tab={'Metadata'} itemKey={TabKey.Metadata}>
            {!isEmpty(otherTags) && (
              <>
                <SpanContentContainer
                  content={otherTags}
                  title={I18n.t('analytics_trace_metadata')}
                  hasBottomLine={false}
                  copyConfig={{
                    moduleName,
                    point: 'meta_data',
                  }}
                  hideSwitchRawType
                  children={(_renderType, content) => (
                    <RawContent structuredContent={content} />
                  )}
                />
              </>
            )}
            {runtime ? (
              <SpanContentContainer
                content={runtime}
                title={I18n.t('analytics_trace_runtime')}
                hasBottomLine={false}
                copyConfig={{
                  moduleName,
                  point: 'runtime',
                }}
                hideSwitchRawType
                children={(_renderType, content) => (
                  <RawContent structuredContent={content} />
                )}
              />
            ) : null}
          </Tabs.TabPane>
        ) : null}

        {extraSpanDetailTabs
          ?.filter(tab =>
            isFunction(tab.visible) ? tab.visible(span) : (tab.visible ?? true),
          )
          ?.map(extraTab => (
            <Tabs.TabPane
              tab={extraTab.label}
              itemKey={extraTab.tabKey}
              key={extraTab.tabKey}
            >
              <div className="w-full h-full px-5 py-4">
                {extraTab.render(span, platformType ?? PlatformType.Cozeloop)}
              </div>
            </Tabs.TabPane>
          ))}
      </Tabs>
    </div>
  );
};
