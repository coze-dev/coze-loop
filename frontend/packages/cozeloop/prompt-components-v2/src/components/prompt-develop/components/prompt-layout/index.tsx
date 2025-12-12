/* eslint-disable complexity */
import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { LoopTabs } from '@cozeloop/components';
import { PromptType } from '@cozeloop/api-schema/prompt';
import { IconCozCross } from '@coze-arch/coze-design/icons';
import { IconButton, Loading, Skeleton, Tabs } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { VersionList } from '@/components/version-list';
import { PromptDevLayout } from '@/components/prompt-dev-layout';

import { usePromptDevProviderContext } from '../prompt-provider';
import { PromptHeader } from '../prompt-header';
import { SnippetArea } from '../prompt-content/snippet-area';
import { NormalArea } from '../prompt-content/normal-area';
import { CompareArea } from '../prompt-content/compare-area';
import { type PromptLayoutProps } from '../../type';

export function PromptLayout({
  getPromptLoading,
  wrapperClassName,
}: PromptLayoutProps) {
  const {
    isPlayground,
    extraTabs = [],
    activeTab,
    tabsChange,
  } = usePromptDevProviderContext();
  const { promptInfo } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
    })),
  );
  const { compareConfig } = usePromptMockDataStore(
    useShallow(state => ({
      compareConfig: state.compareConfig,
    })),
  );
  const {
    versionChangeLoading,
    versionChangeVisible,
    setVersionChangeVisible,
  } = useBasicStore(
    useShallow(state => ({
      versionChangeLoading: state.versionChangeLoading,
      versionChangeVisible: state.versionChangeVisible,
      setVersionChangeVisible: state.setVersionChangeVisible,
    })),
  );

  const isCompareDev = compareConfig?.groups?.length;
  const isSnippet =
    promptInfo?.prompt_basic?.prompt_type === PromptType.Snippet;

  const showTabs =
    extraTabs.length > 0 && !isCompareDev && !isPlayground && !isSnippet;
  return (
    <Skeleton loading={getPromptLoading}>
      <div
        className={classNames(
          'flex flex-col !h-full bg-transparent',
          wrapperClassName,
        )}
      >
        {/* 顶部导航 */}
        <PromptHeader />
        {showTabs ? (
          <LoopTabs
            className="bg-[#FCFCFF] pt-3"
            type="card"
            activeKey={activeTab || 'dev'}
            onChange={tabsChange}
          >
            <Tabs.TabPane
              itemKey="dev"
              tab={I18n.t('orchestration')}
            ></Tabs.TabPane>
            {extraTabs?.map(tab => (
              <Tabs.TabPane itemKey={tab.key} tab={tab.title} key={tab.key} />
            ))}
          </LoopTabs>
        ) : null}
        {!activeTab || activeTab === 'dev' || isCompareDev ? (
          <div className="flex flex-1 overflow-hidden bg-[#FCFCFF]">
            <Loading
              className="flex-1 overflow-hidden !w-full !h-full"
              loading={Boolean(versionChangeLoading)}
              childStyle={{
                height: '100%',
                overflow: 'hidden',
                display: 'flex',
              }}
            >
              {isSnippet ? (
                <SnippetArea />
              ) : isCompareDev ? (
                <CompareArea />
              ) : (
                <NormalArea />
              )}
            </Loading>
            {versionChangeVisible ? (
              <PromptDevLayout
                className="!w-[360px] flex-shrink-0 border-0 border-l border-solid"
                wrapperClassName={showTabs ? '!border-t-0' : ''}
                title={I18n.t('version_record')}
                actionBtns={
                  <IconButton
                    icon={<IconCozCross />}
                    color="secondary"
                    size="small"
                    onClick={() => setVersionChangeVisible(false)}
                  />
                }
              >
                <VersionList />
              </PromptDevLayout>
            ) : null}
          </div>
        ) : null}
        {isCompareDev ? null : (
          <>
            {extraTabs?.map(tab =>
              tab.key === activeTab ? tab.children : null,
            )}
          </>
        )}
      </div>
    </Skeleton>
  );
}
