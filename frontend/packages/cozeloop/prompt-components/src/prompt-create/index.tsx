/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { useMemo, useRef } from 'react';

import { useRequest } from 'ahooks';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import {
  Form,
  FormInput,
  FormTextArea,
  Modal,
  type FormApi,
} from '@coze-arch/coze-design';

interface PromptCreateProps {
  visible: boolean;
  data?: Prompt;
  isEdit?: boolean;
  isCopy?: boolean;
  onOk: (v: Prompt & { cloned_prompt_id?: Int64 }) => void;
  onCancel: () => void;
}
interface FormValueProps {
  prompt_key?: string;
  prompt_name?: string;
  prompt_description?: string;
  version?: string;
}
export function PromptCreate({
  visible,
  data,
  isCopy,
  isEdit,
  onOk,
  onCancel,
}: PromptCreateProps) {
  const formApi = useRef<FormApi<FormValueProps>>();
  const { spaceID } = useSpace();

  const createService = useRequest(
    (prompt: FormValueProps) =>
      StonePromptApi.CreatePrompt({
        prompt_key: prompt.prompt_key || '',
        prompt_name: prompt.prompt_name || '',
        prompt_description: prompt.prompt_description,
        workspace_id: spaceID,
        draft_detail: data?.prompt_commit?.detail,
      }),
    {
      manual: true,
    },
  );
  const updateService = useRequest(
    (prompt: FormValueProps) =>
      StonePromptApi.UpdatePrompt({
        prompt_id: data?.id || '',
        prompt_name: prompt.prompt_name || '',
        prompt_description: prompt.prompt_description,
      }),
    {
      manual: true,
    },
  );
  const copyService = useRequest(
    (prompt: FormValueProps) =>
      StonePromptApi.ClonePrompt({
        prompt_id: data?.id || '',
        cloned_prompt_key: prompt.prompt_key || '',
        cloned_prompt_name: prompt.prompt_name || '',
        cloned_prompt_description: prompt.prompt_description,
        commit_version: data?.prompt_commit?.commit_info?.version,
      }),
    {
      manual: true,
    },
  );
  const handleOk = async () => {
    const formData = await formApi.current?.validate();
    if (!formData) {
      return;
    }

    if (isCopy) {
      const res = await copyService.runAsync(formData);
      onOk({ ...data, cloned_prompt_id: res.cloned_prompt_id });
      sendEvent(EVENT_NAMES.prompt_create, {
        prompt_id: `${data?.id || ''}`,
        prompt_key: data?.prompt_key || '',
        original_version: formData?.version,
      });
    } else if (isEdit) {
      await updateService.runAsync(formData);
      sendEvent(EVENT_NAMES.prompt_create, {
        prompt_id: `${data?.id || ''}`,
        prompt_key: data?.prompt_key || '',
        is_update: true,
      });
      onOk({
        ...data,
        prompt_basic: {
          ...data?.prompt_basic,
          display_name: formData.prompt_name,
          description: formData.prompt_description,
        },
      });
    } else {
      const res = await createService.runAsync(formData);
      sendEvent(EVENT_NAMES.prompt_create, {
        prompt_id: `${data?.id || ''}`,
      });
      onOk({ ...data, id: res.prompt_id });
    }
  };

  const modalTitle = useMemo(() => {
    if (isEdit) {
      return '编辑 Prompt';
    }
    if (isCopy) {
      return '创建副本';
    }
    return '创建 Prompt';
  }, [isCopy, isEdit]);

  return (
    <Modal
      title={modalTitle}
      visible={visible}
      onCancel={onCancel}
      onOk={handleOk}
      cancelText="取消"
      okText="确定"
      okButtonProps={{
        loading:
          createService.loading || updateService.loading || copyService.loading,
      }}
      width={900}
    >
      <Form<FormValueProps>
        getFormApi={api => (formApi.current = api)}
        initValues={{
          prompt_key: isCopy
            ? `${
                (data?.prompt_key?.length || 0) < 95
                  ? `${data?.prompt_key}_copy`
                  : data?.prompt_key
              }`
            : data?.prompt_key,
          prompt_name: isCopy
            ? `${
                (data?.prompt_basic?.display_name?.length || 0) < 95
                  ? `${data?.prompt_basic?.display_name}_copy`
                  : data?.prompt_basic?.display_name
              }`
            : data?.prompt_basic?.display_name,
          prompt_description: data?.prompt_basic?.description,
        }}
      >
        <FormInput
          label="Prompt Key"
          field="prompt_key"
          placeholder="请输入 Prompt key"
          rules={[
            { required: true, message: '请输入 Prompt Key' },
            {
              validator: (_rule, value) => {
                if (value && !/^[a-zA-Z][a-zA-Z0-9_.]*$/.test(value)) {
                  return new Error(
                    '仅支持英文字母、数字、“_”、“.”，且仅支持英文字母开头',
                  );
                }
                return true;
              },
            },
          ]}
          maxLength={100}
          max={100}
          disabled={isEdit}
        />
        <FormInput
          label="Prompt 名称"
          field="prompt_name"
          placeholder="请输入 Prompt 名称"
          rules={[
            { required: true, message: '请输入 Prompt 名称' },
            {
              validator: (_rule, value) => {
                if (value && !/^[\u4e00-\u9fa5a-zA-Z0-9_.-]+$/.test(value)) {
                  return new Error(
                    '仅支持英文字母、数字、中文，“-”，“_”，“.”，且仅支持英文字母、数字、中文开头',
                  );
                }
                if (value && /^[_.-]/.test(value)) {
                  return new Error(
                    '仅支持英文字母、数字、中文，“-”，“_”，“.”，且仅支持英文字母、数字、中文开头',
                  );
                }
                return true;
              },
            },
          ]}
          maxLength={100}
          max={100}
        />
        <FormTextArea
          label="Prompt 描述"
          field="prompt_description"
          placeholder="请输入 Prompt 描述"
          maxCount={500}
          maxLength={500}
        />
      </Form>
    </Modal>
  );
}
