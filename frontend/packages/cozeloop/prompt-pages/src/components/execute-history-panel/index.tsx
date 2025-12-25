// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */

import { useEffect, useRef, useState } from 'react';

import { Resizable } from 're-resizable';
import classNames from 'classnames';
import { useInfiniteScroll } from 'ahooks';
import { getEndTime, getStartTime } from '@cozeloop/observation-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { ResizeSidesheet } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type DebugLog } from '@cozeloop/api-schema/prompt';
import { promptDebug } from '@cozeloop/api-schema';
import {
  IconCozIllusEmpty,
  IconCozIllusEmptyDark,
} from '@coze-arch/coze-design/illustrations';
import { Button, Empty, List, Spin } from '@coze-arch/coze-design';

import { TraceTab } from '../trace-tabs';
import { QueryItem, Status } from './query-item';

interface HistoryPanelProps {
  visible?: boolean;
  promptID?: string;
  onCancel?: () => void;
  onRevert?: () => void;
}

export const ExecuteHistoryPanel = ({
  promptID,
  visible,
  onCancel,
}: HistoryPanelProps) => {
  const dom = useRef<HTMLDivElement>(null);
  const [selectedItem, setSelectedItem] = useState<DebugLog | undefined>();

  const { spaceID } = useSpace();
  const containerRef = useRef<HTMLDivElement>(null);
  const firstLoad = useRef(true);

  const {
    loading,
    loadMore,
    data,
    reloadAsync: fetchDebugHistoryList,
    loadingMore,
    noMore,
  } = useInfiniteScroll<{
    list: Array<DebugLog>;
    has_more?: boolean;
    next_cursor?: string;
  }>(
    async dataSource => {
      if (!promptID || !spaceID) {
        return {
          list: [],
          has_more: false,
          next_cursor: undefined,
        };
      }
      const { next_page_token, has_more, debug_history } =
        await promptDebug.ListDebugHistory({
          prompt_id: promptID,
          workspace_id: spaceID,
          page_token: dataSource?.next_cursor,
          days_limit: 7,
          page_size: 20,
        });
      if (firstLoad.current) {
        setSelectedItem(debug_history?.[0]);
        firstLoad.current = false;
      }
      return {
        list: debug_history ?? [],
        has_more: has_more ?? false,
        next_cursor: next_page_token ?? undefined,
      };
    },
    {
      target: containerRef,
      isNoMore: d => !d?.has_more,
      manual: true,
      reloadDeps: [promptID, spaceID],
    },
  );

  const renderList = () => (
    <div
      ref={containerRef}
      className="w-full h-full p-[16px] box-border border-0 border-r-2 border-semi-border border-solid overflow-auto styled-scrollbar !pr-[10px]"
    >
      {loading ? (
        <div className="flex items-center justify-center h-full w-full">
          <Spin size="small" />
        </div>
      ) : (
        <>
          <List
            dataSource={data?.list}
            className="w-full"
            renderItem={item => (
              <List.Item
                key={item?.debug_id}
                className={classNames(
                  'w-full rounded-[8px] cursor-pointer !border-0 hover:coz-mg-primary',
                  {
                    '!coz-mg-primary':
                      selectedItem && item?.debug_id === selectedItem.debug_id,
                  },
                )}
                onClick={() => {
                  setSelectedItem(item);
                }}
              >
                <QueryItem
                  status={
                    item.status_code === 0 ? Status.Success : Status.Failed
                  }
                  createdTime={Number(item.started_at || 0)}
                  duration={
                    item.output_tokens || item.input_tokens
                      ? Number(item.output_tokens || 0) +
                        Number(item.input_tokens || 0)
                      : '-'
                  }
                  debug_id={item.debug_id}
                  costMs={item.cost_ms}
                  className="w-full"
                />
              </List.Item>
            )}
          />

          <div className="mt-[8px] flex justify-center">
            {!noMore && (
              <Button
                onClick={loadMore}
                loading={loadingMore}
                size="small"
                color="secondary"
              >
                {loadingMore
                  ? I18n.t('prompt_loading_status')
                  : I18n.t('load_more')}
              </Button>
            )}
          </div>
        </>
      )}
    </div>
  );

  const renderContent = () =>
    !selectedItem ? (
      <div className="w-full h-full flex items-center justify-center">
        <Empty
          image={<IconCozIllusEmpty width="160" height="160" />}
          darkModeImage={<IconCozIllusEmptyDark width="160" height="160" />}
          description={I18n.t('no_debug_record')}
        />
      </div>
    ) : (
      <TraceTab
        layout="vertical"
        debugID={selectedItem?.debug_id}
        startTime={getStartTime(selectedItem?.started_at || 1)}
        endTime={getEndTime(
          selectedItem?.ended_at || Date.now(),
          selectedItem?.cost_ms || 0,
        )}
        className="h-full"
      />
    );

  useEffect(() => {
    if (visible) {
      fetchDebugHistoryList();
    } else {
      firstLoad.current = true;
    }
    setSelectedItem(undefined);
  }, [visible]);

  return (
    <ResizeSidesheet
      visible={visible}
      onCancel={onCancel}
      dragOptions={{
        defaultWidth: 1200,
        maxWidth: 1600,
        minWidth: 1000,
      }}
      bodyStyle={{
        padding: '0 0 8px',
        borderTop: '1px solid var(--semi-color-border)',
        overflowY: 'hidden',
      }}
      title={I18n.t('debug_history')}
      zIndex={9}
    >
      <div className="flex w-full h-full" ref={dom}>
        <Resizable
          minWidth={'200px'}
          maxWidth={'420px'}
          defaultSize={{ width: '380px', height: '100%' }}
          className="h-full"
          enable={{ right: true }}
        >
          {renderList()}
        </Resizable>
        <div className="flex-1 min-w-0 overflow-auto styled-scrollbar !pr-[6px]">
          {renderContent()}
        </div>
      </div>
    </ResizeSidesheet>
  );
};
