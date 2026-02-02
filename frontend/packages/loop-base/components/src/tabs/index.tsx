// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cs from 'classnames';
import { Tabs, type TabsProps } from '@coze-arch/coze-design';

import styles from './index.module.less';

export const LoopTabs = (props: TabsProps) => {
  const { className, ...rest } = props;
  return <Tabs {...rest} className={cs(styles.tabs, className)} />;
};
