// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozLoose, IconCozTight } from '@coze-arch/coze-design/icons';
import { Radio, Tooltip } from '@coze-arch/coze-design';

export const useExpandButton = ({
  shrinkTooltip = I18n.t('fold'),
  expandTooltip = I18n.t('expand'),
}: {
  shrinkTooltip?: string;
  expandTooltip?: string;
}) => {
  const [expand, setExpand] = useState(true);
  const ExpandNode = (
    <Radio.Group
      type="button"
      value={expand ? 'expand' : 'shrink'}
      onChange={e => setExpand(e.target.value === 'expand' ? true : false)}
    >
      <Tooltip content={shrinkTooltip} theme="dark">
        <Radio value="shrink" addonClassName="flex items-center">
          <IconCozTight className="text-lg" />
        </Radio>
      </Tooltip>
      <Tooltip content={expandTooltip} theme="dark">
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
