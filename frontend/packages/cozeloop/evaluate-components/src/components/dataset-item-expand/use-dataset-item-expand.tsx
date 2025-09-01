import { useState } from 'react';

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
      <Tooltip content="紧凑视图" theme="dark">
        <Radio value="shrink" addonClassName="flex items-center">
          <IconCozTight className="text-lg" />
        </Radio>
      </Tooltip>
      <Tooltip content="宽松视图" theme="dark">
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
