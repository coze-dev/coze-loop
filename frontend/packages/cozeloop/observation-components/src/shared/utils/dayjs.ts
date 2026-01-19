// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import dayjsUTC from 'dayjs/plugin/utc';
import dayjsTimezone from 'dayjs/plugin/timezone';
import quartersOfYear from 'dayjs/plugin/quarterOfYear';
import isoWeek from 'dayjs/plugin/isoWeek';
import dayjs, { type ConfigType, type Dayjs } from 'dayjs';

dayjs.extend(isoWeek); // 注意：这里插件的注册顺序不能随意改变，此外重复注册插件可能会有 bug。
dayjs.extend(quartersOfYear);
dayjs.extend(dayjsUTC);
dayjs.extend(dayjsTimezone);

const dayJsTimeZone = (param?: ConfigType, timeZone?: string): Dayjs => {
  if (!timeZone) {
    return dayjs(param);
  }
  return dayjs(param).tz(timeZone);
};

export default dayjs;
export { dayJsTimeZone };
