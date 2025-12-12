// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Modal, type ModalProps } from '@coze-arch/coze-design';

/**
 * 未保存弹框提示
 */
export function unsaveWarning(modalProps?: ModalProps) {
  return Modal.warning({
    title: I18n.t('data_engine_info_unsaved'),
    content: I18n.t('data_engine_confirm_close_warning'),
    cancelText: I18n.t('cancel'),
    okText: I18n.t('confirm'),
    ...modalProps,
  });
}
