/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { useMemo, useRef } from 'react';

import { nanoid } from 'nanoid';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { FooterActions, useReportEvent } from '@cozeloop/components';
import { PromptType, type Prompt } from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import {
  Form,
  FormInput,
  FormTextArea,
  Modal,
  type RuleItem,
  withField,
  type FormApi,
} from '@coze-arch/coze-design';

import { EVENT_NAMES } from '@/consts';

import { PromptVersionSelect } from '../prompt-version-select';

interface PromptCreateModalProps {
  spaceID: string;
  visible: boolean;
  data?: Prompt;
  isEdit?: boolean;
  isCopy?: boolean;
  isSnippet?: boolean;
  customTitle?: string;
  formRules?: Record<string, RuleItem[]>;
  enableSelectVersion?: boolean;
  onOk: (v: Prompt & { cloned_prompt_id?: Int64 }) => void;
  onCancel: () => void;
}
interface FormValueProps {
  prompt_key?: string;
  prompt_name?: string;
  prompt_description?: string;
  version?: string;
}

const COPY_PROMPT_KEY_MAX_LEN = 95;

const FormPromptVersionSelect = withField(PromptVersionSelect);
export function PromptCreateModal({
  spaceID,
  visible,
  data,
  customTitle,
  isCopy,
  isEdit,
  isSnippet,
  onOk,
  onCancel,
  formRules,
  enableSelectVersion,
}: PromptCreateModalProps) {
  const sendEvent = useReportEvent();
  const formApi = useRef<FormApi<FormValueProps>>();

  const createService = useRequest(
    (prompt: FormValueProps) =>
      StonePromptApi.CreatePrompt({
        prompt_key: prompt.prompt_key || '',
        prompt_name: prompt.prompt_name || '',
        prompt_description: prompt.prompt_description,
        workspace_id: spaceID,
        draft_detail: data?.prompt_commit?.detail,
        prompt_type: isSnippet ? PromptType.Snippet : PromptType.Normal,
      }),
    {
      manual: true,
      refreshDeps: [isSnippet],
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
        commit_version: prompt.version || '',
      }),
    {
      manual: true,
    },
  );
  const handleOk = async () => {
    const formData = await formApi.current?.validate().catch(console.log);
    if (!formData) {
      return;
    }

    if (isCopy) {
      const res = await copyService.runAsync(formData);
      sendEvent(EVENT_NAMES.prompt_create, {
        prompt_id: `${data?.id || ''}`,
        prompt_key: data?.prompt_key || '',
        original_version: formData?.version,
      });
      onOk({ ...data, id: res.cloned_prompt_id });
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
      let postData = formData;
      if (isSnippet) {
        const promptKeyId = nanoid()?.replace(/\-/g, '_')?.toLowerCase();
        const promptKey = `fornax.segment.${promptKeyId}`;
        postData = { ...formData, prompt_key: promptKey };
      }
      const res = await createService.runAsync(postData);
      sendEvent(EVENT_NAMES.prompt_create, {
        prompt_id: `${data?.id || ''}`,
      });
      onOk({
        ...data,
        prompt_key: formData.prompt_key,
        prompt_basic: {
          ...data?.prompt_basic,
          display_name: formData.prompt_name,
          description: formData.prompt_description,
        },
        id: res.prompt_id,
      });
    }
  };

  const modalTitle = useMemo(() => {
    if (customTitle) {
      return customTitle;
    }
    if (isEdit) {
      return isSnippet
        ? I18n.t('prompt_edit_prompt_snippet')
        : I18n.t('edit_prompt');
    }
    if (isCopy) {
      return isSnippet
        ? I18n.t('prompt_copy_prompt_snippet')
        : `${I18n.t('copy')} Prompt`;
    }
    return isSnippet
      ? I18n.t('prompt_create_prompt_snippet')
      : I18n.t('create_prompt');
  }, [isCopy, isEdit, isSnippet, customTitle]);

  const btmCode = useMemo(() => {
    if (isEdit) {
      return isSnippet ? '' : 'd27344';
    } else if (isCopy) {
      return isSnippet ? '' : 'd27346';
    } else {
      return isSnippet ? '' : 'd74327';
    }
  }, [isCopy, isEdit, isSnippet]);

  const promptNameLabel = isSnippet
    ? I18n.t('prompt_prompt_snippet_name')
    : I18n.t('prompt_name');
  const promptDescLabel = isSnippet
    ? I18n.t('prompt_prompt_snippet_description')
    : I18n.t('prompt_description');

  return (
    <Modal
      data-btm="c26531"
      title={modalTitle}
      visible={visible}
      onCancel={onCancel}
      width={isSnippet ? 640 : 900}
      footer={
        <FooterActions
          confirmBtnProps={{
            loading:
              createService.loading ||
              updateService.loading ||
              copyService.loading,
            onClick: handleOk,
            'data-btm': btmCode,
            'data-btm-title': `${I18n.t('prompt_modal_title_confirm', { modalTitle })}`,
          }}
          cancelBtnProps={{ onClick: onCancel }}
        />
      }
    >
      <Form<FormValueProps>
        getFormApi={api => (formApi.current = api)}
        initValues={{
          prompt_key: isCopy
            ? `${
                (data?.prompt_key?.length || 0) < COPY_PROMPT_KEY_MAX_LEN
                  ? `${data?.prompt_key}_copy`
                  : data?.prompt_key
              }`
            : data?.prompt_key,
          prompt_name: isCopy
            ? `${
                (data?.prompt_basic?.display_name?.length || 0) <
                COPY_PROMPT_KEY_MAX_LEN
                  ? `${data?.prompt_basic?.display_name}_copy`
                  : data?.prompt_basic?.display_name
              }`
            : data?.prompt_basic?.display_name,
          prompt_description: data?.prompt_basic?.description,
          version: isCopy
            ? data?.prompt_commit?.commit_info?.version
            : undefined,
        }}
      >
        {isCopy ? (
          <FormPromptVersionSelect
            className="w-full"
            label={I18n.t('version_number')}
            field="version"
            rules={[
              {
                required: true,
                message: I18n.t('please_select_a_version_number'),
              },
            ]}
            spaceID={spaceID}
            promptID={data?.id}
            disabled={!enableSelectVersion}
          />
        ) : null}
        {isSnippet ? null : (
          <FormInput
            label="Prompt Key"
            field="prompt_key"
            placeholder={I18n.t('prompt_please_input_prompt_key')}
            rules={[
              {
                required: true,
                message: I18n.t('prompt_please_input_prompt_key_caps'),
              },
              {
                validator: (_rule, value, callback) => {
                  if (value && value.length > 100) {
                    callback(I18n.t('prompt_prompt_key_length_limit'));
                    return false;
                  }
                  if (value && !/^[a-zA-Z][a-zA-Z0-9_.]*$/.test(value)) {
                    callback(I18n.t('prompt_key_format'));
                    return false;
                  }
                  return true;
                },
              },
              ...(formRules?.prompt_key || []),
            ]}
            maxLength={100}
            max={100}
            disabled={isEdit}
          />
        )}
        <FormInput
          label={promptNameLabel}
          field="prompt_name"
          placeholder={`${I18n.t('please_input')} ${promptNameLabel}`}
          rules={[
            {
              required: true,
              message: `${I18n.t('please_input')} ${promptNameLabel}`,
            },
            {
              validator: (_rule, value, callback) => {
                if (value && value.length > 100) {
                  callback(
                    `${I18n.t('prompt_prompt_name_length_limit', { promptNameLabel })}`,
                  );
                  return false;
                }
                if (value && !/^[\u4e00-\u9fa5a-zA-Z0-9_.-]+$/.test(value)) {
                  callback(I18n.t('prompt_name_format'));
                  return false;
                }
                if (value && /^[_.-]/.test(value)) {
                  callback(I18n.t('prompt_name_format'));
                  return false;
                }
                return true;
              },
            },
            ...(formRules?.prompt_name || []),
          ]}
          maxLength={100}
          max={100}
        />

        <FormTextArea
          label={promptDescLabel}
          field="prompt_description"
          placeholder={`${I18n.t('please_input')} ${promptDescLabel}`}
          maxCount={500}
          maxLength={500}
          rules={[...(formRules?.prompt_description || [])]}
        />
      </Form>
    </Modal>
  );
}
