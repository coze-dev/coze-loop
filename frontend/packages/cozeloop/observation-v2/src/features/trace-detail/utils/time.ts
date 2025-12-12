import { dayJsTimeZone } from '@/shared/utils/dayjs';

const offsetTime = 24;

export const getEndTime = (
  startTime: number | string,
  latency: number | string,
  timeZone?: string,
) =>
  dayJsTimeZone(Number(startTime), timeZone)
    .add(Number(latency) + 1000, 'millisecond')
    .add(offsetTime, 'hours')
    .valueOf()
    .toString();

export const getStartTime = (startTime: number | string, timeZone?: string) =>
  dayJsTimeZone(Number(startTime), timeZone)
    .subtract(offsetTime, 'hours')
    .valueOf()
    .toString();
