import { useEffect, useState } from 'react';

import { useRequest } from 'ahooks';
import { TextWithCopy } from '@cozeloop/components';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import { Input, Modal, Space, Toast, Typography } from '@coze-arch/coze-design';

interface PromptDeleteProps {
  data?: Prompt;
  visible: boolean;
  onCacnel?: () => void;
  onOk?: () => void;
}
export function PromptDelete({
  data,
  visible,
  onCacnel,
  onOk,
}: PromptDeleteProps) {
  const [deleteKey, setDeleteKey] = useState('');
  const { runAsync: runDeleteAsync, loading: deleteLoading } = useRequest(
    promptId => StonePromptApi.DeletePrompt({ prompt_id: promptId }),
    {
      manual: true,
      onSuccess: () => {
        Toast.success({
          content: '删除成功',
          showClose: false,
        });
        onOk?.();
      },
    },
  );

  const handleOk = () => {
    if (data?.id && deleteKey === data?.prompt_key) {
      runDeleteAsync(data?.id);
    }
  };

  useEffect(() => {
    if (visible) {
      setDeleteKey('');
    }
  }, [visible]);
  return (
    <Modal
      title="删除Prompt"
      visible={visible}
      onCancel={onCacnel}
      onOk={handleOk}
      okButtonProps={{
        disabled: Boolean(!deleteKey || deleteKey !== data?.prompt_key),
        loading: deleteLoading,
      }}
      okText="确定"
      cancelText="取消"
    >
      <Space vertical style={{ width: '100%' }} align="start">
        <Typography.Text>
          输入想要删除的Prompt Key：
          <TextWithCopy
            content={data?.prompt_key}
            maxWidth={400}
            className="gap-2"
            copyTooltipText="复制 Prompt Key"
          />
        </Typography.Text>
        <Input
          style={{ width: '100%' }}
          placeholder="请输入 Prompt Key 再次确认"
          value={deleteKey}
          onChange={setDeleteKey}
        />
      </Space>
    </Modal>
  );
}
