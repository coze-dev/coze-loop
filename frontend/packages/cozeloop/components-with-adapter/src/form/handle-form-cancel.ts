// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Modal } from '@coze-arch/coze-design';

export const handleFormCancel = (changed?: boolean, onCancel?: () => void) => {
  if (changed) {
    Modal.confirm({
      title: I18n.t('form_leave_title'),
      content: I18n.t('form_leave_desc'),
      onOk: onCancel,
      okText: I18n.t('confirm'),
      cancelText: I18n.t('cancel'),
    });
  } else {
    onCancel?.();
  }
};
