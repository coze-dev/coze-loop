// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactJsonViewProps } from 'react-json-view';

export const jsonViewerConfig: Partial<ReactJsonViewProps> = {
  name: false,
  displayDataTypes: false,
  indentWidth: 2,
  iconStyle: 'triangle',
  enableClipboard: false,
  collapsed: 5,
  collapseStringsAfterLength: 300,
};
