// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { LoopTabs } from '@cozeloop/components';
import { type EvaluationDetailPageTabsType } from '@cozeloop/adapter-interfaces/evaluate';
import { Tabs } from '@coze-arch/coze-design';

export enum EvaluationDetailPageTabKey {
  EVAL = 'eval',
  EXPERIMENT = 'experiment',
}

/**
 * 评测集详情页 tab 适配器
 *
 * 为了使评测集详情页在不同环境下能有不同 tab，因此引入该适配器
 *
 * 理想情况应该将所有 tab 配置写在不同适配器内部，根据不同环境调用不同适配器即可，但这种方案需要对现有代码（主要是 pkg 分包/依赖）造成巨大改动，在目前极限倒排条件下难以确保质量。
 * 因此先将原先共有的 tab 从外部传入，适配器内部仅添加不同环境下新增的 tab。
 */
export const EvaluationDetailPageTabs: EvaluationDetailPageTabsType = ({
  tabConfigs,
}) => {
  const [activeTab, setActiveTab] = useState<EvaluationDetailPageTabKey>(
    EvaluationDetailPageTabKey.EVAL,
  );
  return (
    <LoopTabs
      lazyRender
      className="flex-1 mt-4 overflow-hidden w-full"
      type="card"
      activeKey={activeTab}
      onChange={key => setActiveTab(key as EvaluationDetailPageTabKey)}
    >
      {tabConfigs.map(config => (
        <Tabs.TabPane
          itemKey={config.tabKey}
          tab={config.tabName}
          key={config.tabKey}
        >
          {config.children}
        </Tabs.TabPane>
      ))}
    </LoopTabs>
  );
};
