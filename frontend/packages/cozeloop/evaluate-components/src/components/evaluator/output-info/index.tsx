import { useEffect } from 'react';

import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tooltip } from '@coze-arch/coze-design';

import { useDefaultPromptEvaluatorToolsStore } from './use-default-prompt-evaluator-tools-store';

export function OutputInfo({ className }: { className?: string }) {
  const { toolsDescription, fetchData } = useDefaultPromptEvaluatorToolsStore();

  useEffect(() => {
    fetchData();
  }, []);

  return (
    <div className={className}>
      <div className="flex flex-row items-center h-5 text-sm font-medium coz-fg-primary mb-2">
        {'输出'}
        <Tooltip
          content={
            '通过 Function Call 从 LLM 中提取数据，固定评估器输出格式为“得分-原因”。'
          }
        >
          <IconCozInfoCircle className="ml-1 coz-fg-secondary" />
        </Tooltip>
      </div>

      <div className="coz-fg-secondary text-[13px] leading-5 font-normal mb-[6px]">
        <span className="font-medium">{'得分：'}</span>
        {toolsDescription?.score}
      </div>
      <div className="coz-fg-secondary text-[13px] leading-5 font-normal">
        <span className="font-medium">{'原因：'}</span>
        {toolsDescription?.reason}
      </div>
    </div>
  );
}
