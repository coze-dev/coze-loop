// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
/* eslint-disable max-params */
import React, { useEffect } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { getPlaceholderErrorContent } from '@cozeloop/prompt-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Message, Role } from '@cozeloop/api-schema/prompt';
import { Toast } from '@coze-arch/coze-design';

import { convertMultimodalMessage, messageId } from '@/utils/prompt';
import { createLLMRun } from '@/utils/llm';
import { usePromptStore } from '@/store/use-prompt-store';
import {
  type DebugMessage,
  usePromptMockDataStore,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { isResponding, useLLMStreamRun } from '@/hooks/use-llm-stream-run';

import { SendMsgArea } from '../send-msg-area';
import { CompareMessageArea } from '../message-area';

export function ExecuteArea() {
  const { setStreaming, streaming } = useBasicStore(
    useShallow(state => ({
      setStreaming: state.setStreaming,
      streaming: state.streaming,
    })),
  );
  const { messageList, variables } = usePromptStore(
    useShallow(state => ({
      modelConfig: state.modelConfig,
      messageList: state.messageList,
      variables: state.variables,
    })),
  );
  const {
    setHistoricMessage,
    historicMessage = [],
    toolCalls,
    setToolCalls,
    userDebugConfig,
  } = usePromptMockDataStore(
    useShallow(state => ({
      setHistoricMessage: state.setHistoricMessage,
      historicMessage: state.historicMessage,
      toolCalls: state.toolCalls,
      setToolCalls: state.setToolCalls,
      userDebugConfig: state.userDebugConfig,
    })),
  );

  const stepDebugger = userDebugConfig?.single_step_debug;

  const {
    startStream,
    smoothExecuteResult,
    abort,
    stepDebuggingTrace,
    respondingStatus,
    reasoningContentResult,
    stepDebuggingContent,
    debugId,
    resetInfo,
    streamRefTools,
  } = useLLMStreamRun();

  const runLLM = (
    queryMsg?: Message,
    history?: DebugMessage[],
    traceKey?: string,
    notReport?: boolean,
  ) => {
    setStreaming?.(true);

    createLLMRun({
      startStream,
      message: queryMsg,
      history,
      traceKey,
      notReport,
      singleRound: false,
    });
  };

  const lastIndex = historicMessage.length - 1;

  const rerunSendMessage = () => {
    const history = historicMessage.slice(0, lastIndex);
    const lastContent = historicMessage?.[lastIndex - 1];
    const last = lastContent;

    const chatArray = history.filter(v => Boolean(v)) as Message[];

    const historyHasEmpty = Boolean(
      chatArray.length &&
        chatArray.some(it => {
          if (it?.parts?.length) {
            return false;
          }
          return !it?.content && !it.tool_calls?.length;
        }),
    );

    if (historyHasEmpty) {
      return Toast.error(I18n.t('historical_data_has_empty_content'));
    }

    const placeholderHasError = messageList?.some(message => {
      if (message.role === Role.Placeholder) {
        return Boolean(getPlaceholderErrorContent(message, variables || []));
      }
      return false;
    });
    if (placeholderHasError) {
      return Toast.error(I18n.t('placeholder_var_error'));
    }

    setHistoricMessage?.(history);
    const newHistory = historicMessage
      .slice(0, lastIndex - 1)
      .map(it => ({
        id: it.id,
        role: it?.role,
        content: it?.content,
        parts: it?.parts,
      }))
      .filter(v => Boolean(v));

    runLLM(
      last
        ? {
            content: last.content,
            role: last.role,
            parts: last.parts,
          }
        : undefined,
      newHistory,
    );
  };

  const stopStreaming = () => {
    abort();
    if (streaming) {
      setHistoricMessage?.(list => [
        ...(list || []),
        {
          isEdit: false,
          id: messageId(),
          role: Role.Assistant,
          content: smoothExecuteResult,
          tool_calls: toolCalls,
          debug_id: `${debugId || ''}`,
        },
      ]);
    }
    setStreaming?.(false);
    resetInfo();
  };

  const sendMessage = (message?: Message) => {
    if (!messageList?.length && !(message?.content || message?.parts?.length)) {
      Toast.error(I18n.t('add_prompt_tpl_or_input_question'));
      return;
    }

    const placeholderHasError = messageList?.some(msg => {
      if (msg.role === Role.Placeholder) {
        return Boolean(getPlaceholderErrorContent(msg, variables || []));
      }
      return false;
    });
    if (placeholderHasError) {
      return Toast.error(I18n.t('placeholder_var_execute_error'));
    }
    const chatArray = historicMessage.filter(v => Boolean(v));
    const historyHasEmpty = Boolean(
      chatArray.length &&
        chatArray.some(it => {
          if (it?.parts?.length) {
            return false;
          }
          return !it?.content && !it.tool_calls?.length;
        }),
    );

    if (message?.content || message?.parts?.length) {
      if (historyHasEmpty) {
        return Toast.error(I18n.t('historical_data_has_empty_content'));
      }

      if (message) {
        const newMessage = convertMultimodalMessage(message);
        setHistoricMessage?.(list => [
          ...(list || []),
          {
            isEdit: false,
            id: messageId(),
            role: newMessage.role,
            content: newMessage.content,
            parts: newMessage.parts,
          },
        ]);
      }

      const history = chatArray.map(it => ({
        role: it.role,
        content: it.content,
        parts: it.parts,
      }));
      runLLM(message, history);
    } else if (chatArray.length) {
      const last = chatArray?.[chatArray.length - 1];
      if (last.role === Role.Assistant) {
        rerunSendMessage();
      } else {
        if (historyHasEmpty && chatArray.length > 2) {
          return Toast.error(I18n.t('historical_data_has_empty_content'));
        }
        const history = chatArray.slice(0, chatArray.length - 1).map(it => ({
          role: it.role,
          content: it.content,
          parts: it.parts,
        }));
        runLLM(
          { content: last.content, role: last.role, parts: last.parts },
          history,
        );
      }
    } else {
      runLLM(undefined, []);
    }
  };

  const stepSendMessage = () => {
    const newHistory = historicMessage
      .filter(v => Boolean(v))
      .map(it => ({
        id: it.id,
        role: it?.role,
        content: it?.content,
        parts: it?.parts,
      }));

    const toolsHistory: DebugMessage[] = (toolCalls || [])
      .map(it => [
        {
          content: stepDebuggingContent,
          role: Role.Assistant,
          tool_calls: [it],
          id: messageId(),
        },
        {
          id: messageId(),
          content: it.mock_response || '',
          role: Role.Tool,
          tool_call_id: it?.tool_call?.id,
        },
      ])
      .flat();

    setStreaming?.(true);
    runLLM(undefined, [...newHistory, ...toolsHistory], stepDebuggingTrace);
  };

  useEffect(() => {
    if (!isResponding(respondingStatus)) {
      if (!stepDebuggingTrace) {
        setStreaming?.(false);
        setToolCalls?.([]);
      }
    } else {
      setStreaming?.(true);
    }
  }, [respondingStatus, stepDebuggingTrace]);

  return (
    <div
      className="flex-1 box-border flex flex-col overflow-hidden"
      style={{ padding: '18px 0 24px 18px' }}
    >
      <CompareMessageArea
        streaming={streaming}
        streamingMessage={smoothExecuteResult}
        historicMessage={historicMessage}
        setHistoricMessage={setHistoricMessage}
        toolCalls={streaming && !stepDebugger ? streamRefTools : toolCalls}
        reasoningContentResult={reasoningContentResult}
        rerunLLM={rerunSendMessage}
        stepDebuggingTrace={stepDebuggingTrace}
        setToolCalls={setToolCalls}
        stepSendMessage={stepSendMessage}
      />
      <SendMsgArea
        streaming={streaming}
        onMessageSend={sendMessage}
        stopStreaming={stopStreaming}
      />
    </div>
  );
}
