/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable security/detect-object-injection */
/* eslint-disable max-params */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type Dispatch, type SetStateAction } from 'react';

import { subscribeWithSelector } from 'zustand/middleware';
import { create } from 'zustand';
import { nanoid } from 'nanoid';
import { produce } from 'immer';
import {
  type DebugMessage as BasicDebugMessage,
  type MockTool,
  type VariableVal,
  type DebugToolCall,
  type ContentPart,
  type DebugConfig,
  type CompareConfig,
  type CompareGroup,
  type Message,
  type VariableDef,
  type Tool,
  type ModelConfig,
  type ToolCallConfig,
  ToolChoiceType,
} from '@cozeloop/api-schema/prompt';
import { type Model } from '@cozeloop/api-schema/llm-manage';

export interface DebugMessage extends BasicDebugMessage {
  id?: string;
  isEdit?: boolean;
}

export type FileItemStatus =
  | 'success'
  | 'uploadFail'
  | 'validateFail'
  | 'validating'
  | 'uploading'
  | 'wait';

export interface ContentPartLoop extends ContentPart {
  uid?: string;
  status?: FileItemStatus;
}

export interface CompareGroupLoop extends CompareGroup {
  streaming?: boolean;
  currentModel?: Model;
}

export type CompareConfigLoop = Omit<CompareConfig, 'groups'> & {
  groups?: CompareGroupLoop[];
};

export interface PromptMockDataState {
  historicMessage?: DebugMessage[];
  mockVariables?: VariableVal[];
  mockTools?: MockTool[];
  userDebugConfig?: DebugConfig;
  compareConfig?: CompareConfigLoop;
  toolCalls?: DebugToolCall[];
}

type PromptActionType<S> = Dispatch<SetStateAction<S>>;

interface PromptMockDataAction {
  setHistoricMessage: PromptActionType<DebugMessage[]>;
  setUserDebugConfig: PromptActionType<DebugConfig>;
  setMockVariables: PromptActionType<VariableVal[]>;
  setMockTools: PromptActionType<MockTool[]>;
  setCompareConfig: PromptActionType<CompareConfigLoop | undefined>;
  setToolCalls: PromptActionType<DebugToolCall[]>;
  addNewComparePrompt: (compareGroup: CompareGroupLoop) => void;
  deleteComparePrompt: (index: number) => void;
  setMessageListById: (
    index: number,
    messageList?: SetStateAction<Array<Message & { key?: string }> | undefined>,
  ) => void;
  setVariablesById: (
    index: number,
    variables?: SetStateAction<VariableDef[] | undefined>,
  ) => void;
  setToolsById: (
    index: number,
    tools?: SetStateAction<Tool[] | undefined>,
  ) => void;
  setModelConfigById: (
    index: number,
    config?: SetStateAction<ModelConfig | undefined>,
  ) => void;
  setMockVariablesById: (
    index: number,
    mockVariables: SetStateAction<VariableVal[] | undefined>,
  ) => void;
  setHistoricMessageById: (
    index: number,
    historicMessage?: SetStateAction<DebugMessage[]>,
  ) => void;
  setMockToolsById: (
    index: number,
    mockTools: SetStateAction<MockTool[] | undefined>,
  ) => void;
  setToolCallConfigById: (
    index: number,
    toolCallConfig: SetStateAction<ToolCallConfig | undefined>,
  ) => void;
  setStreamingById: (
    index: number,
    streaming: SetStateAction<boolean | undefined>,
  ) => void;
  setCurrentModelById: (
    index: number,
    model: SetStateAction<Model | undefined>,
  ) => void;
  clearMockdataStore: () => void;
}

const normalUpdateFunc = (
  value: any,
  key: string,
  set: any,
  get: () => PromptMockDataState,
) => {
  if (typeof value === 'function') {
    const newValue = value(get()[key as keyof PromptMockDataState]);
    set(
      produce((state: PromptMockDataState) => {
        state[key as keyof PromptMockDataState] = newValue;
      }),
    );
  } else {
    set(
      produce((state: PromptMockDataState) => {
        state[key as keyof PromptMockDataState] = value;
      }),
    );
  }
};

export const usePromptMockDataStore = create<
  PromptMockDataState & PromptMockDataAction
