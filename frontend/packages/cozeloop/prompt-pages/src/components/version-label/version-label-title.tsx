// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tooltip } from '@coze-arch/coze-design';

export function VersionLabelTitle() {
  return (
    <div className="flex items-center">
      <div>版本标识</div>
      <Tooltip
        theme="dark"
        content={
          <div>
            标记版本特性，可通过标识在 SDK 拉取 Prompt 特定版本。查看
            <a
              style={{
                color: '#AAA6FF',
                textDecoration: 'none',
              }}
              href="https://loop.coze.cn/open/docs/cozeloop/prompt_version"
              target="_blank"
            >
              用户手册
            </a>
          </div>
        }
      >
        <IconCozInfoCircle className="coz-fg-secondary ml-1 cursor-pointer" />
      </Tooltip>
    </div>
  );
}
