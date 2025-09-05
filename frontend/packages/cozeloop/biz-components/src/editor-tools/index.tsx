// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { handleCopy } from '@cozeloop/components';
import {
  type Message,
  type Role,
  type Prompt,
  ContentType,
} from '@cozeloop/api-schema/prompt';
import { IconCozCopy, IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { IconButton, Popconfirm, Space } from '@coze-arch/coze-design';

export type PromptMessage<R = Role> = Omit<Message, 'role'> & {
  role?: R;
  id?: string;
  key?: string;
  optimize_key?: string;
};

interface EditorToolsProps<R> {
  onDelete?: (id?: string | number) => void;
  message?: PromptMessage<R>;
  disabled?: boolean;
  promptInfo?: Prompt;
  optimizeBtnHidden?: boolean;
  onMessageChange?: (message: PromptMessage<R>) => void;
}

export function EditorTools<R>({
  onDelete,
  message,
  disabled,
}: EditorToolsProps<R>) {
  const handleInfo = () => {
    if (message?.parts?.length) {
      return message?.parts
        ?.map(it => {
          if (it.type === ContentType.MultiPartVariable && it?.text) {
            return `<multimodal-variable>${it.text}</multimodal-variable>`;
          }
          return it.text;
        })
        .join('');
    }
    return message?.content;
  };
  return (
    <Space>
      <IconButton
        icon={<IconCozCopy />}
        color="secondary"
        size="mini"
        onClick={() => {
          const info = handleInfo() ?? '';
          handleCopy(info);
        }}
      />
      {!onDelete ? null : disabled ? (
        <IconButton
          icon={<IconCozTrashCan />}
          color="secondary"
          size="mini"
          disabled={disabled}
        />
      ) : (
        <Popconfirm
          title={I18n.t('delete_prompt_template')}
          content={I18n.t('confirm_delete_current_prompt_template')}
          cancelText={I18n.t('Cancel')}
          okText={I18n.t('delete')}
          okButtonProps={{ color: 'red' }}
          onConfirm={() => onDelete?.(`${message?.key || message?.id || ''}`)}
        >
          <IconButton
            icon={<IconCozTrashCan />}
            color="secondary"
            size="mini"
          />
        </Popconfirm>
      )}
    </Space>
  );
}
