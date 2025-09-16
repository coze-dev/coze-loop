// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/max-line-per-function */
import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from 'react';

import classNames from 'classnames';
import { useDebounceFn } from 'ahooks';
import {
  ContentType,
  type DebugToolCall,
  type ModelConfig,
  Role,
} from '@cozeloop/api-schema/prompt';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';

import { scrollToBottom } from '@/utils/prompt';
import { type DebugMessage } from '@/store/use-mockdata-store';
import { useCompare } from '@/hooks/use-compare';
import peIcon from '@/assets/loop.svg';

import { MessageItem } from '../message-item';

import styles from './index.module.less';

interface CompareMessageAreaProps {
  uid?: number;
  className?: string;
  streaming?: boolean;
  modelConfig?: ModelConfig;
  rerunLLM?: () => void;
  streamingMessage?: string;
  historicMessage?: DebugMessage[];
  setHistoricMessage?: Dispatch<SetStateAction<DebugMessage[]>>;
  toolCalls?: DebugToolCall[] | undefined;
  reasoningContentResult?: string;
  stepDebuggingTrace?: string;
  setToolCalls?: Dispatch<SetStateAction<DebugToolCall[]>>;
  stepSendMessage?: () => void;
  isMultiGroup?: boolean;
}

export interface CompareMessageAreaRef {
  scrollToBottom: () => void;
}

export const CompareMessageArea = forwardRef<
  CompareMessageAreaRef,
  CompareMessageAreaProps
