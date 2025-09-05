/* eslint-disable complexity */
/* eslint-disable max-len */
import React from 'react';

import { useShallow } from 'zustand/react/shallow';
import { BasicModelConfigEditor } from '@cozeloop/prompt-components';
import { CollapseCard } from '@cozeloop/components';
import { useModelList, useSpace } from '@cozeloop/biz-hooks-adapter';
import { ContentType } from '@cozeloop/api-schema/prompt';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tag, Tooltip, Typography } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { useBasicStore } from '@/store/use-basic-store';
import { useCompare } from '@/hooks/use-compare';

export function ModelConfigCard() {
  const { spaceIDWhenDemoSpaceItsPersonal } = useSpace();
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

  const service = useModelList(spaceIDWhenDemoSpaceItsPersonal);

  return (
    <CollapseCard
      title={<Typography.Text strong>模型配置</Typography.Text>}
      subInfo={
        multiModalError ? (
          <Tooltip
            content="所选模型不支持多模态，请调整变量类型或更换模型"
            theme="dark"
          >
            <Tag color="red" prefixIcon={<IconCozInfoCircle />}>
              模型不支持
            </Tag>
          </Tooltip>
        ) : null
      }
      defaultVisible
      key={`${modelConfig?.model_id}-${promptInfo?.prompt_commit?.commit_info?.version}-${promptInfo?.prompt_draft?.draft_info?.base_version}`}
    >
      <BasicModelConfigEditor
        value={modelConfig}
        onChange={config => {
          setModelConfig({ ...config });
        }}
        disabled={streaming || readonly}
        models={service.data?.models}
        onModelChange={setCurrentModel}
        modelSelectProps={{
          className: 'w-full',
          loading: service.loading,
        }}
        defaultActiveFirstModel={Boolean(
          !promptInfo?.prompt_key && !modelConfig?.model_id,
        )}
      />
    </CollapseCard>
  );
}
