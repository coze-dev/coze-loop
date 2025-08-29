import {
  type EvalTarget,
  type RuntimeParam,
} from '@cozeloop/api-schema/evaluation';
import { Popover, Typography } from '@coze-arch/coze-design';

import { PromptDynamicParams } from './prompt-dynamic-params';

interface Props {
  data: RuntimeParam;
  evalTarget?: EvalTarget;
}

export function DynamicParams({ data, evalTarget }: Props) {
  return (
    <Popover
      content={
        <div className="max-h-[640px] overflow-auto">
          <div className="px-5 py-3 text-[16px] font-medium coz-fg-plus">
            参数详情
          </div>
          <div className="w-[612px] px-5 pb-6">
            <PromptDynamicParams data={data} evalTarget={evalTarget} />
          </div>
        </div>
      }
    >
      <span>
        <Typography.Text link>参数详情</Typography.Text>
      </span>
    </Popover>
  );
}
