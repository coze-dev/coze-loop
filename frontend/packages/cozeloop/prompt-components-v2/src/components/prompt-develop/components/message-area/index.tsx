// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
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

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { SLEEP_TIME } from '@cozeloop/toolkit';
import {
  ContentType,
  type DebugToolCall,
  type ModelConfig,
  Role,
} from '@cozeloop/api-schema/prompt';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';

import { usePromptStore } from '@/store/use-prompt-store';
import { type DebugMessage } from '@/store/use-mockdata-store';
import { useCompare } from '@/hooks/use-compare';
import peIcon from '@/assets/loop.svg';

import { usePromptDevProviderContext } from '../prompt-provider';
import { useStreamingScrollManager } from './streaming-scroll-manager';
import { VirtualMessageItem } from './message-item/virtual-message-item';
import { MessageItem } from './message-item';

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
    const isInitializedRef = useRef(false);
    const { debugAreaConfig } = usePromptDevProviderContext();
    const {
      messageList,
      currentModel,
      mockTools,
      historicMessage = [],
      setHistoricMessage,
      mockVariables,
    } = useCompare(uid);

    const { promptInfo } = usePromptStore(
      useShallow(state => ({ promptInfo: state.promptInfo })),
    );

    const originalSystemPrompt = messageList?.find(
      it => it.role === Role.System,
    );

    const isMultiModal = currentModel?.ability?.multi_modal;
    const domRef = useRef<HTMLDivElement>(null);
    const [showScrollButton, setShowScrollButton] = useState(false);
    const [buttonStyle, setButtonStyle] = useState({});
    const [isInScroll, setIsInScroll] = useState(false);

    const [isTransitioning, setIsTransitioning] = useState(false);
    const historicChatItemVisibleMap = useRef<Record<string, boolean>>({});

    const historyChatListStr = JSON.stringify(historicMessage);
    const lastHistoricChatLen = useRef(historicMessage.length);

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

    // 使用智能滚动管理器
    const { handleScrollToBottom } = useStreamingScrollManager({
      containerRef: domRef,
      isStreaming: Boolean(streaming),
      streamingText: `${streamingMessage}${reasoningContentResult}`,
      onScrollStateChange: state => {
        setShowScrollButton(!state.isAtBottom);
        setIsInScroll(state.isScrolling || false);
        if (!state.isScrolling && !state.isAtBottom) {
          const SCROLL_BUTTON_OFFSET = 20;
          const bottom = SCROLL_BUTTON_OFFSET - state.scrollPosition;
          setButtonStyle({ bottom: `${bottom}px` });
        }
      },
    });

    useEffect(() => {
      setIsTransitioning(true);
      if (historicMessage.length < lastHistoricChatLen.current) {
        setShowScrollButton(false);
        setIsInScroll(false);
        setButtonStyle({ bottom: '0px' });

        const lastHistoricChat = historicMessage[historicMessage.length - 1];
        if (
          streaming &&
          lastHistoricChat &&
          historicChatItemVisibleMap.current[lastHistoricChat.id || '']
        ) {
          handleScrollToBottom();
        }
        lastHistoricChatLen.current = historicMessage.length;
        setTimeout(() => {
          setIsTransitioning(false);
        }, SLEEP_TIME);
      } else {
        setIsTransitioning(false);
        lastHistoricChatLen.current = historicMessage.length;
      }
    }, [historicMessage.length, streaming]);

    useEffect(() => {
      if (toolCalls?.length && !isTransitioning) {
        handleScrollToBottom();
      }
    }, [isTransitioning, toolCalls?.length]);

    useEffect(() => {
      if (!isInitializedRef.current) {
        isInitializedRef.current = true;
        setTimeout(() => {
          handleScrollToBottom();
        }, 100);
      }
    }, []);

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
        style={{
          backgroundImage:
            !historicMessage.length && !streaming
              ? `url(${debugAreaConfig?.emptyLogoUrl || peIcon})`
              : '',
        }}
      >
        {historicMessage?.map((item: DebugMessage, index: number) => (
          <VirtualMessageItem
            key={item.id}
            item={item || {}}
            estimatedHeight={160}
            rootMargin="150px"
            threshold={[0, 0.01, 0.1, 0.5, 1]}
            forceRender={index === historicMessage.length - 1 || item.isEdit}
            onVisibilityChange={v => {
              if (item?.id) {
                historicChatItemVisibleMap.current[item.id] = v;
              }
            }}
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
            btnConfig={{
              hideMessageTypeSelect: debugAreaConfig?.hideRoleChange,
            }}
            renderExtraActions={() =>
              debugAreaConfig?.renderExtraToolActions?.({
                message: item,
                lastMessage: historicMessage[index - 1],
                originalSystemPrompt,
                variables: mockVariables,
                prompt: promptInfo,
              })
            }
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

        {/* 滚动到底部按钮 */}
        <div
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
