// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { type TabsProps, Tabs as SemiTabs } from '@coze-arch/coze-design';

import styles from './index.module.less';

export function Tabs({ className, ...props }: TabsProps) {
  return <SemiTabs className={classNames(styles.tabs, className)} {...props} />;
}

Tabs.TabPane = SemiTabs.TabPane;
Tabs.TabItem = SemiTabs.TabItem;
