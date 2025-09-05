/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
import { forwardRef, useEffect, useImperativeHandle, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
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
import { createLLMRun } from '@/utils/llm';
import {
  usePromptMockDataStore,
  type DebugMessage,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { isResponding, useLLMStreamRun } from '@/hooks/use-llm-stream-run';
import { MessageListRoundType } from '@/consts';
import peIcon from '@/assets/loop.svg';

import { MessageItem } from '../message-item';

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
  clearHistory: () => void;
}

export const ChatSubArea = forwardRef<ChatSubAreaRef, ChatSubAreaProps>(
  (
    { index = 0, times = 0, streamingSendEnd, className, acceptResult },
    ref,
  ) => {
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
    } = useLLMStreamRun();

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
      }).then(() => setHasResult(true));
    };

    const stopStreaming = () => {
      abort();
      if (streaming) {
        setCurrentHistory(list => [
          ...(list || []),
          {
            isEdit: false,
            id: messageId(),
            message: {
              role: Role.Assistant,
              content: smoothExecuteResult,
              tool_calls: toolCalls,
            },
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
              组 {index + 1}
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
                重试
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
                  采纳
                </Button>
              )}
            </Space>
          </Space>
        ) : null}

        <div className={styles['execute-sub-area-content']}>
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
                  hideOptimize: true,
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
                hideOptimize: true,
              }}
              tools={mockTools}
            />
          ) : null}

          {currentHistory?.length || streaming ? null : (
            <img
              src={peIcon}
              className={styles['execute-sub-area-content-img']}
            />
          )}
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
                停止响应
              </Button>
            </Space>
          ) : null}
        </Space>
      </div>
    );
  },
);
