import { useState } from 'react';

import { IconCozLoose, IconCozTight } from '@coze-arch/coze-design/icons';
import { Radio, Tooltip } from '@coze-arch/coze-design';

export const useExpandButton = ({
  shrinkTooltip = '折叠',
  expandTooltip = '展开',
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
