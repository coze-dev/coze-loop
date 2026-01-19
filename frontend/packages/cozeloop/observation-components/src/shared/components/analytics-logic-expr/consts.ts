// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export enum TimeUnit {
  MS = 'ms',
}

export const LOGIC_OPERATOR_RECORDS: Partial<
  Record<
    string,
    {
      label: string;
    }
  >
> = {
  eq: {
    label: 'task_filter_eq',
  },
  not_eq: {
    label: 'task_filter_not_eq',
  },
  gt: {
    label: 'task_filter_gt',
  },
  lt: {
    label: 'task_filter_lt',
  },
  gte: {
    label: 'task_filter_gte',
  },
  lte: {
    label: 'task_filter_lte',
  },
  like: {
    label: 'task_filter_like',
  },
  match: {
    label: 'task_filter_like',
  },
  in: {
    label: 'task_filter_in',
  },
  not_in: {
    label: 'task_filter_not_in',
  },
  is_null: {
    label: 'task_filter_is_null',
  },
  not_exist: {
    label: 'task_filter_is_null',
  },
  exist: {
    label: 'task_filter_not_null',
  },
  not_null: {
    label: 'task_filter_not_null',
  },
  in_list: {
    label: 'task_filter_in',
  },
  not_match: {
    label: 'task_filter_not_match',
  },
};

export enum FilterFields {
  // key需要进行特化文案展示的字段
  BIZ_ID = 'biz_id',
  // 预设选项需要进行特化文案展示的字段
  STATUS_KEY = 'status',
  BOT_ENV_KEY = 'bot_env',
  QUERY_TYPE_KEY = 'query_type',
  // 需要进行单位展示的字段
  DURATION = 'duration',
  LATENCY = 'latency',
  LATENCY_FIRST_RESP = 'latency_first_resp',
  START_TIME_FIRST_RESP = 'start_time_first_resp',
  INPUT_TOKENS = 'input_token',
  OUTPUT_TOKENS = 'output_token',
  TOTAL_TOKENS = 'total_token',
  TOKENS = 'token',
  BOT_ID = 'bot_id',
  APP_ID = 'app_id',
  FEEDBACK = 'feedback_auto_evaluator',
  FEEDBACK_MANUAL = 'feedback_manual',
  FEEDBACK_COZE = 'feedback_coze',
  WORKFLOW_ID = 'workflow_id',
  ARK_BOT_ID = 'ark.bot_id',
  FEEDBACK_API = 'feedback_openapi',
  AGENT_RUNTIME_ID = 'cozeloop_agent_runtime_id',
}

export const SELECT_RENDER_CMP_OP_LIST = ['in_list'];

export const SELECT_MULTIPLE_RENDER_CMP_OP_LIST = ['in', 'not_in'];

export const EMPTY_RENDER_CMP_OP_LIST = [
  'is_null',
  'not_null',
  'not_exist',
  'exist',
];

export const NUMBER_RENDER_CMP_OP_LIST = [
  FilterFields.DURATION,
  FilterFields.LATENCY,
  FilterFields.LATENCY_FIRST_RESP,
  FilterFields.START_TIME_FIRST_RESP,
  FilterFields.INPUT_TOKENS,
  FilterFields.OUTPUT_TOKENS,
  FilterFields.TOTAL_TOKENS,
  FilterFields.TOKENS,
  FilterFields.FEEDBACK,
];

export const THREADS_STATUS_RECORDS: Partial<
  Record<
    string,
    {
      label: string;
    }
  >
> = {
  success: {
    label: 'observation_threads_options_success',
  },
  error: {
    label: 'observation_threads_options_fail',
  },
};

export const THREADS_FEEDBACK_COZE_RECORDS = {
  like: {
    label: 'like',
  },
  dislike: {
    label: 'dislike',
  },
};

export enum BotEnv {
  DEV = 0,
  ONLINE = 1,
}

export const BOT_ENV_RECORDS: Partial<
  Record<
    BotEnv,
    {
      label: string;
    }
  >
> = {
  [BotEnv.DEV]: {
    label: 'observation_threads_options_dev',
  },
  [BotEnv.ONLINE]: {
    label: 'observation_threads_options_online',
  },
};
