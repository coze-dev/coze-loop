// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type FC, type ReactNode } from 'react';

import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';

export enum EvaluationDetailPageTabKey {
  EVAL = 'eval',
  EXPERIMENT = 'experiment',
}

/**
 * 评测集详情页 tab 配置
 */
export interface EvaluateDetailPageTabConfig {
  /** 作为 Tabs.TabPane 的 `itemKey` */
  tabKey: EvaluationDetailPageTabKey;
  /** 作为 Tabs.TabPane 的 `tab` */
  tabName: ReactNode;
  /** 作为 Tabs.TabPane 的 `children` */
  children: ReactNode;
}

export type EvaluationDetailPageTabsType = FC<{
  spaceId: string;
  /** 评测集详情 */
  evaluationSet?: EvaluationSet;
  tabConfigs: Array<EvaluateDetailPageTabConfig>;
}>;
