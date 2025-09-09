// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useShallow } from 'zustand/react/shallow';
import { IconCozCross } from '@coze-arch/coze-design/icons';
import {
  IconButton,
  Loading,
  Skeleton,
  Typography,
} from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';

import { VersionList } from '../version-list';
import { TraceTab } from '../trace-tabs';
import { PromptHeader } from '../prompt-header';
import { ExecuteHistoryPanel } from '../execute-history-panel';
import { NormalArea } from './normal-area';
import { CompareArea } from './compare-area';

interface PromptDevProps {
  getPromptLoading?: boolean;
  readonly?: boolean;
}

export function PromptDev({ getPromptLoading }: PromptDevProps) {
  const {
    versionChangeLoading,
    versionChangeVisible,
    setVersionChangeVisible,
    debugId,
    setDebugId,
    executeHistoryVisible,
    setExecuteHistoryVisible,
  } = useBasicStore(
    useShallow(state => ({
      versionChangeLoading: state.versionChangeLoading,
      versionChangeVisible: state.versionChangeVisible,
      setVersionChangeVisible: state.setVersionChangeVisible,
      debugId: state.debugId,
      setDebugId: state.setDebugId,
      executeHistoryVisible: state.executeHistoryVisible,
      setExecuteHistoryVisible: state.setExecuteHistoryVisible,
    })),
  );

  const { promptInfo } = usePromptStore(
    useShallow(state => ({ promptInfo: state.promptInfo })),
  );

  const { compareConfig } = usePromptMockDataStore(
    useShallow(state => ({
      compareConfig: state.compareConfig,
    })),
  );

  return (
    <Skeleton loading={getPromptLoading}>
      <div className="flex flex-col !h-full bg-transparent">
        {/* 顶部导航 */}
        <PromptHeader />
        {/* Main Content */}
        <div className="flex flex-1 overflow-hidden bg-white">
          <Loading
            className="flex-1 overflow-hidden !w-full !h-full"
            loading={Boolean(versionChangeLoading)}
            childStyle={{ height: '100%', overflow: 'hidden', display: 'flex' }}
          >
            {compareConfig?.groups?.length ? <CompareArea /> : <NormalArea />}
          </Loading>
          {versionChangeVisible ? (
            <div className="w-[360px] flex flex-col flex-shrink-0 border-0 border-l border-solid">
              <div
                className="h-[40px] px-6 py-2 box-border coz-fg-plus w-full flex justify-between items-center border-0 border-r border-t border-b border-solid"
                style={{ background: '#F6F6FB' }}
              >
                <Typography.Text strong>版本记录</Typography.Text>
                <IconButton
                  icon={<IconCozCross />}
                  color="secondary"
                  size="small"
                  onClick={() => setVersionChangeVisible(false)}
                />
              </div>
              <VersionList />
            </div>
          ) : null}
        </div>
      </div>
      <TraceTab
        displayType="drawer"
        debugID={debugId}
        drawerVisible={Boolean(debugId)}
        drawerClose={() => {
          setDebugId(undefined);
        }}
      />
      <ExecuteHistoryPanel
        promptID={promptInfo?.id}
        visible={executeHistoryVisible}
        onCancel={() => setExecuteHistoryVisible(false)}
      />
    </Skeleton>
  );
}
