import { type Message } from '@cozeloop/api-schema/evaluation';

import { EvaluatorPromptEditor } from '../evaluator-prompt-editor';

export function PromptMessage({
  className,
  message,
}: {
  className?: string;
  message?: Message;
}) {
  return (
    <EvaluatorPromptEditor
      className={className}
      key={JSON.stringify(message)}
      disabled={true}
      messageTypeDisabled={true}
      maxHeight={500}
      minHeight={36}
      dragBtnHidden={true}
      modalVariableEnable={true}
      message={message}
    />
  );
}
