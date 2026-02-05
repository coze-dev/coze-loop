// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
import { forwardRef, useEffect, useImperativeHandle, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type Message,
  Role,
  type DebugToolCall,
} from '@cozeloop/api-schema/prompt';
import {
  IconCozArrowForward,
  IconCozRefresh,
  IconCozStopCircle,
} from '@coze-arch/coze-design/icons';
import { Button, Space, Tag } from '@coze-arch/coze-design';

import { messageId } from '@/utils/prompt';
import {
  createLLMRun,
  PromptExecuteStatus,
  sendPromptExecuteEvent,
} from '@/utils/llm';
import {
  usePromptMockDataStore,
  type DebugMessage,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { isResponding, useLLMStreamRun } from '@/hooks/use-llm-stream-run';
import { MessageListRoundType } from '@/consts';
import peIcon from '@/assets/loop.svg';

import { usePromptDevProviderContext } from '../prompt-provider';
import { MessageItem } from './message-item';

import styles from './index.module.less';

interface ChatSubAreaProps {
  index: number;
  times?: number;
  className?: string;
  streamingSendEnd?: () => void;
  acceptResult?: (message?: DebugMessage) => void;
}

export interface ChatSubAreaRef {
  streaming?: boolean;
  hasResult?: boolean;
  sendMessage: (userQuery?: string) => void;
  stopStreaming: () => void;
  clearHistory: () => void;
}

export const ChatSubArea = forwardRef<ChatSubAreaRef, ChatSubAreaProps>(
  (
    { index = 0, times = 0, streamingSendEnd, className, acceptResult },
    ref,
  ) => {
    const { debugAreaConfig, sendEvent } = usePromptDevProviderContext();
    const {
      setStreaming: setAllStreaming,
      roundType,
      streaming: allStreaming,
    } = useBasicStore(
      useShallow(state => ({
        streaming: state.streaming,
        setStreaming: state.setStreaming,
        roundType: state.roundType,
      })),
    );

    const { historicChat, mockTools } = usePromptMockDataStore(
      useShallow(state => ({
        userDebugConfig: state.userDebugConfig,
        historicChat: state.historicMessage,
        mockTools: state.mockTools,
      })),
    );

    const isSingleRound = roundType === MessageListRoundType.Single;

    const [streaming, setStreaming] = useState(false);
    const [hasResult, setHasResult] = useState(false);
    const [toolCalls, setToolCalls] = useState<DebugToolCall[]>([]);
    const [currentHistory, setCurrentHistory] = useState<DebugMessage[]>([]);

    const {
      startStream,
      smoothExecuteResult,
      abort,
      stepDebuggingTrace,
      stepDebuggingContent,
      respondingStatus,
      debugId,
      reasoningContentResult,
    } = useLLMStreamRun(undefined, () =>
      sendPromptExecuteEvent(PromptExecuteStatus.Canceled, sendEvent),
    );

    const runLLM = (
      queryMsg?: Message,
      history?: DebugMessage[],
      traceKey?: string,
    ) => {
      setAllStreaming(true);
      setStreaming(true);
      createLLMRun({
        startStream,
        message: queryMsg,
        history,
        traceKey,
        notReport: index > 0,
        singleRound: true,
        setToolCalls,
        setHistoricChat: setCurrentHistory,
        toolCalls,
        sendEvent,
      })
        .then(() => setHasResult(true))
        .catch(() => setHasResult(false));
    };

    const stopStreaming = () => {
      abort();
      if (streaming) {
        setCurrentHistory(list => [
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

        if (smoothExecuteResult || toolCalls.length) {
          setHasResult(true);
        }
      }
      setStreaming?.(false);
      setTimeout(() => streamingSendEnd?.(), 500);
    };

    const stepSendMessage = () => {
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
            tool_call_id: it.tool_call?.id,
          },
        ])
        .flat();

      setStreaming?.(true);
      const oldHistory = (historicChat || [])
        .filter(v => Boolean(v))
        .map(it => ({
          id: it?.id,
          role: it?.role,
          content: it?.content,
          parts: it?.parts,
        }));
      runLLM(undefined, [...oldHistory, ...toolsHistory], stepDebuggingTrace);
    };

    const sendMessage = () => {
      setCurrentHistory([]);
      setHasResult(false);
      setStreaming(true);
      setToolCalls([]);
      const oldHistory = (historicChat || [])
        .filter(v => Boolean(v))
        .map(it => ({
          id: it?.id,
          role: it?.role,
          content: it?.content,
          parts: it?.parts,
        }));
      runLLM(undefined, oldHistory);
    };

    useImperativeHandle(ref, () => ({
      streaming,
      hasResult,
      sendMessage,
      stopStreaming,
      clearHistory: () => {
        setCurrentHistory([]);
        setHasResult(false);
        setStreaming(false);
      },
    }));

    useEffect(() => {
      if (!isResponding(respondingStatus) && streaming) {
        if (!stepDebuggingTrace) {
          setStreaming(false);
          setTimeout(() => streamingSendEnd?.(), 500);
        }
      }
    }, [respondingStatus, stepDebuggingTrace, streaming]);

    return (
      <div
        className={classNames(
          'border border-solid coz-stroke-primary rounded-lg flex flex-col',
          className,
        )}
      >
        {times > 1 ? (
          <Space className="px-2 py-1 items-center justify-between w-full box-border">
            <Tag size="small" color="primary">
              {I18n.t('control_group')} {index + 1}
            </Tag>
            <Space>
              <Button
                color="secondary"
                size="small"
                disabled={streaming}
                onClick={() => {
                  sendMessage();
                }}
                icon={<IconCozRefresh />}
              >
                {I18n.t('retry')}
              </Button>

              {isSingleRound ? null : (
                <Button
                  color="highlight"
                  size="small"
                  disabled={
                    !hasResult ||
                    streaming ||
                    allStreaming ||
                    Boolean(stepDebuggingTrace)
                  }
                  onClick={() =>
                    acceptResult?.(currentHistory?.[currentHistory.length - 1])
                  }
                  icon={<IconCozArrowForward className="rotate-180" />}
                >
                  {I18n.t('accept')}
                </Button>
              )}
            </Space>
          </Space>
        ) : null}

        <div
          className={styles['execute-sub-area-content']}
          style={{
            backgroundImage:
              !currentHistory?.length && !streaming
                ? `url(${debugAreaConfig?.emptyLogoUrl || peIcon})`
                : '',
          }}
        >
          {currentHistory?.length ? (
            currentHistory?.map((item: DebugMessage) => (
              <MessageItem
                key={item.id}
                item={item || {}}
                smooth={false}
                btnConfig={{
                  hideEdit: true,
                  hideDelete: true,
                  hideMessageTypeSelect: true,
                }}
                tools={mockTools}
              />
            ))
          ) : streaming || smoothExecuteResult ? (
            <MessageItem
              streaming
              key="streaming"
              item={{
                role: Role.Assistant,
                content: smoothExecuteResult || '',
                tool_calls: toolCalls,
                reasoning_content: reasoningContentResult,
              }}
              smooth
              stepDebuggingTrace={stepDebuggingTrace}
              setToolCalls={setToolCalls}
              stepSendMessage={stepSendMessage}
              btnConfig={{
                hideEdit: true,
                hideDelete: true,
                hideMessageTypeSelect: true,
              }}
              tools={mockTools}
            />
          ) : null}
        </div>
        <Space className="justify-center h-[28px] shrink-0 w-full">
          {streaming ? (
            <Space align="center">
              <Button
                color="primary"
                icon={<IconCozStopCircle />}
                size="mini"
                onClick={stopStreaming}
              >
                {I18n.t('stop_respond')}
              </Button>
            </Space>
          ) : null}
        </Space>
      </div>
    );
  },
);
