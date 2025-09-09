// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Typography } from '@coze-arch/coze-design';

import { type DatasetItemProps } from '../../type';

export const FloatDatasetItemReadOnly = ({
  fieldContent,
  expand,
  displayFormat,
}: DatasetItemProps) => (
  <div
    style={
      displayFormat
        ? {
            border: '1px solid var(--coz-stroke-primary)',
            borderRadius: '6px',
            backgroundColor: 'var(--coz-bg-plus)',
            padding: 12,
            minHeight: 48,
          }
        : {}
    }
  >
    {expand ? (
      <Typography.Text className="!text-[13px] max-h-[292px] overflow-y-auto">
        {fieldContent?.text}
      </Typography.Text>
    ) : (
      <Typography.Text
        className="!text-[13px] w-full overflow-hidden"
        ellipsis={{
          showTooltip: {
            opts: {
              content: (
                <div className="max-h-[200px] overflow-auto">
                  {fieldContent?.text}
                </div>
              ),
            },
          },
          rows: 1,
        }}
      >
        {fieldContent?.text}
      </Typography.Text>
    )}
  </div>
);
