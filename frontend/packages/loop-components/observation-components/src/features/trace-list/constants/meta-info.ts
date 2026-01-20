// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const metaInfo = {
  field_metas: {
    input: {
      filter_types: ['match', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'string',
    },
    span_type: {
      filter_types: ['in', 'not_in', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'string',
    },
    status: {
      field_options: {
        string_list: ['success', 'error'],
      },
      filter_types: ['in'],
      support_customizable_option: false,
      value_type: 'string',
    },
    output: {
      filter_types: ['match', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'string',
    },
    span_name: {
      filter_types: ['match', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'string',
    },
    thread_id: {
      filter_types: ['in', 'not_in', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'string',
    },
    trace_id: {
      support_customizable_option: true,
      value_type: 'string',
      filter_types: ['in', 'not_in', 'exist', 'not_exist'],
    },
    duration: {
      filter_types: ['gte', 'lte', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'long',
    },
    latency_first_resp: {
      filter_types: ['gte', 'lte', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'long',
    },
    message_id: {
      filter_types: ['in', 'not_in', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'string',
    },
    user_id: {
      filter_types: ['in', 'not_in', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'string',
    },
    metadata: {
      filter_types: [],
      support_customizable_option: true,
      value_type: '',
    },
    logid: {
      filter_types: ['in', 'not_in', 'exist', 'not_exist'],
      support_customizable_option: true,
      value_type: 'string',
    },
  },
};
