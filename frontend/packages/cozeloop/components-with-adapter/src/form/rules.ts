// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';

export function getRequiredRule(): { required?: boolean; message?: string } {
  return {
    required: true,
    message: I18n.t('fornax_base_required_error'),
  };
}
