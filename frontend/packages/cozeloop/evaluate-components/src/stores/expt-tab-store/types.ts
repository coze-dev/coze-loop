// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Experiment } from '@cozeloop/api-schema/evaluation';

export interface ExptTabDefinition {
  name: string | React.JSX.Element;
  /** tab 的 itemKey */
  type: string;
  /** 评测对象描述 */
  tabComponent?: (props: {
    experiment: Experiment;
  }) => React.JSX.Element | React.ReactNode;
  /** 评测对象描述 */
  nameTag?: React.JSX.Element | React.ReactNode;
}
