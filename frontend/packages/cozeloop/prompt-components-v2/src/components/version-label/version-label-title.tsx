// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tooltip } from '@coze-arch/coze-design';

export function VersionLabelTitle() {
  return (
    <div className="flex items-center">
      <div>{I18n.t('prompt_version_tag')}</div>
      <Tooltip
        theme="dark"
        content={
          <div>
            {I18n.t('prompt_mark_version_feature_sdk_fetch')}
            <a
              style={{
                color: '#AAA6FF',
                textDecoration: 'none',
              }}
              href="https://loop.coze.cn/open/docs/cozeloop/prompt_version"
              target="_blank"
            >
              {I18n.t('prompt_user_manual')}
            </a>
          </div>
        }
      >
        <IconCozInfoCircle className="coz-fg-secondary ml-1 cursor-pointer" />
      </Tooltip>
    </div>
  );
}
