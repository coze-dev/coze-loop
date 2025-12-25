// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as common from './common';
export { common };
export interface View {
  id: string,
  enterprise_id?: string,
  workspace_id?: string,
  view_name: string,
  platform_type?: common.PlatformType,
  spanList_type?: common.SpanListType,
  filters: string,
  is_system: boolean,
}