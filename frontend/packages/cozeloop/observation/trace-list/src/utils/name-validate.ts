// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { I18n } from '@cozeloop/i18n-adapter';

const MAX_NAME_LENGTH = 20;
export const validateViewName = (name: string, viewNames: string[]) => {
  if (name.trim() === '') {
    return {
      isValid: false,
      message: I18n.t('validation_not_allowed_to_be_empty'),
    };
  }

  if (name.trim().length > MAX_NAME_LENGTH) {
    return {
      isValid: false,
      message: I18n.t('validation_name_length_limit', { num: MAX_NAME_LENGTH }),
    };
  }
  if (viewNames.includes(name.trim())) {
    return {
      isValid: false,
      message: I18n.t('validation_view_name_exists'),
    };
  }
  return {
    isValid: true,
    message: '',
  };
};
