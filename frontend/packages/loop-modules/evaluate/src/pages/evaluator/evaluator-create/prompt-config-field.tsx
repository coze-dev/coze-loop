// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';

import { ConfigContent } from './config-content';

interface Props {
  disabled?: boolean;
  refreshEditorModelKey?: number;
}

export function PromptConfigField({ disabled, refreshEditorModelKey }: Props) {
  return (
    <>
      <div className="h-[28px] mb-3 text-[16px] leading-7 font-medium coz-fg-plus flex flex-row items-center">
        {I18n.t('config_info')}
      </div>
      <ConfigContent
        disabled={disabled}
        refreshEditorModelKey={refreshEditorModelKey}
      />
    </>
  );
}
