/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable @typescript-eslint/no-magic-numbers */

import dayjs from 'dayjs';

import { i18nService } from '@/i18n';

function getDateAtZeroClock(date: Date) {
  return new Date(date.setHours(0, 0, 0, 0));
}

function getDayjsStartOfToday() {
  return dayjs().startOf('day');
}
function getDayjsEndOfYesterday() {
  return dayjs().subtract(1, 'day').endOf('day').millisecond(0);
}

export enum PresetRange {
  Unset = 'unset',
  Min5 = '5m',
  Min15 = '15m',
  Min30 = '30m',
  Hour1 = '1h',
  Hour2 = '2h',
  Hour3 = '3h',
  Hour6 = '6h',
  Hour12 = '12h',
  Hour24 = '24h',
  Day1 = '1d',
  Day2 = '2d',
  Day3 = '3d',
  Day5 = '5d',
  Day7 = '7d',
  Day15 = '15d',
  Day30 = '30d',
  Day90 = '90d',
  Day180 = '180d',
  Day365 = '365d',
  Month1 = '1mo',
  Month3 = '3mo',
  Yesterday = '~y',
  BeforeYesterday = '~b',
  DayToNow = '~d',
  WeekToNow = '~w',
  MonthToNow = '~m',
  YesterdayExcludeToday = '~yet',
  PastWeek = '~pw',
  PastMonth = '~pm',
  PastYear = '~py',
  AllTime = 'all',
}

