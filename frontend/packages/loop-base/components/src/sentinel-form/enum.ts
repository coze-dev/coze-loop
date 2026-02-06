// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export enum EventNames {
  // 进入相关
  INIT_LOOP_FORM = 'init_loop_form',
  // 保点击保存，表单报错
  LOOP_FORM_FIELD_VALIDATE_ERROR = 'loop_form_field_validate_error', // 表单项报错次数

  // 点击保存，接口报错
  LOOP_FORM_SUBMIT_INTERFACE_ERROR = 'loop_form_submit_interface_error', // 提交接口报错次数

  // 点击保存，成功
  LOOP_FORM_SUBMIT_SUCCESS = 'loop_form_submit_success', // 提交成功

  // 离开表单
  LOOP_FORM_FIELD_CHANGE_TIMELINE = 'loop_form_field_change_timeline', // 表单项变更时间轴
  LOOP_FORM_CLOSE = 'loop_form_close', // 离开表单
}
