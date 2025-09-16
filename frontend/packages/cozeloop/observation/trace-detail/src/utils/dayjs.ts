// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import dayjsUTC from 'dayjs/plugin/utc';
import dayjsTimezone from 'dayjs/plugin/timezone';
import quartersOfYear from 'dayjs/plugin/quarterOfYear';
import isoWeek from 'dayjs/plugin/isoWeek';
import dayjs from 'dayjs';
import { type ConfigType, type Dayjs } from 'dayjs';
import { envs } from '@cozeloop/env-adapter';

const IS_OVERSEA = envs.isOversea;

// 本地测试时候使用
dayjs.extend(isoWeek); // 注意：这里插件的注册顺序不能随意改变，此外重复注册插件可能会有 bug。
dayjs.extend(quartersOfYear);
dayjs.extend(dayjsUTC);
dayjs.extend(dayjsTimezone);

export const CURRENT_TIMEZONE = IS_OVERSEA ? 'UTC' : 'Asia/Shanghai';
export const CURRENT_TIMEZONE_OFFSET_LABEL = IS_OVERSEA
  ? 'UTC+00:00'
  : 'UTC+08:00';
const dayJsTimeZone = (param?: ConfigType): Dayjs => {
  if (IS_OVERSEA) {
    return dayjs.utc(param);
  }
  return dayjs(param).tz(CURRENT_TIMEZONE);
};

export default dayJsTimeZone;

export { Dayjs, type QUnitType } from 'dayjs';
