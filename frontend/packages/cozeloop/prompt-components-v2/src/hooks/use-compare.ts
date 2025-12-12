/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable security/detect-object-injection */
/* eslint-disable complexity */
/* eslint-disable max-lines-per-function */
import { type SetStateAction } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import { isUndefined } from 'lodash-es';
import {
  type Message,
  type MockTool,
  type ModelConfig,
  type Tool,
  type ToolCallConfig,
  type VariableDef,
  type VariableVal,
} from '@cozeloop/api-schema/prompt';
import { type Model } from '@cozeloop/api-schema/llm-manage';

import { usePromptStore } from '@/store/use-prompt-store';
import {
  type DebugMessage,
  usePromptMockDataStore,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';

export const useCompare = (uid?: number) => {
  const {
    modelConfig,
    variables,
    currentModel,
    tools,
    messageList,
    setModelConfig,
    setCurrentModel,
    setVariables,
    setMessageList,
    setTools,
    toolCallConfig,
    setToolCallConfig,
  } = usePromptStore(
    useShallow(state => ({
      modelConfig: state.modelConfig,
      variables: state.variables,
      currentModel: state.currentModel,
      tools: state.tools,
      messageList: state.messageList,
      setModelConfig: state.setModelConfig,
      setCurrentModel: state.setCurrentModel,
      setVariables: state.setVariables,
      setMessageList: state.setMessageList,
      setTools: state.setTools,
      toolCallConfig: state.toolCallConfig,
      setToolCallConfig: state.setToolCallConfig,
    })),
  );

  const { streaming, setStreaming } = useBasicStore(
    useShallow(state => ({
      streaming: state.streaming,
      setStreaming: state.setStreaming,
    })),
  );

  const {
    compareConfig,
    mockVariables,
    historicMessage,
    mockTools,
    setMockTools,
    setMockVariables,
    setHistoricMessage,
    setHistoricMessageById,
    setMessageListById,
    setVariablesById,
    setToolsById,
    setModelConfigById,
    setToolCallConfigById,
    setMockVariablesById,
    setMockToolsById,
    setStreamingById,
    setCurrentModelById,
  } = usePromptMockDataStore(
    useShallow(state => ({
      mockVariables: state.mockVariables,
      historicMessage: state.historicMessage,
      mockTools: state.mockTools,
      compareConfig: state.compareConfig,
      setMockVariables: state.setMockVariables,
      setHistoricMessage: state.setHistoricMessage,
      setMockTools: state.setMockTools,
      setMessageListById: state.setMessageListById,
      setVariablesById: state.setVariablesById,
      setToolsById: state.setToolsById,
      setModelConfigById: state.setModelConfigById,
      setHistoricMessageById: state.setHistoricMessageById,
      setToolCallConfigById: state.setToolCallConfigById,
      setMockVariablesById: state.setMockVariablesById,
      setMockToolsById: state.setMockToolsById,
      setStreamingById: state.setStreamingById,
      setCurrentModelById: state.setCurrentModelById,
    })),
  );

  if (!isUndefined(uid)) {
    const compareItem = compareConfig?.groups?.[uid] || {};
    return {
      messageList: compareItem?.prompt_detail?.prompt_template?.messages?.map(
        (it: Message & { key?: string }, index) => {
          if (!it?.key) {
            return {
              ...it,
              key: `${uid}-${index}`,
            };
          }
          return it;
        },
      ),
      modelConfig: compareItem?.prompt_detail?.model_config,
      variables: compareItem?.prompt_detail?.prompt_template?.variable_defs,
      tools: compareItem?.prompt_detail?.tools,
      toolCallConfig: compareItem?.prompt_detail?.tool_call_config,
      mockVariables: compareItem?.debug_core?.mock_variables,
      mockTools: compareItem?.debug_core?.mock_tools,
      historicMessage: compareItem?.debug_core?.mock_contexts?.map(
        (it: DebugMessage) => {
          if (!it.id) {
            return { ...it, id: nanoid() };
          }
          return it;
        },
      ),
      streaming: compareItem?.streaming,
      currentModel: compareItem?.currentModel,
      setVariables: (v: SetStateAction<VariableDef[] | undefined>) =>
        setVariablesById(uid, v),
      setMockVariables: (v: SetStateAction<VariableVal[] | undefined>) =>
        setMockVariablesById(uid, v),
      setModelConfig: (v: SetStateAction<ModelConfig | undefined>) =>
        setModelConfigById(uid, v),
      setTools: (v: SetStateAction<Array<Tool> | undefined>) =>
        setToolsById(uid, v),
      setToolCallConfig: (v: SetStateAction<ToolCallConfig | undefined>) =>
        setToolCallConfigById(uid, v),
      setMockTools: (v: SetStateAction<Array<MockTool> | undefined>) =>
        setMockToolsById(uid, v),
      setMessageList: (
        v?: SetStateAction<Array<Message & { key?: string }> | undefined>,
      ) => setMessageListById(uid, v),
      setStreaming: (v: SetStateAction<boolean | undefined>) =>
        setStreamingById(uid, v),
      setCurrentModel: (v: SetStateAction<Model | undefined>) =>
        setCurrentModelById(uid, v),
      setHistoricMessage: (v?: SetStateAction<DebugMessage[]>) =>
        setHistoricMessageById(uid, v),
    };
  }

  return {
    messageList,
    modelConfig,
    variables,
    tools,
    streaming,
    mockVariables,
    mockTools,
    toolCallConfig,
    historicMessage,
    currentModel,
    setStreaming,
    setModelConfig,
    setCurrentModel,
    setVariables,
    setMessageList,
    setTools,
    setHistoricMessage,
    setMockVariables,
    setToolCallConfig,
    setMockTools,
  };
};
