// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import DemoSpaceIcon from '../assets/demo-space-icon.svg';
const BOE_DEMO_SPACE_ID = '7476830560543850540';

const ONLINE_DEMO_SPACE_ID = '7487806534651887643';

export const DEMO_SPACE_ID = IS_RELEASE_VERSION
  ? ONLINE_DEMO_SPACE_ID
  : BOE_DEMO_SPACE_ID;

export const demoSpace = {
  id: DEMO_SPACE_ID,
  name: 'Demo 空间',
  icon_url: DemoSpaceIcon,
};

/** 是否禁用多模态评测 */
export const IS_DISABLED_MULTI_MODEL_EVAL = true as boolean;
/** 是否隐藏实验详情筛选, 临时 */
export const IS_HIDDEN_EXPERIMENT_DETAIL_FILTER = true as boolean;
