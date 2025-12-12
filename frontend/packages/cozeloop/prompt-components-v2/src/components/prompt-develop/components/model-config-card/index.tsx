/* eslint-disable complexity */
/* eslint-disable max-len */
import { useShallow } from 'zustand/react/shallow';
import { I18n } from '@cozeloop/i18n-adapter';
import { CollapseCard } from '@cozeloop/components';
import { ContentType } from '@cozeloop/api-schema/prompt';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tag, Tooltip, Typography } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { useBasicStore } from '@/store/use-basic-store';
import { useCompare } from '@/hooks/use-compare';
import { type ModelConfigWithName } from '@/components/model-config-editor/utils';

import { usePromptDevProviderContext } from '../prompt-provider';
import { BasicModelConfigEditor } from '../../../model-config-editor/basic-model-config-editor';

export function ModelConfigCard() {
  const { modelInfo } = usePromptDevProviderContext();
  const {
    modelConfig,
    setModelConfig,
    setCurrentModel,
    promptInfo,
    currentModel,
    messageList,
  } = usePromptStore(
    useShallow(state => ({
      modelConfig: state.modelConfig,
      setModelConfig: state.setModelConfig,
      setCurrentModel: state.setCurrentModel,
      promptInfo: state.promptInfo,
      currentModel: state.currentModel,
      messageList: state.messageList,
    })),
  );
  const isMultiModalModel = currentModel?.ability?.multi_modal;
  const multiModalError = messageList?.some(message => {
    if (
      message.parts?.some(
        part => part.type === ContentType.MultiPartVariable,
      ) &&
      !isMultiModalModel
    ) {
      return true;
    }
    return false;
  });
  const { readonly } = useBasicStore(
    useShallow(state => ({ readonly: state.readonly })),
  );
  const { streaming } = useCompare();

  return (
    <CollapseCard
      title={<Typography.Text strong>{I18n.t('model_config')}</Typography.Text>}
      subInfo={
        multiModalError ? (
          <Tooltip
            content={I18n.t('selected_model_not_support_multi_modal')}
            theme="dark"
          >
            <Tag color="red" prefixIcon={<IconCozInfoCircle />}>
              {I18n.t('model_not_support')}
            </Tag>
          </Tooltip>
        ) : null
      }
      defaultVisible
      key={`${modelConfig?.model_id}-${promptInfo?.prompt_commit?.commit_info?.version}-${promptInfo?.prompt_draft?.draft_info?.base_version}`}
    >
      <BasicModelConfigEditor
        value={modelConfig as ModelConfigWithName}
        onChange={config => {
          setModelConfig({ ...config });
        }}
        disabled={streaming || readonly}
        models={modelInfo?.list || []}
        onModelChange={setCurrentModel}
        modelSelectProps={{
          className: 'w-full',
          loading: modelInfo?.loading,
        }}
        defaultActiveFirstModel={Boolean(
          !promptInfo?.prompt_key && !modelConfig?.model_id,
        )}
      />
    </CollapseCard>
  );
}
