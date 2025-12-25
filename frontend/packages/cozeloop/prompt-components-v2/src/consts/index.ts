// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Role, TemplateType, VariableType } from '@cozeloop/api-schema/prompt';

export const VARIABLE_MAX_LEN = 50;
export const UPDATED_DRAFT_CODE = 600503308;
export const CALL_SLEEP_TIME = 400;

export const MAX_FILE_SIZE_MB = 20;
export const MAX_FILE_SIZE = MAX_FILE_SIZE_MB * 1024;
export const MAX_IMAGE_FILE = 20;

export const modelConfigLabelMap: Record<string, string> = {
  temperature: I18n.t('temperature'),
  max_tokens: I18n.t('max_tokens'),
  top_p: 'Top P',
  top_k: 'Top K',
  presence_penalty: I18n.t('presence_penalty'),
  frequency_penalty: I18n.t('frequency_penalty'),
  json_mode: 'JSON Mode',
  extra: I18n.t('prompt_additional_configuration'),
  thinking_type: I18n.t('prompt_deep_thinking_switch'),
  max_completion_tokens: I18n.t('prompt_deep_thinking_length'),
  reasoning_effort: I18n.t('prompt_deep_thinking_degree'),
};

export const DEFAULT_MAX_TOKENS = 4096;

export enum PromptStorageKey {
  PLAYGROUND_INFO = 'playground-info',
  PLAYGROUND_MOCKSET = 'playground-mockset',
}

export const EVENT_NAMES = {
  /** 用户创建Prompt的方式，创建成功时上报 */
  prompt_create: 'prompt_create',
  /** 用户新增function_call情况，点击确认按钮时上报 */
  prompt_function_call_add: 'prompt_function_call_add',
  /** 用户运行模型情况 */
  prompt_execute_info: 'prompt_execute_info',
  /** 单步调试时mock值修改 */
  prompt_step_debugger_mock_change: 'prompt_step_debugger_mock_change',
  /** 竞技场类型切换 */
  prompt_compare_type_change: 'prompt_compare_type_change',
  /** 选中比较的prompt */
  prompt_compare_use: 'prompt_compare_use',
  /** prompt提交埋点 */
  prompt_submit_info: 'prompt_submit_info',
  /** prompt运行记录 */
  prompt_execute_log: 'prompt_execute_log',
  /** PE模块，点击运行按钮调试prompt */
  prompt_execute: 'prompt_execute',
  /** view code 按钮点击次数 */
  prompt_click_view_code: 'prompt_click_view_code',
  /** 方法新增 */
  prompt_tool_add: 'prompt_tool_add',
  /** 方法删除 */
  prompt_tool_delete: 'prompt_tool_delete',
  /** 进入自由对比模式 */
  pe_mode_compare: 'pe_mode_compare',
  /** 查看版本切换 */
  cozeloop_pe_version: 'cozeloop_pe_version',
  cozeloop_pe_column_collapse: 'cozeloop_pe_column_collapse',
  prompt_insert_mock_msg: 'prompt_insert_mock_msg',
};

export enum MessageListRoundType {
  Multi = 'multi',
  Single = 'single',
}

export enum MessageListGroupType {
  Single = 'single',
  Multi = 'multi',
}

export const VARIABLE_TYPE_ARRAY_MAP = {
  [VariableType.String]: 'String',
  [VariableType.Integer]: 'Integer',
  [VariableType.Float]: 'Float',
  [VariableType.Boolean]: 'Boolean',
  [VariableType.Object]: 'Object',
  [VariableType.Array_String]: 'Array<String>',
  [VariableType.Array_Integer]: 'Array<Integer>',
  [VariableType.Array_Float]: 'Array<Float>',
  [VariableType.Array_Boolean]: 'Array<Boolean>',
  [VariableType.Array_Object]: 'Array<Object>',
  [VariableType.Placeholder]: 'Placeholder',
  [VariableType.MultiPart]: I18n.t('multimodal'),
};

export const VARIABLE_TYPE_ARRAY_TAG = {
  [VariableType.String]: '1',
  [VariableType.Integer]: '2',
  [VariableType.Float]: '4',
  [VariableType.Boolean]: '3',
  [VariableType.Object]: '6',
  [VariableType.Array_String]: '99',
  [VariableType.Array_Integer]: '100',
  [VariableType.Array_Float]: '102',
  [VariableType.Array_Boolean]: '101',
  [VariableType.Array_Object]: '103',
  [VariableType.Placeholder]: 'Placeholder',
};

export const DEFAULT_MESSAGE_TYPE_ARRAY = [
  { label: 'System', value: Role.System },
  { label: 'User', value: Role.User },
  { label: 'Assistant', value: Role.Assistant },
  { label: 'Placeholder', value: Role.Placeholder },
];

export const keyTextMap = {
  thinking_type: I18n.t('prompt_deep_thinking_switch'),
  max_completion_tokens: I18n.t('prompt_deep_thinking_length'),
  reasoning_effort: I18n.t('prompt_deep_thinking_degree'),
};

export const LABEL_MAP = {
  [TemplateType.Normal]: 'Normal',
  [TemplateType.GoTemplate]: 'GoTemplate',
  [TemplateType.Jinja2]: 'Jinja2',
};
