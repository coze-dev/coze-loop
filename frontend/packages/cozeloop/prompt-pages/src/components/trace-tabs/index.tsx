/* eslint-disable @coze-arch/max-line-per-function */

import { useMemo, useState } from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import {
  NODE_CONFIG_MAP,
  TraceDetail,
  SpanType,
} from '@cozeloop/observation-component-adapter';
import { ResizeSidesheet, TextWithCopy } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  FieldType,
  PlatformType,
  QueryType,
} from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';
import {
  IconCozIllusEmpty,
  IconCozIllusEmptyDark,
} from '@coze-arch/coze-design/illustrations';
import { IconCozRefresh } from '@coze-arch/coze-design/icons';
import { Empty, Spin, TabPane, Tabs, Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

interface TraceTabProps {
  debugID?: Int64;
  startTime?: Int64;
  endTime?: Int64;
  layout?: 'horizontal' | 'vertical';
  className?: string;
  drawerVisible?: boolean;
  displayType?: 'drawer' | 'normal';
  drawerClose?: () => void;
}

export const TraceTab = ({
  debugID,
  startTime,
  endTime,
  layout,
  className,
  drawerVisible,
  displayType = 'normal',
  drawerClose,
}: TraceTabProps) => {
  const [activeTab, setActiveTab] = useState('0');
  const { spaceID, space: { name: spaceName = '' } = {} } = useSpace();
  const {
    data: spansItem,
    loading,
    runAsync,
  } = useRequest(
    async () => {
      const { spans } = await observabilityTrace.ListSpans({
        platform_type: PlatformType.Prompt,
        start_time: (startTime || 1).toString(),
        end_time: (endTime || Date.now()).toString(),
        workspace_id: spaceID,
        filters: {
          filter_fields: [
            {
              field_name: 'debug_id',
              field_type: FieldType.Long,
              values: [`${debugID || ''}`],
              query_type: QueryType.In,
            },
          ],
        },
        order_bys: [{ field: 'start_time', is_asc: false }],
        page_size: 30,
      });
      spans.reverse();
      setActiveTab('0');
      return spans;
    },
    { ready: Boolean(debugID), refreshDeps: [debugID] },
  );

  const isDrawer = displayType === 'drawer';

  const traceItemHeader = useMemo(
    () => (
      <div
        className={classNames(
          'flex gap-2 items-center py-4 px-5 border-0 border-solid border-[#EAEEF5]',
          {
            'border-b': layout === 'vertical',
          },
        )}
      >
        <span className="w-9 h-9">
          {NODE_CONFIG_MAP[SpanType.LLMCall].icon?.({
            className: '!w-[18px] !h-[18px]',
            size: 'large',
          })}
        </span>
        <Typography.Text strong className="text-[16px]">
          PromptExecutor
        </Typography.Text>
        {spansItem?.[Number(activeTab)]?.trace_id ? (
          <TextWithCopy
            style={{ lineHeight: '22px' }}
            displayText="Trace ID"
            content={spansItem[Number(activeTab)].trace_id}
            copyTooltipText="复制 Trace ID"
          />
        ) : null}
      </div>
    ),
    [activeTab, layout, spansItem],
  );
  const traceItem = useMemo(() => {
    if (loading) {
      return (
        <div className="w-full h-full flex items-center justify-center">
          <Spin />
        </div>
      );
    }

    if (!spansItem?.length) {
      return (
        <Empty
          className="h-full justify-center w-full"
          image={<IconCozIllusEmpty width="160" height="160" />}
          darkModeImage={<IconCozIllusEmptyDark width="160" height="160" />}
          description={
            <div className="flex flex-col">
              <Typography.Text
                type="tertiary"
                className="flex items-center gap-1"
              >
                Prompt调试数据在路上，请刷新重试
              </Typography.Text>
              <Typography.Text
                className="text-brand-9 cursor-pointer pt-3"
                icon={<IconCozRefresh className="text-secondary" />}
                onClick={runAsync}
              >
                刷新
              </Typography.Text>
            </div>
          }
        />
      );
    }
    return (
      <div className="flex flex-col h-full w-full">
        {isDrawer ? null : traceItemHeader}
        {(spansItem || []).length > 1 ? (
          <Tabs
            className={classNames(className, styles.tabs)}
            onChange={key => setActiveTab(key)}
          >
            {spansItem?.map((item, index) => (
              <TabPane
                tabIndex={index}
                tab={`第${index + 1}步`}
                key={item.span_id || index}
                itemKey={String(index)}
                className="px-5"
              >
                <TraceDetail
                  className="h-full"
                  spaceID={spaceID}
                  spaceName={spaceName}
                  searchType={'trace_id'}
                  moduleName="prompt_history_trace_list"
                  id={item.trace_id || ''}
                  startTime={item.started_at}
                  layout={layout}
                  hideTraceDetailHeader
                  platformType={PlatformType.Prompt}
                />
              </TabPane>
            ))}
          </Tabs>
        ) : (
          <div
            className={classNames('flex-1 w-full h-full flex overflow-hidden', {
              'pt-5 px-1': layout === 'vertical',
            })}
          >
            <TraceDetail
              className={className}
              spaceID={spaceID}
              spaceName={spaceName}
              searchType={'trace_id'}
              moduleName="prompt_history_trace_list"
              id={spansItem?.[0]?.trace_id || ''}
              startTime={spansItem?.[0]?.started_at}
              layout={layout}
              hideTraceDetailHeader
              platformType={PlatformType.Prompt}
            />
          </div>
        )}
      </div>
    );
  }, [
    loading,
    spansItem,
    isDrawer,
    traceItemHeader,
    className,
    layout,
    spaceID,
    spaceName,
    runAsync,
  ]);

  if (isDrawer) {
    return (
      <ResizeSidesheet
        visible={drawerVisible}
        title={
          <div className="flex justify-between items-center pr-5">
            {traceItemHeader}
          </div>
        }
        headerStyle={{ padding: '0 16px 0 0', border: 0 }}
        onCancel={drawerClose}
        dragOptions={{
          defaultWidth: 1200,
          maxWidth: 1600,
          minWidth: 1000,
        }}
        bodyStyle={{
          padding: '0 0 8px',
          overflowY: 'hidden',
          display: 'flex',
        }}
        zIndex={9}
        closable={false}
      >
        {traceItem}
      </ResizeSidesheet>
    );
  }

  return traceItem;
};