>()(
  subscribeWithSelector((set, get) => ({
    historicMessage: [],
    userDebugConfig: {
      single_step_debug: false,
    },
    mockVariables: [],
    mockTools: [],
    compareConfig: { groups: [] },
    toolCalls: [],
    setToolCalls: value => normalUpdateFunc(value, 'toolCalls', set, get),
    setCompareConfig: value =>
      normalUpdateFunc(value, 'compareConfig', set, get),
    setMockVariables: value =>
      normalUpdateFunc(value, 'mockVariables', set, get),
    setMockTools: value => normalUpdateFunc(value, 'mockTools', set, get),
    setUserDebugConfig: value =>
      normalUpdateFunc(value, 'userDebugConfig', set, get),
    setHistoricMessage: value =>
      normalUpdateFunc(value, 'historicMessage', set, get),
    addNewComparePrompt: (compareGroup: CompareGroup) =>
      set(
        produce((state: PromptMockDataState) => {
          state.compareConfig?.groups?.push(compareGroup);
        }),
      ),
    deleteComparePrompt: (index: number) =>
      set(
        produce((state: PromptMockDataState) => {
          state.compareConfig?.groups?.splice(index, 1);
        }),
      ),
    setMessageListById: (
      index: number,
      messageList?: SetStateAction<
        Array<Message & { key?: string }> | undefined
      >,
    ) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }

          const group = groups[index];
          if (!group?.prompt_detail?.prompt_template) {
            return;
          }

          let newValue: Array<Message & { key?: string }> | undefined = [];
          if (typeof messageList === 'function') {
            const currentMessages =
              group.prompt_detail.prompt_template.messages;
            newValue = messageList(currentMessages);
          } else {
            newValue = messageList;
          }
          group.prompt_detail.prompt_template.messages = newValue?.map(it => {
            if (!it.key) {
              return { ...it, key: nanoid() };
            }
            return it;
          });
        }),
      ),
    setVariablesById: (
      index: number,
      variables?: SetStateAction<VariableDef[] | undefined>,
    ) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }
          const group = groups[index];
          if (!group?.prompt_detail?.prompt_template) {
            return;
          }
          let newValue: VariableDef[] | undefined = [];
          if (typeof variables === 'function') {
            const currentVariables =
              group.prompt_detail.prompt_template.variable_defs;
            newValue = variables(currentVariables || []);
          } else {
            newValue = variables;
          }
          group.prompt_detail.prompt_template.variable_defs = newValue;
        }),
      ),
    setToolsById: (index: number, tools?: SetStateAction<Tool[] | undefined>) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }
          const group = groups[index];
          if (!group?.prompt_detail) {
            return;
          }
          let newValue: Tool[] | undefined = [];
          if (typeof tools === 'function') {
            const currentTools = group.prompt_detail.tools;
            newValue = tools(currentTools || []);
          } else {
            newValue = tools;
          }
          group.prompt_detail.tools = newValue;
        }),
      ),
    setModelConfigById: (
      index: number,
      config?: SetStateAction<ModelConfig | undefined>,
    ) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }
          const group = groups[index];
          if (!group?.prompt_detail) {
            return;
          }
          let newValue: ModelConfig | undefined = {};
          if (typeof config === 'function') {
            const currentConfig = group.prompt_detail.model_config;
            newValue = config(currentConfig || {});
          } else {
            newValue = config;
          }
          group.prompt_detail.model_config = newValue;
        }),
      ),
    setMockVariablesById: (
      index: number,
      mockVariables: SetStateAction<VariableVal[] | undefined>,
    ) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }
          const group = groups[index];
          if (!group?.debug_core) {
            return;
          }
          let newValue: VariableVal[] | undefined = [];
          if (typeof mockVariables === 'function') {
            const currentMockVariables = group.debug_core.mock_variables;
            newValue = mockVariables(currentMockVariables || []);
          } else {
            newValue = mockVariables;
          }
          group.debug_core.mock_variables = newValue;
        }),
      ),
    setHistoricMessageById: (
      index: number,
      historicMessage?: SetStateAction<DebugMessage[]>,
    ) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }
          const group = groups[index];
          if (!group?.debug_core) {
            return;
          }
          let newValue: DebugMessage[] | undefined = [];
          if (typeof historicMessage === 'function') {
            const currentHistoricChat = group.debug_core.mock_contexts;
            newValue = historicMessage(currentHistoricChat || []);
          } else {
            newValue = historicMessage || [];
          }

          group.debug_core.mock_contexts = newValue.map(it => {
            if (!it.id) {
              return {
                ...it,
                id: nanoid(),
              };
            }
            return it;
          });
        }),
      ),
    setMockToolsById: (
      index: number,
      mockTools: SetStateAction<MockTool[] | undefined>,
    ) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }
          const group = groups[index];
          if (!group?.debug_core) {
            return;
          }
          let newValue: MockTool[] | undefined = [];
          if (typeof mockTools === 'function') {
            const currentMockTools = group.debug_core.mock_tools;
            newValue = mockTools(currentMockTools || []);
          } else {
            newValue = mockTools;
          }
          group.debug_core.mock_tools = newValue;
        }),
      ),
    setToolCallConfigById: (
      index: number,
      toolCallConfig: SetStateAction<ToolCallConfig | undefined>,
    ) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }
          const group = groups[index];
          if (!group?.prompt_detail) {
            return;
          }
          let newValue: ToolCallConfig | undefined = {};
          if (typeof toolCallConfig === 'function') {
            const currentToolCallConfig = group.prompt_detail.tool_call_config;
            newValue = toolCallConfig(
              currentToolCallConfig || { tool_choice: ToolChoiceType.Auto },
            );
          } else {
            newValue = toolCallConfig;
          }
          group.prompt_detail.tool_call_config = newValue;
        }),
      ),
    setStreamingById: (
      index: number,
      streaming: SetStateAction<boolean | undefined>,
    ) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }
          const group = groups[index];
          if (!group) {
            return;
          }
          let newValue: boolean | undefined = false;
          if (typeof streaming === 'function') {
            const currentStreaming = group.streaming;
            newValue = streaming(currentStreaming || false);
          } else {
            newValue = streaming;
          }
          group.streaming = newValue;
        }),
      ),
    setCurrentModelById: (
      index: number,
      model: SetStateAction<Model | undefined>,
    ) =>
      set(
        produce((state: PromptMockDataState) => {
          const groups = state.compareConfig?.groups;
          if (!groups || index < 0 || index >= groups.length) {
            return;
          }
          const group = groups[index];
          if (!group) {
            return;
          }
          let newValue: Model | undefined = {};
          if (typeof model === 'function') {
            const { currentModel } = group;
            newValue = model(currentModel || {});
          } else {
            newValue = model;
          }

          group.currentModel = newValue;
        }),
      ),
    clearMockdataStore: () =>
      set(
        produce((state: PromptMockDataState) => {
          state.historicMessage = [];
          state.mockVariables = [];
          state.mockTools = [];
          state.userDebugConfig = {
            single_step_debug: false,
          };
          state.compareConfig = { groups: [] };
          state.toolCalls = [];
        }),
      ),
  })),
);
