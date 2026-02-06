// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { TextArea } from '@coze-arch/coze-design';

import { type DatasetItemProps } from '../../../type';

export const PlainTextDatasetItemEdit = ({
  fieldContent,
  onChange,
}: DatasetItemProps) => (
  <TextArea
    value={fieldContent?.text}
    className="rounded-[6px]"
    autosize={{ minRows: 1, maxRows: 6 }}
    onChange={value => {
      onChange?.({
        ...fieldContent,
        text: value,
      });
    }}
  />
);
