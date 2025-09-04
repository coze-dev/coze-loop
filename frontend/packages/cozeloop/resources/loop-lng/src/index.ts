// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable import/order -- skip*/
// auth
import authLocaleZhCN from './locales/auth/zh-CN.json';
import authLocaleEnUS from './locales/auth/en-US.json';

// base
import baseLocaleZhCN from './locales/base/zh-CN.json';
import baseLocaleEnUS from './locales/base/en-US.json';

// common
import commonLocaleZhCN from './locales/common/zh-CN.json';
import commonLocaleEnUS from './locales/common/en-US.json';

// components
import componentsLocaleZhCN from './locales/components/zh-CN.json';
import componentsLocaleEnUS from './locales/components/en-US.json';

// evaluate
import evaluateLocaleZhCN from './locales/evaluate/zh-CN.json';
import evaluateLocaleEnUS from './locales/evaluate/en-US.json';

// observation
import observationLocaleZhCN from './locales/observation/zh-CN.json';
import observationLocaleEnUS from './locales/observation/en-US.json';

// prompt
import promptLocaleZhCN from './locales/prompt/zh-CN.json';
import promptLocaleEnUS from './locales/prompt/en-US.json';

// tag
import tagLocaleZhCN from './locales/tag/zh-CN.json';
import tagLocaleEnUS from './locales/tag/en-US.json';

export const localeZhCN = Object.assign(
  {},
  commonLocaleZhCN,
  componentsLocaleZhCN,
  baseLocaleZhCN,
  authLocaleZhCN,
  promptLocaleZhCN,
  evaluateLocaleZhCN,
  observationLocaleZhCN,
  tagLocaleZhCN,
);

export const localeEnUS = Object.assign(
  {},
  commonLocaleEnUS,
  componentsLocaleEnUS,
  baseLocaleEnUS,
  authLocaleEnUS,
  promptLocaleEnUS,
  evaluateLocaleEnUS,
  observationLocaleEnUS,
  tagLocaleEnUS,
);
