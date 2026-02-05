// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
export const evaluateTargetValidators = {
  evalTargetType: [{ required: true, message: I18n.t('select_type') }],
  evalTarget: [
    { required: true, message: I18n.t('please_select_evaluate_target') },
  ],
  evalTargetVersion: [
    {
      required: true,
      message: I18n.t('evaluate_please_select_target_version'),
    },
  ],
  // todo: 这里注册进来
  evalTargetMapping: [
    { required: true, message: I18n.t('config_evaluation_object_mapping') },
  ],
};
