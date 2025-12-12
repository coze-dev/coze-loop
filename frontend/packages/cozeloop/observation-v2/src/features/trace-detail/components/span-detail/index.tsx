/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-magic-numbers */
/* eslint-disable complexity */
import { useMemo } from 'react';

import { isEmpty, isFunction } from 'lodash-es';
import cs from 'classnames';
import { PlatformType, type span } from '@cozeloop/api-schema/observation';
import { IconCozIllusEmpty } from '@coze-arch/coze-design/illustrations';
import { Col, Empty, Row, Tabs } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';
import { useTraceDetailContext } from '@/features/trace-detail/hooks/use-trace-detail-context';
import { type CozeloopTraceDetailProps } from '@/features/trace-detail/containers/trace-detail/interface';
import {
  TraceStructData,
  SpanContentContainer,
  RawContent,
  getSpanContentField,
} from '@/features/trace-data';

import { SpanFieldList } from '../span-detail-list';
import { useResponseApiCacheStore } from '../../store/response-api-cache';
import { SpanDetailHeader } from './span-header';
import { useSpanOverviewField } from './field';

import styles from './index.module.less';

interface SpanDetailProps {
  span: span.OutputSpan;
  className?: string;
  moduleName?: string;
  spanConfig: CozeloopTraceDetailProps['spanDetailConfig'];
}

enum TabKey {
  Run = 'run',
  Metadata = 'metadata',
}

export const SpanDetail = ({
  span,
  moduleName,
  className,
  spanConfig,
}: SpanDetailProps) => {
  const { custom_tags } = span;
  const { runtime, ...otherTags } = custom_tags || {};
  const responseApiCache = useResponseApiCacheStore(state => state.cache);

  const finalRuntime = runtime ?? span.system_tags?.runtime;
  const overviewFields = useSpanOverviewField(span);
  const extraFields = useMemo(() => {
    const extraTagList = spanConfig?.extraTagList;
    if (!extraTagList) {
      return [];
    }
    return extraTagList.map(tag => ({
      key: tag.title,
      title: tag.title,
      item: typeof tag.item === 'function' ? tag.item(span) : tag.item,
      enableCopy: tag.enableCopy,
      width: 224,
    }));
  }, [spanConfig?.extraTagList, span]);
  const spanContentList = useMemo(() => getSpanContentField(span), [span]);
  const {
    extraSpanDetailTabs,
    defaultActiveTabKey,
    platformType,
    customParams,
  } = useTraceDetailContext();
  const {
    showTags = true,
    baseInfoPosition = 'right',
    maxColNum,
    minColWidth,
  } = spanConfig ?? {};
  const { t } = useLocale();
  const actualDefaultActiveTabKey = useMemo(() => {
    const targetTabs = extraSpanDetailTabs?.find(
      tab => tab.tabKey === defaultActiveTabKey,
    );
    if (
      !targetTabs ||
      (!isFunction(targetTabs.visible) && !targetTabs.visible) ||
      (isFunction(targetTabs.visible) &&
        !(targetTabs.visible as Function)(span))
    ) {
      return TabKey.Run;
    }
    return defaultActiveTabKey;
  }, [defaultActiveTabKey, extraSpanDetailTabs, span]);

  return (
    <div className={cs(className, styles.container)}>
      <SpanDetailHeader span={span} />
      <Tabs className={styles.tab} defaultActiveKey={actualDefaultActiveTabKey}>
        <Tabs.TabPane tab={t('analytics_trace_run')} itemKey={TabKey.Run}>
          <Row className={styles['tab-content']}>
            <Col span={baseInfoPosition === 'top' ? 24 : 19}>
              {spanContentList?.length > 0 ? (
                <>
                  {baseInfoPosition === 'top' && (
                    <SpanFieldList
                      fields={overviewFields.concat(extraFields)}
                      span={span}
                      maxColNum={maxColNum}
                      minColWidth={minColWidth}
                      layout="horizontal"
                    />
                  )}
                  <div className="flex flex-col">
                    <TraceStructData
                      span={span}
                      spanRenderConfig={customParams?.spanRenderConfig}
                      responseApiCache={responseApiCache}
                    />
                  </div>
                </>
              ) : (
                <div className="flex items-center justify-center h-full w-full mt-[150px]">
                  <Empty
                    image={
                      <IconCozIllusEmpty style={{ width: 150, height: 150 }} />
                    }
                    title={t('reported_data_not_found')}
                    description={t('report_in_sdk')}
                  />
                </div>
              )}
            </Col>
            {baseInfoPosition === 'right' ? (
              <Col span={5} className={styles['span-detail']}>
                <SpanFieldList
                  fields={overviewFields.concat(extraFields)}
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
                  title={t('analytics_trace_metadata')}
                  hasBottomLine={false}
                  copyConfig={{
                    moduleName,
                    point: 'meta_data',
                  }}
                  span={span}
                  hideSwitchRawType
                  children={(_renderType, content) => (
                    <RawContent
                      structuredContent={content}
                      span={span}
                      enabledValuesTypes={[]}
                    />
                  )}
                />
              </>
            )}
            {finalRuntime ? (
              <SpanContentContainer
                content={finalRuntime}
                title={t('analytics_trace_runtime')}
                hasBottomLine={false}
                copyConfig={{
                  moduleName,
                  point: 'runtime',
                }}
                span={span}
                hideSwitchRawType
                children={(_renderType, content) => (
                  <RawContent
                    structuredContent={content}
                    span={span}
                    enabledValuesTypes={[]}
                  />
                )}
              />
            ) : null}
          </Tabs.TabPane>
        ) : null}

        {extraSpanDetailTabs
          ?.filter(tab =>
            isFunction(tab.visible)
              ? (tab.visible as Function)(span)
              : (tab.visible ?? true),
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
