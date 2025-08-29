import { useMemo } from 'react';

import {
  PromptEditor,
  type PromptMessage,
  type PromptEditorProps,
  getMultimodalVariableText,
} from '@cozeloop/prompt-components';
import { type ContentPart, prompt } from '@cozeloop/api-schema/prompt';
import {
  type Content,
  ContentType,
  Role,
  type Message,
} from '@cozeloop/api-schema/evaluation';

export type EvaluatorPromptEditorProps = Omit<
  PromptEditorProps<Role>,
  'message' | 'onMessageChange'
> & {
  message?: Message;
  onMessageChange?: (message: Message) => void;
};

function isMultiPartVariable(contentType: ContentType | undefined) {
  return contentType === ContentType.MultiPartVariable;
}
function isMultiPart(contentType: ContentType | undefined) {
  return contentType === ContentType.MultiPart;
}

const messageTypeList = [
  {
    label: 'System',
    value: Role.System,
  },
  {
    label: 'User',
    value: Role.User,
  },
];

function promptPartsToMultiParts(parts: ContentPart[]): Content[] {
  const multiParts = parts?.map(part => {
    const typeMap = {
      [prompt.ContentType.Text]: ContentType.Text,
      MultiPart: ContentType.MultiPartVariable,
      multi_part_variable: ContentType.MultiPartVariable,
    };
    const multiPart: Content = {
      content_type:
        typeMap[part.type as prompt.ContentType] ?? ContentType.Text,
      text: part.text,
    };
    return multiPart;
  });
  return multiParts;
}

/** 把Prompt的Message格式转化为评估器这边定义的Message格式 */
export function EvaluatorPromptEditor(props: EvaluatorPromptEditorProps) {
  const { message, onMessageChange, ...rest } = props;
  const stringMessage: PromptMessage<Role> | undefined = useMemo(() => {
    if (isMultiPart(message?.content?.content_type)) {
      const messageContent = message.content?.multi_part
        ?.map(part => {
          if (isMultiPartVariable(part?.content_type)) {
            const str = getMultimodalVariableText(part.text ?? '');
            return str;
          }
          return part?.text ?? '';
        })
        .join('');
      return {
        ...message,
        content: messageContent,
      };
    }
    return {
      ...message,
      content: message?.content?.text,
    };
  }, [message]);

  const handleMessageChange = (newMsg: PromptMessage<Role>) => {
    // 这里认为newMsg中的消息类型为字符串
    const multiParts = newMsg?.parts;
    const hasMultiPartVariable =
      Array.isArray(multiParts) && multiParts.length > 0;
    // 有没有多模态变量进行不同的处理
    if (!hasMultiPartVariable) {
      onMessageChange?.({
        role: newMsg.role,
        content: {
          content_type: ContentType.Text,
          text: newMsg.content,
        },
      });
    } else {
      const multiPart = promptPartsToMultiParts(multiParts);
      onMessageChange?.({
        role: newMsg.role,
        content: {
          content_type: ContentType.MultiPart,
          multi_part: multiPart,
        },
      });
    }
  };

  return (
    <PromptEditor<Role>
      {...rest}
      messageTypeList={props.messageTypeList ?? messageTypeList}
      message={stringMessage}
      onMessageChange={handleMessageChange}
      modalVariableBtnHidden={false}
    />
  );
}
