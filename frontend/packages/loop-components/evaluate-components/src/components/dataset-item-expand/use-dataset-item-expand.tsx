// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozLoose, IconCozTight } from '@coze-arch/coze-design/icons';
import { Radio, Tooltip } from '@coze-arch/coze-design';

export const useDatasetItemExpand = () => {
  const [expand, setExpand] = useState(false);
  const ExpandNode = (
    <Radio.Group
      type="button"
      className="!gap-0"
      value={expand ? 'expand' : 'shrink'}
      onChange={e => setExpand(e.target.value === 'expand' ? true : false)}
    >
      <Tooltip content={I18n.t('compact_view')} theme="dark">
        <Radio value="shrink" addonClassName="flex items-center">
          <IconCozTight className="text-lg" />
        </Radio>
      </Tooltip>
      <Tooltip content={I18n.t('loose_view')} theme="dark">
        <Radio value="expand" addonClassName="flex items-center">
          <IconCozLoose className="text-lg" />
        </Radio>
      </Tooltip>
    </Radio.Group>
  );

  return {
    expand,
    setExpand,
    ExpandNode,
  };
};
