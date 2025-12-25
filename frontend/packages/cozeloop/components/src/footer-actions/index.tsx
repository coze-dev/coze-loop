// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/naming-convention */
import { Button, type ButtonProps } from '@coze-arch/coze-design';

import { useI18n } from '../provider';

interface Props {
  confirmBtnProps?: ButtonProps & {
    'data-btm'?: string;
    'data-btm-title'?: string;
    text?: string;
  };
  cancelBtnProps?: ButtonProps & {
    'data-btm'?: string;
    'data-btm-title'?: string;
    text?: string;
  };
}
export function FooterActions({
  confirmBtnProps: { text: confirmBtnText, ...confirmBtnProps } = {},
  cancelBtnProps: { text: cancelBtnText, ...cancelBtnProps } = {},
}: Props) {
  const I18n = useI18n();
  return (
    <div className="flex justify-end">
      <Button color="primary" {...cancelBtnProps}>
        {cancelBtnText ?? I18n.t('cancel')}
      </Button>
      <Button className="ml-2" {...confirmBtnProps}>
        {confirmBtnText ?? I18n.t('global_btn_confirm')}
      </Button>
    </div>
  );
}
