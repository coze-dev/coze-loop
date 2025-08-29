import { type Model } from '@cozeloop/api-schema/llm-manage';
import { ContentType, type Message } from '@cozeloop/api-schema/evaluation';

export function multiModelValidate(
  messages: Message[],
  model: Model | undefined,
): string | undefined {
  const hasMultiModelVar = messages?.some(
    message => message.content?.content_type === ContentType.MultiPart,
  );
  if (hasMultiModelVar && !model?.ability?.multi_modal) {
    return '所选模型不支持多模态,请调整变量类型或更换模型';
  }
  return;
}