>(
  (
    {
      uid,
      className,
      rerunLLM,
      streaming,
      streamingMessage,
      toolCalls,
      reasoningContentResult,
      stepDebuggingTrace,
      setToolCalls,
      stepSendMessage,
      isMultiGroup,
    }: CompareMessageAreaProps,
    ref,
  ) => {
    const {
      messageList,
      modelConfig,
      currentModel,
      mockTools,
      historicMessage,
      setHistoricMessage,
      setMessageList,
      mockVariables,
    } = useCompare(uid);

    const isMultiModal = currentModel?.ability?.multi_modal;
    const domRef = useRef<HTMLDivElement>(null);
    const [shouldAutoScroll, setShouldAutoScroll] = useState(true);
    const [showScrollButton, setShowScrollButton] = useState(false);
    const [buttonStyle, setButtonStyle] = useState({});
    const [isInScroll, setIsInScroll] = useState(false);
    const buttonRef = useRef<HTMLDivElement>(null);

    const historyChatListStr = JSON.stringify(historicMessage);

    const updateEditableByIdx = useCallback(
      (editable: boolean, idx: number) => {
        const historyChatList =
          historicMessage?.map((it, index) => {
            if (index === idx) {
              return { ...it, isEdit: editable };
            }
            return { ...it, isEdit: false };
          }) || [];
        setHistoricMessage?.([...historyChatList]);
      },
      [historyChatListStr],
    );

    const updateTypeByIdx = useCallback(
      (currentType: Role, idx: number) => {
        const historyChatList =
          historicMessage?.map(
            (it: DebugMessage, index: number): DebugMessage => {
              if (index === idx) {
                const { parts = [], content } = it || {};
                const newContent =
                  parts?.find(item => item?.type === ContentType.Text)?.text ||
                  content;
                return {
                  ...it,
                  role: currentType,
                  parts: undefined,
                  content: newContent,
                  isEdit: false,
                };
              }
              return { ...it, isEdit: false };
            },
          ) || [];
        setHistoricMessage?.([...historyChatList]);
      },
      [historyChatListStr],
    );

    const updateMessageItemByIdx = useCallback(
      (item: DebugMessage, idx: number) => {
        const historyChatList =
          historicMessage?.map(
            (it: DebugMessage, index: number): DebugMessage => {
              if (index === idx) {
                return { ...it, ...item, isEdit: false };
              }
              return { ...it, isEdit: false };
            },
          ) || [];
        setHistoricMessage?.([...historyChatList]);
      },
      [historyChatListStr],
    );

    const deleteChatByIdx = useCallback(
      (idx: number) => {
        const historyChatList = historicMessage?.slice() || [];
        historyChatList?.splice(idx, 1);
        setHistoricMessage?.([...historyChatList]);
      },
      [historyChatListStr],
    );

    useEffect(() => {
      if ((streamingMessage || reasoningContentResult) && shouldAutoScroll) {
        scrollToBottom(domRef); // 当streamingMessage变化时，自动滚动到底部
      }
    }, [streamingMessage, reasoningContentResult, shouldAutoScroll]); // 依赖于streamingMessage和shouldAutoScroll

    const { run: handleScrollStop } = useDebounceFn(
      () => {
        setIsInScroll(false);
      },
      { wait: 300 },
    );

    useEffect(() => {
      const handleUserScroll = () => {
        if (!domRef.current) {
          setShouldAutoScroll(false);
          return;
        }

        const { scrollTop, scrollHeight, clientHeight } = domRef.current;
        const isAtBottom = scrollHeight - clientHeight <= scrollTop + 1;

        setShowScrollButton(!isAtBottom);
        setIsInScroll(true);
        handleScrollStop();
        // 计算按钮位置
        if (domRef.current && buttonRef.current) {
          const bottom = 20 - scrollTop;
          setButtonStyle({ bottom: `${bottom}px` });
        }

        if (!isAtBottom && shouldAutoScroll) {
          setShouldAutoScroll(false);
        } else if (isAtBottom && !shouldAutoScroll) {
          setShouldAutoScroll(true);
        }
      };

      const scrollContainer = domRef.current;
      scrollContainer?.addEventListener('scroll', handleUserScroll);
      // 初始化时计算一次位置
      handleUserScroll();

      return () => {
        scrollContainer?.removeEventListener('scroll', handleUserScroll);
        setIsInScroll(false);
      };
    }, [shouldAutoScroll]);

    const handleScrollToBottom = useCallback(() => {
      if (domRef.current) {
        scrollToBottom(domRef);
        setShouldAutoScroll(true);
      }
    }, []);

    useEffect(() => {
      if (!streaming) {
        setShouldAutoScroll(true);
      }
    }, [streaming]);

    useImperativeHandle(ref, () => ({
      scrollToBottom: () => {
        handleScrollToBottom();
      },
    }));

    return (
      <div
        className={classNames(
          styles['execute-area-content'],
          'styled-scrollbar',
          className,
        )}
        ref={domRef}
      >
        {historicMessage?.map((item: DebugMessage, index: number) => (
          <MessageItem
            modelConfig={modelConfig}
            key={item.id}
            item={item || {}}
            lastItem={
              historicMessage[index - 1] || {
                message: messageList?.[messageList?.length - 1],
              }
            }
            updateEditable={v => updateEditableByIdx(v, index)}
            updateType={v => updateTypeByIdx(v, index)}
            updateMessageItem={v => updateMessageItemByIdx(v, index)}
            deleteChat={() => deleteChatByIdx(index)}
            smooth={false}
            rerunLLM={rerunLLM}
            canReRun={
              item.role === Role.Assistant &&
              index === historicMessage.length - 1
            }
            canFile={isMultiModal && item.role === Role.User}
            tools={mockTools}
            messageList={messageList}
            setMessageList={setMessageList as any}
            variables={mockVariables}
          />
        ))}
        {streaming && !isMultiGroup ? (
          <MessageItem
            streaming
            key="streaming"
            item={{
              role: Role.Assistant,
              content: streamingMessage || '',
              tool_calls: toolCalls,
              reasoning_content: reasoningContentResult,
            }}
            smooth
            stepDebuggingTrace={stepDebuggingTrace}
            setToolCalls={setToolCalls}
            tools={mockTools}
            rerunLLM={rerunLLM}
            stepSendMessage={stepSendMessage}
          />
        ) : null}
        {historicMessage?.length || streaming ? null : (
          <img src={peIcon} className={styles['execute-area-content-img']} />
        )}
        <div
          ref={buttonRef}
          className={classNames(styles['execute-area-content-to-bottom'], {
            [styles.visible]: showScrollButton && !isInScroll,
          })}
          onClick={handleScrollToBottom}
          style={buttonStyle}
        >
          <IconCozArrowDown />
        </div>
      </div>
    );
  },
);
