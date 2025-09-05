/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
/* eslint-disable max-params */
import React, { useEffect, useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import classNames from 'classnames';
import { getPlaceholderErrorContent } from '@cozeloop/prompt-components';
import {
  BenefitBanner,
  BenefitBannerScene,
} from '@cozeloop/biz-components-adapter';
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
import { MessageListGroupType } from '@/consts';

import { GroupSelectContext } from '../send-msg-area/group-select';
import { SendMsgArea } from '../send-msg-area';
import { ChatSubArea, type ChatSubAreaRef } from '../message-area/sub-area';
import {
  CompareMessageArea,
  type CompareMessageAreaRef,
} from '../message-area';

export function ExecuteArea() {
  const { setStreaming, streaming, groupType } = useBasicStore(
    useShallow(state => ({
      setStreaming: state.setStreaming,
      streaming: state.streaming,
      groupType: state.groupType,
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

  const [showMultiGroupResult, setShowMultiGroupResult] = useState(false);
  const isMultiGroup = groupType === MessageListGroupType.Multi;
  const messageAreaRef = useRef<CompareMessageAreaRef>(null);
  const [groupNum, setGroupNum] = useState(2);
  const [array, setArray] = useState<Array<{ key: string }>>([]);
  const textRunAreaRefs = useRef<ChatSubAreaRef[] | null[]>([]);
  const [acceptKey, setAcceptKey] = useState<string>(nanoid());

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
      return Toast.error('历史数据有空内容');
    }

    const placeholderHasError = messageList?.some(message => {
      if (message.role === Role.Placeholder) {
        return Boolean(getPlaceholderErrorContent(message, variables || []));
      }
      return false;
    });
    if (placeholderHasError) {
      return Toast.error('Placeholder 变量不存在或命名错误');
    }

    setHistoricMessage?.(history);

    if (isMultiGroup) {
      setShowMultiGroupResult(true);
      setTimeout(() => {
        textRunAreaRefs.current.forEach((tref: ChatSubAreaRef | null) => {
          tref?.sendMessage();
        });
      }, 300);
    } else {
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
    }
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
      Toast.error('请添加 Prompt 模板或输入提问内容');
      return;
    }

    const placeholderHasError = messageList?.some(msg => {
      if (msg.role === Role.Placeholder) {
        return Boolean(getPlaceholderErrorContent(msg, variables || []));
      }
      return false;
    });
    if (placeholderHasError) {
      return Toast.error('Placeholder 变量不存在或命名错误');
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

    if (isMultiGroup && (message?.content || message?.parts?.length)) {
      const hasResult = textRunAreaRefs.current.some(it => it?.hasResult);

      if (hasResult) {
        Toast.warning('请先采纳运行建议');
        return;
      }
    }

    if (message?.content || message?.parts?.length) {
      if (historyHasEmpty) {
        return Toast.error('历史数据有空内容');
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

      if (isMultiGroup) {
        setShowMultiGroupResult(true);
        setTimeout(() => {
          textRunAreaRefs.current.forEach((tref: ChatSubAreaRef | null) => {
            tref?.sendMessage();
          });
        }, 300);
      } else {
        const history = chatArray.map(it => ({
          role: it.role,
          content: it.content,
          parts: it.parts,
        }));
        runLLM(message, history);
      }
    } else if (chatArray.length) {
      const last = chatArray?.[chatArray.length - 1];
      if (isMultiGroup) {
        setShowMultiGroupResult(true);
        if (last?.role === Role.Assistant) {
          const newHistory = chatArray.slice(0, chatArray.length - 1);
          setHistoricMessage?.(newHistory);
        }
        setTimeout(() => {
          textRunAreaRefs.current.forEach((tref: ChatSubAreaRef | null) => {
            tref?.sendMessage();
          });
        }, 300);
      } else {
        if (last.role === Role.Assistant) {
          rerunSendMessage();
        } else {
          if (historyHasEmpty && chatArray.length > 2) {
            return Toast.error('历史数据有空内容');
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
      }
    } else {
      if (isMultiGroup) {
        setShowMultiGroupResult(true);
        setTimeout(() => {
          textRunAreaRefs.current.forEach((tref: ChatSubAreaRef | null) => {
            tref?.sendMessage();
          });
        }, 300);
      } else {
        runLLM(undefined, []);
      }
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

  useEffect(() => {
    setArray(arr => {
      if (arr.length >= groupNum) {
        return arr.slice(0, groupNum);
      } else {
        const newArr = new Array(groupNum - arr.length).fill(0);
        const newArray = newArr.map(() => ({ key: messageId() }));
        return arr.concat(newArray);
      }
    });
  }, [groupNum]);

  const streamingSendEnd = () => {
    const hasStreaming = array.some(
      (_it, index) => textRunAreaRefs.current?.[index]?.streaming,
    );
    setStreaming?.(hasStreaming);
  };

  const onClearHistory = () => {
    textRunAreaRefs.current.forEach((tref: ChatSubAreaRef | null) => {
      tref?.clearHistory();
    });
    setShowMultiGroupResult(false);
  };

  useEffect(() => {
    setShowMultiGroupResult(false);
  }, [groupType]);

  return (
    <div
      className="flex-1 box-border flex flex-col overflow-hidden gap-1"
      style={{ padding: '18px 0 24px 18px' }}
    >
      <CompareMessageArea
        ref={messageAreaRef}
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
        isMultiGroup={isMultiGroup}
      />

      {isMultiGroup ? (
        <div
          className={classNames('flex gap-1 overflow-x-auto h-1/2 px-1 pt-1', {
            '!h-0': !showMultiGroupResult,
            'opacity-0': !showMultiGroupResult,
          })}
          key={acceptKey}
        >
          {array.map((item, index) => (
            <ChatSubArea
              className="!w-1/2 !min-w-[400px] flex-shrink-0"
              key={item.key}
              index={index}
              times={groupNum}
              ref={el => (textRunAreaRefs.current[index] = el)}
              streamingSendEnd={streamingSendEnd}
              acceptResult={message => {
                if (message) {
                  setHistoricMessage?.(prev => [...prev, message]);
                  setArray(arr => arr.map(it => ({ ...it, key: messageId() })));
                  messageAreaRef?.current?.scrollToBottom();
                  setShowMultiGroupResult(false);
                  setAcceptKey(nanoid());
                }
              }}
            />
          ))}
        </div>
      ) : null}
      <BenefitBanner
        closable={false}
        className="mb-2 mr-6"
        scene={BenefitBannerScene.PromptDetail}
      />
      <GroupSelectContext.Provider value={{ groupNum, setGroupNum }}>
        <SendMsgArea
          streaming={streaming}
          onMessageSend={sendMessage}
          stopStreaming={stopStreaming}
          onClearHistory={onClearHistory}
          isSingleRound={showMultiGroupResult}
        />
      </GroupSelectContext.Provider>
    </div>
  );
}