export const getTimePickerPresets = () => ({
  [PresetRange.Unset]: {
    text: i18nService.t('customize'),
    start: () => new Date(),
    end: () => new Date(),
  },
  [PresetRange.Min5]: {
    text: i18nService.t('observation_time_last_minutes', {
      count: 5,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 60 * 5),
    end: () => new Date(),
  },
  [PresetRange.Min15]: {
    text: i18nService.t('observation_time_last_minutes', {
      count: 15,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 60 * 15),
    end: () => new Date(),
  },
  [PresetRange.Min30]: {
    text: i18nService.t('observation_time_last_minutes', {
      count: 30,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 60 * 30),
    end: () => new Date(),
  },
  [PresetRange.Hour1]: {
    text: i18nService.t('observation_time_last_hours', {
      count: 1,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 1),
    end: () => new Date(),
  },
  [PresetRange.Hour2]: {
    text: i18nService.t('observation_time_last_hours', {
      count: 2,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 2),
    end: () => new Date(),
  },
  [PresetRange.Hour3]: {
    text: i18nService.t('observation_time_last_hours', {
      count: 3,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 3),
    end: () => new Date(),
  },
  [PresetRange.Hour6]: {
    text: i18nService.t('observation_time_last_hours', {
      count: 6,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 6),
    end: () => new Date(),
  },
  [PresetRange.Hour12]: {
    text: i18nService.t('observation_time_last_hours', {
      count: 12,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 12),
    end: () => new Date(),
  },
  [PresetRange.Hour24]: {
    text: i18nService.t('observation_time_last_hours', {
      count: 24,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 24),
    end: () => new Date(),
  },
  [PresetRange.Day1]: {
    text: i18nService.t('observation_time_last_days', {
      count: 1,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 24),
    end: () => new Date(),
  },
  [PresetRange.Day2]: {
    text: i18nService.t('observation_time_last_days', {
      count: 2,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 24 * 2),
    end: () => new Date(),
  },
  [PresetRange.Day3]: {
    text: i18nService.t('observation_time_last_days', {
      count: 3,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 24 * 3),
    end: () => new Date(),
  },
  [PresetRange.Day5]: {
    text: i18nService.t('observation_time_last_days', {
      count: 5,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 24 * 5),
    end: () => new Date(),
  },
  [PresetRange.Day7]: {
    text: i18nService.t('observation_time_last_days', {
      count: 7,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 24 * 7),
    end: () => new Date(),
  },
  [PresetRange.Day15]: {
    text: i18nService.t('observation_time_last_days', {
      count: 15,
    }),
    start: () => new Date(new Date().valueOf() - 1000 * 3600 * 24 * 15),
    end: () => new Date(),
  },
  [PresetRange.Month1]: {
    text: i18nService.t('observation_time_past_month'),
    start: () => dayjs().subtract(1, 'month').toDate(),
    end: () => dayjs().toDate(),
  },
  [PresetRange.Month3]: {
    text: i18nService.t('observation_time_past_3_months'),
    start: () => dayjs().subtract(3, 'month').toDate(),
    end: () => dayjs().toDate(),
  },
  [PresetRange.DayToNow]: {
    text: i18nService.t('observation_time_today_so_far'),
    start: () => getDateAtZeroClock(new Date()),
    end: () => new Date(),
  },
  [PresetRange.Yesterday]: {
    text: i18nService.t('observation_time_yesterday'),
    start: () =>
      getDateAtZeroClock(new Date(new Date().valueOf() - 1000 * 3600 * 24)),
    end: () => getDateAtZeroClock(new Date()),
  },
  [PresetRange.BeforeYesterday]: {
    text: i18nService.t('observation_time_day_before_yesterday'),
    start: () =>
      getDateAtZeroClock(new Date(new Date().valueOf() - 1000 * 3600 * 24 * 2)),
    end: () =>
      getDateAtZeroClock(new Date(new Date().valueOf() - 1000 * 3600 * 24)),
  },
  [PresetRange.WeekToNow]: {
    text: i18nService.t('observation_time_week_so_far'),
    start: () => {
      const now = new Date();
      const day = now.getDay();
      return getDateAtZeroClock(
        new Date(now.valueOf() - 1000 * 3600 * 24 * (day - 1)),
      );
    },
    end: () => new Date(),
  },
  [PresetRange.YesterdayExcludeToday]: {
    text: i18nService.t('observation_time_yesterday'),
    start: () =>
      getDateAtZeroClock(new Date(new Date().valueOf() - 1000 * 3600 * 24)),
    end: () => getDayjsEndOfYesterday().toDate(),
  },
  [PresetRange.PastWeek]: {
    text: i18nService.t('observation_time_past_week'),
    start: () => getDayjsStartOfToday().subtract(7, 'day').toDate(),
    end: () => getDayjsEndOfYesterday().toDate(),
  },
  [PresetRange.PastMonth]: {
    text: i18nService.t('observation_time_past_month'),
    start: () => getDayjsStartOfToday().subtract(1, 'month').toDate(),
    end: () => getDayjsEndOfYesterday().toDate(),
  },
  [PresetRange.PastYear]: {
    text: i18nService.t('observation_time_past_year'),
    start: () => getDayjsStartOfToday().subtract(1, 'year').toDate(),
    end: () => getDayjsEndOfYesterday().toDate(),
  },

  [PresetRange.MonthToNow]: {
    text: i18nService.t('observation_range_this_month'),
    start: () => dayjs().startOf('month').toDate(),
    end: () => dayjs().toDate(),
  },
  [PresetRange.Day30]: {
    text: i18nService.t('observation_time_last_days', {
      count: 30,
    }),
    start: () => dayjs().subtract(30, 'd').toDate(),
    end: () => dayjs().toDate(),
  },
  [PresetRange.Day90]: {
    text: i18nService.t('observation_time_last_days', {
      count: 90,
    }),
    start: () => dayjs().subtract(90, 'd').toDate(),
    end: () => dayjs().toDate(),
  },
  [PresetRange.Day180]: {
    text: i18nService.t('observation_time_last_days', {
      count: 180,
    }),
    start: () => dayjs().subtract(180, 'd').toDate(),
    end: () => dayjs().toDate(),
  },
  [PresetRange.Day365]: {
    text: i18nService.t('observation_time_last_days', {
      count: 365,
    }),
    start: () => dayjs().subtract(365, 'd').toDate(),
    end: () => dayjs().toDate(),
  },
  [PresetRange.AllTime]: {
    text: i18nService.t('all_time'),
    start: () => dayjs().subtract(365, 'd').toDate(),
    end: () => dayjs().toDate(),
  },
});

export const YEAR_DAY_COUNT = 364;
export const MAX_DAY_COUNT = 7;

export const PRESET_CAN_NOT_REFRESH = [
  PresetRange.Unset,
  PresetRange.Yesterday,
  PresetRange.BeforeYesterday,
];

export const PERFORMANCE_PRESETS_LIST = [
  PresetRange.Unset,
  PresetRange.Min5,
  PresetRange.Min15,
  PresetRange.Min30,
  PresetRange.Hour1,
  PresetRange.Hour2,
  PresetRange.Hour3,
  PresetRange.Hour6,
  PresetRange.Hour12,
  PresetRange.Hour24,
  PresetRange.Day2,
  PresetRange.Day3,
  PresetRange.Day5,
  PresetRange.Day7,
  PresetRange.Yesterday,
  PresetRange.BeforeYesterday,
  PresetRange.DayToNow,
  PresetRange.WeekToNow,
];

export const OVERVIEW_PRESETS_LIST = [
  PresetRange.Unset,
  PresetRange.YesterdayExcludeToday,
  PresetRange.PastWeek,
  PresetRange.PastMonth,
  PresetRange.AllTime,
];

export const TRACE_PRESETS_LIST = [
  PresetRange.Unset,
  PresetRange.Min5,
  PresetRange.Hour1,
  PresetRange.Hour24,
  PresetRange.Yesterday,
  PresetRange.Day3,
  PresetRange.Day7,
  PresetRange.DayToNow,
  PresetRange.WeekToNow,
  PresetRange.AllTime,
];

export const THREAD_PRESETS_LIST = [
  PresetRange.Unset,
  PresetRange.Min5,
  PresetRange.Hour1,
  PresetRange.Day3,
  PresetRange.Day7,
  PresetRange.Month1,
  PresetRange.Month3,
  PresetRange.DayToNow,
];

export const generatePickerOptions = (presetRanges: PresetRange[]) =>
  presetRanges.map(preset => {
    const { text } = getTimePickerPresets()[preset];
    return {
      value: preset,
      label: text,
    };
  });

export const performanceDatePickerOptions = generatePickerOptions(
  PERFORMANCE_PRESETS_LIST,
);

export const overviewDatePickerOptions = generatePickerOptions(
  OVERVIEW_PRESETS_LIST,
);

export const threadDatePickerOptions =
  generatePickerOptions(THREAD_PRESETS_LIST);

export const METRICS_PRESETS_LIST = [
  PresetRange.Unset,
  PresetRange.Hour1,
  PresetRange.Hour12,
  PresetRange.DayToNow,

  PresetRange.Yesterday,
  PresetRange.Day7,
  PresetRange.WeekToNow,
  PresetRange.MonthToNow,

  PresetRange.Day30,
];

// 统计个人免费版
export const METRICS_FREE_PRESETS_LIST = [
  PresetRange.Hour1,
  PresetRange.Hour3,
  PresetRange.Day1,
  PresetRange.Day3,
  PresetRange.Day7,
  PresetRange.Day30,
  PresetRange.Day180,
  PresetRange.AllTime,
  PresetRange.Unset,
];

export const metricsDatePickerOptions = generatePickerOptions(
  METRICS_FREE_PRESETS_LIST,
);

export function calcPresetTime(selectedPreset: PresetRange) {
  const preset =
    selectedPreset && selectedPreset !== PresetRange.Unset
      ? getTimePickerPresets()[selectedPreset]
      : undefined;
  if (preset) {
    return {
      startTime: Number(
        typeof preset.start === 'function' ? preset.start() : preset.start,
      ),
      endTime: Number(
        typeof preset.end === 'function' ? preset.end() : preset.end,
      ),
    };
  }
  return undefined;
}

export const THREAD_MAX_DAY_COUNT = 92;

// 免费版
export const TRACE_FREE_PRESETS_LIST = [
  PresetRange.Hour1,
  PresetRange.Hour3,
  PresetRange.Day1,
  PresetRange.Day3,
  PresetRange.Day7,
  PresetRange.Unset,
];

// 专业版
export const TRACE_PRO_PRESETS_LIST = [
  PresetRange.Hour1,
  PresetRange.Hour3,
  PresetRange.Day1,
  PresetRange.Day3,
  PresetRange.Day7,
  PresetRange.Day90,
  PresetRange.Unset,
];

// 个人旗舰版
export const TRACE_PERSONAL_PRESETS_LIST = [
  PresetRange.Hour1,
  PresetRange.Hour3,
  PresetRange.Day1,
  PresetRange.Day3,
  PresetRange.Day7,
  PresetRange.Day90,
  PresetRange.Unset,
  PresetRange.AllTime,
];

// 团队版
export const TRACE_TEAM_PRESETS_LIST = [
  PresetRange.Hour1,
  PresetRange.Hour3,
  PresetRange.Day1,
  PresetRange.Day3,
  PresetRange.Day7,
  PresetRange.Day15,
  PresetRange.Day30,
  PresetRange.Day180,
  PresetRange.Unset,
  PresetRange.AllTime,
];

// 企业版
export const TRACE_ENTERPRISE_PRESETS_LIST = [
  PresetRange.Hour1,
  PresetRange.Hour3,
  PresetRange.Day1,
  PresetRange.Day3,
  PresetRange.Day7,
  PresetRange.Day15,
  PresetRange.Day30,
  PresetRange.Unset,
  PresetRange.AllTime,
];

export const traceDatePickerOptions = generatePickerOptions(
  TRACE_ENTERPRISE_PRESETS_LIST,
);
