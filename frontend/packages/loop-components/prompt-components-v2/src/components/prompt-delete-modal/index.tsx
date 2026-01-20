// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { TextWithCopy } from '@cozeloop/components';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import {
  type ButtonProps,
  Input,
  Modal,
  Space,
  Toast,
  Typography,
} from '@coze-arch/coze-design';

interface PromptDeleteModalProps {
  data?: Prompt;
  visible: boolean;
  onCacnel?: () => void;
  onOk?: () => void;
}
export function PromptDeleteModal({
  data,
  visible,
  onCacnel,
  onOk,
}: PromptDeleteModalProps) {
  const [deleteKey, setDeleteKey] = useState('');

  const service = useRequest(
    promptId => StonePromptApi.DeletePrompt({ prompt_id: promptId }),
    {
      manual: true,
      onSuccess: () => {
        Toast.success({
          content: I18n.t('delete_success'),
          showClose: false,
        });
        onOk?.();
      },
    },
  );

  const handleOk = () => {
    if (data?.id && deleteKey === data?.prompt_key) {
      service.runAsync(data?.id).catch(console.log);
    }
  };

  useEffect(() => {
    if (visible) {
      setDeleteKey('');
    }
  }, [visible]);
  return (
    <Modal
      data-btm="c99085"
      title={I18n.t('delete_prompt')}
      visible={visible}
      onCancel={onCacnel}
      onOk={handleOk}
      okButtonProps={
        {
          disabled: Boolean(!deleteKey || deleteKey !== data?.prompt_key),
          loading: service.loading,
          'data-btm': 'd66291',
          'data-btm-title': I18n.t('prompt_confirm_delete'),
        } as unknown as ButtonProps
      }
      okText={I18n.t('confirm')}
      cancelText={I18n.t('cancel')}
      width={640}
    >
      <Space vertical style={{ width: '100%' }} align="start" spacing={0}>
        <Typography.Text type="danger">
          {I18n.t('prompt_data_cannot_recover_after_deletion')}
        </Typography.Text>
        <Typography.Text>
          {I18n.t('prompt_confirm_deletion_input_prompt_key')}
          <TextWithCopy
            content={data?.prompt_key}
            maxWidth={400}
            className="gap-2"
            copyTooltipText={I18n.t('copy_prompt_key')}
          />
        </Typography.Text>
        <Input
          className="w-full mt-4"
          placeholder={I18n.t('prompt_key_again_confirm')}
          value={deleteKey}
          onChange={setDeleteKey}
        />
      </Space>
    </Modal>
  );
}
