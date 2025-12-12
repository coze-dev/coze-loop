/* eslint-disable security/detect-object-injection */
/* eslint-disable @typescript-eslint/no-non-null-assertion */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
/* eslint-disable max-lines-per-function */

import { type SetStateAction } from 'react';

import { isUndefined } from 'lodash-es';
import {
  type DebugStreamingRequest,
  type DebugToolCall,
  type Message,
  type Prompt,
  Role,
  ToolChoiceType,
} from '@cozeloop/api-schema/prompt';

import { usePromptStore } from '@/store/use-prompt-store';
import {
  usePromptMockDataStore,
  type DebugMessage,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { type LLMStreamResponse } from '@/hooks/use-llm-stream-run';
import { EVENT_NAMES } from '@/consts';

import {
  convertMultimodalMessageToSend,
  messageId,
  messagesHasSnippet,
} from './prompt';

export enum PromptExecuteStatus {
  Success = 0,
  Failed = 1,
  Canceled = 2,
}

export const sendPromptExecuteEvent = (
  status: PromptExecuteStatus, // 0 成功，1 失败，2 取消
  sendEvent?: (name: string, params: Record<string, unknown>) => void,
) => {
  const { promptInfo } = usePromptStore.getState();

  const { compareConfig } = usePromptMockDataStore.getState();
  sendEvent?.(EVENT_NAMES.prompt_execute, {
    prompt_id: `${promptInfo?.id || 'playground'}`,
    prompt_key: promptInfo?.prompt_key || 'playground',
    mode: compareConfig?.groups?.length ? 'compare' : 'normal',
    status,
  });
};

export const createLLMRun = ({
  startStream,
  uid,
  message,
  history,
  traceKey,
  notReport,
  singleRound,
  toolCalls: originToolCalls,
  setToolCalls: setOriginToolCalls,
  setHistoricChat: setOriginHistoricChat,
  sendEvent,
}: {
  startStream: (
    params: DebugStreamingRequest,
    stepDebug?: boolean | undefined,
    onStartSuccess?: () => void,
  ) => Promise<LLMStreamResponse>;
  uid?: number;
  message?: Message;
  history?: DebugMessage[];
  traceKey?: string;
  notReport?: boolean;
  singleRound?: boolean;
  toolCalls?: DebugToolCall[];
  setToolCalls?: React.Dispatch<React.SetStateAction<DebugToolCall[]>>;
  setHistoricChat?: React.Dispatch<React.SetStateAction<DebugMessage[]>>;
  sendEvent?: (name: string, params: Record<string, unknown>) => void;
}) => {
  const { readonly } = useBasicStore.getState();
  const {
    promptInfo,
    modelConfig: draftModelConfig,
    messageList: draftMessageList,
    toolCallConfig: draftToolCallConfig,
    currentModel: draftCurrentModel,
    variables: draftVariables,
    tools: draftTools,
    templateType,
  } = usePromptStore.getState();

  const {
    setHistoricMessage: setDraftHistoricMessage,
    compareConfig,
    userDebugConfig,
    mockTools: draftMockTools,
    mockVariables: draftMockVariables,
    toolCalls: draftToolCalls,
    setToolCalls: setDraftToolCalls,
    setHistoricMessageById,
  } = usePromptMockDataStore.getState();
  const compareItem = isUndefined(uid)
    ? undefined
    : compareConfig?.groups?.[uid];

  const messageList =
    compareItem?.prompt_detail?.prompt_template?.messages || draftMessageList;
  const toolCallConfig =
    compareItem?.prompt_detail?.tool_call_config || draftToolCallConfig;
  const modelConfig =
    compareItem?.prompt_detail?.model_config || draftModelConfig;
  const tools = compareItem?.prompt_detail?.tools || draftTools;
  const variables =
    compareItem?.prompt_detail?.prompt_template?.variable_defs ||
    draftVariables;
  const currentModel = draftCurrentModel;
  const mockVariables =
    compareItem?.debug_core?.mock_variables || draftMockVariables;

  const toolCalls = originToolCalls || draftToolCalls;
  const setToolCalls = setOriginToolCalls || setDraftToolCalls;

  const functionCllAble = currentModel?.ability?.function_call;

  const mockTools = functionCllAble
    ? compareItem?.debug_core?.mock_tools || draftMockTools
    : undefined;

  const setHistoricMessage =
    setOriginHistoricChat ||
    (isUndefined(uid)
      ? setDraftHistoricMessage
      : (list: SetStateAction<DebugMessage[]>) =>
          setHistoricMessageById(uid, list));

  const stepDebugger = !compareConfig?.groups?.length
    ? userDebugConfig?.single_step_debug
    : false;
  const singleStepDebug = currentModel?.ability?.function_call
    ? Boolean(stepDebugger)
    : false;

  const newHistoriceMessages: Message[] = (history || [])?.map(it => ({
    role: it.role,
    content: it.content,
    tool_calls: it.tool_calls?.map(item => item.tool_call!),
    parts: it.parts,
    tool_call_id: it.tool_call_id,
  }));

  const newMessage = message
    ? convertMultimodalMessageToSend(message)
    : message;

  const newPromptInfo = readonly
    ? { prompt_commit: promptInfo?.prompt_commit }
    : {
        prompt_draft: {
          draft_info: promptInfo?.prompt_draft?.draft_info,
          detail: {
            prompt_template: {
              ...promptInfo?.prompt_draft?.detail?.prompt_template,
              template_type: templateType?.value,
              messages: messageList,
              variable_defs: variables,
              has_snippet: messagesHasSnippet(messageList || []),
            },
            tools: functionCllAble ? tools : undefined,
            tool_call_config: toolCallConfig,
            model_config: modelConfig,
          },
        },
      };
  const newPrompt = {
    ...promptInfo,
    ...newPromptInfo,
  } as unknown as Prompt;
  const query: DebugStreamingRequest = {
    prompt: newPrompt,
    messages: newMessage
      ? [...newHistoriceMessages, newMessage]
      : newHistoriceMessages,
    variable_vals: mockVariables?.map(it => ({
      key: it.key,
      value: it.value ? it.value : undefined,
      placeholder_messages: it.placeholder_messages,
      multi_part_values: it.multi_part_values,
    })),
    mock_tools: mockTools,
    single_step_debug: singleStepDebug,
    debug_trace_key: traceKey,
  };

  const sendPromptExecuteInfo = () => {
    if (!notReport) {
      const isMultiModalFlag = Boolean(message?.parts?.length);
      sendEvent?.(EVENT_NAMES.prompt_execute_info, {
        prompt_id: `${promptInfo?.id || 'playground'}`,
        prompt_key: promptInfo?.prompt_key || 'playground',
        model_id: `${currentModel?.model_id || ''}`,
        function_enable: Boolean(currentModel?.ability?.function_call),
        function_call_open: toolCallConfig?.tool_choice !== ToolChoiceType.None,
        variables_count: variables?.length,
        json_mode_enable: currentModel?.ability?.json_mode,
        json_mode_open: currentModel?.ability?.json_mode
          ? modelConfig?.json_mode
          : false,
        single_round: singleRound,
        step_debugger_open: stepDebugger,
        multi_modal_able: currentModel?.ability?.multi_modal,
        is_multi_modal: isMultiModalFlag,
        compare_type: compareConfig?.groups?.length ? 'compare' : 'normal',
      });
    }
  };

  return startStream(query, Boolean(traceKey), sendPromptExecuteInfo)
    .then(res => {
      sendPromptExecuteEvent(PromptExecuteStatus.Success, sendEvent);

      if (!res.debugTrace) {
        const id = messageId();
        setHistoricMessage?.(list => [
          ...(list || []),
          {
            isEdit: false,
            id,
            role: Role.Assistant,
            content: res.message,
            debug_id: `${res.debugId}`,
            cost_ms: res.costInfo?.duration,
            output_tokens: res.costInfo?.outpotTokens,
            input_tokens: res.costInfo?.inputTokens,
            reasoning_content: res.reasoningContent,
            tool_calls: res.tools?.map(it => {
              if (stepDebugger) {
                const item = toolCalls?.find(
                  i => i?.tool_call?.id === it?.tool_call?.id,
                );
                return { ...it, mock_response: item?.mock_response };
              }
              return it;
            }),
            parts: res.parts,
          },
        ]);
      } else {
        setToolCalls?.(prev =>
          (res.tools || []).map(item => {
            const target = prev.find(
              i => i.tool_call?.id === item.tool_call?.id,
            );

            return target || item;
          }),
        );
      }
    })
    .catch(e => {
      console.log(e);
      sendPromptExecuteEvent(PromptExecuteStatus.Failed, sendEvent);
    });
};
