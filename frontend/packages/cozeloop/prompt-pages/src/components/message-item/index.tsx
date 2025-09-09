// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */

/* eslint-disable complexity */

import { type SetStateAction, useMemo, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { formateMsToSeconds } from '@cozeloop/toolkit';
import { useUserInfo } from '@cozeloop/biz-hooks-adapter';
import { MessageToolBtns } from '@cozeloop/biz-components-adapter';
import {
  type ContentPart,
  ContentType,
  type DebugToolCall,
  type Message,
  type MockTool,
  type ModelConfig,
  Role,
  type VariableVal,
} from '@cozeloop/api-schema/prompt';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import {
  Avatar,
  Button,
  ImagePreview,
  Space,
  Tag,
  TextArea,
  Tooltip,
  Typography,
  Image,
} from '@coze-arch/coze-design';
import { MdBoxLazy } from '@coze-arch/bot-md-box-adapter/lazy';

import { usePromptStore } from '@/store/use-prompt-store';
import {
  usePromptMockDataStore,
  type DebugMessage,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import IconLogo from '@/assets/mini-logo.svg';

import { FunctionList } from './function-list';

import styles from './index.module.less';

interface MessageItemProps {
  item: DebugMessage;
  lastItem?: DebugMessage;
  smooth?: boolean;
  canReRun?: boolean;
  canFile?: boolean;
  stepDebuggingTrace?: string;
  btnConfig?: {
    hideMessageTypeSelect?: boolean;
    hideDelete?: boolean;
    hideEdit?: boolean;
    hideRerun?: boolean;
    hideCopy?: boolean;
    hideTypeChange?: boolean;
    hideCancel?: boolean;
    hideOk?: boolean;
    hideTrace?: boolean;
    hideOptimize?: boolean;
  };
  modelConfig?: ModelConfig;
  updateType?: (type: Role) => void;
  updateMessage?: (msg?: string) => void;
  updateEditable?: (v: boolean) => void;
  updateMessageItem?: (v: DebugMessage) => void;
  deleteChat?: () => void;
  rerunLLM?: () => void;
  setToolCalls?: React.Dispatch<React.SetStateAction<DebugToolCall[]>>;
  streaming?: boolean;
  tools?: MockTool[];
  variables?: VariableVal[];
  stepSendMessage?: () => void;
  messageList?: Array<Message & { key?: string }>;
  setMessageList?: SetStateAction<
    Array<Message & { key?: string }> | undefined
  >;
}

export function MessageItem({
  item,
  lastItem,
  smooth,
  updateMessageItem,
  streaming,
  setToolCalls,
  stepDebuggingTrace,
  tools,
  deleteChat,
  updateEditable,
  rerunLLM,
  canReRun,
  stepSendMessage,
  messageList,
  setMessageList,
  variables,
  btnConfig,
}: MessageItemProps) {
  const [reasoningExpand, setReasoningExpand] = useState(true);
  const userInfo = useUserInfo();

  const { setDebugId } = useBasicStore(
    useShallow(state => ({ setDebugId: state.setDebugId })),
  );

  const { promptInfo } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
    })),
  );

  const { compareConfig, userDebugConfig } = usePromptMockDataStore(
    useShallow(state => ({
      compareConfig: state.compareConfig,
      userDebugConfig: state.userDebugConfig,
    })),
  );
  const stepDebugger = userDebugConfig?.single_step_debug;

  const isCompare = Boolean(compareConfig?.groups?.length);

  const {
    cost_ms,
    isEdit,
    output_tokens,
    input_tokens,
    reasoning_content,
    role = Role.System,
    content: oldContent = '',
    parts = [],
    tool_calls,
  } = item;

  const isAI = role === Role.Assistant;
  const content = parts?.length
    ? parts.find(it => it?.type === ContentType.Text)?.text || ''
    : oldContent;

  const imgParts = parts?.filter(it => it.type === ContentType.ImageURL);

  const [editMsg, setEditMsg] = useState<string>(content);
  const [isMarkdown, setIsMarkdown] = useState(
    Boolean(localStorage.getItem('fornax_prompt_markdown') !== 'false') ||
      !isAI,
  );

  const avatarDom = useMemo(() => {
    if (role === Role.User) {
      return userInfo?.avatar_url ? (
        <Avatar
          className={styles['message-avatar']}
          size="default"
          src={userInfo?.avatar_url}
        />
      ) : (
        <Avatar
          className={styles['message-avatar']}
          size="default"
          color="blue"
        >
          U
        </Avatar>
      );
    }
    if (role === Role.Assistant) {
      return (
        <Avatar
          className={styles['message-avatar']}
          src={IconLogo}
          size="default"
        ></Avatar>
      );
    }
    if (role === Role.System) {
      return (
        <Avatar className={styles['message-avatar']} size="default">
          S
        </Avatar>
      );
    }
  }, [role, userInfo?.avatar_url]);

  const optimizeSystemPrompt = messageList?.find(it => it.role === Role.System);

  return (
    <div className={styles['message-item']}>
      {avatarDom}

      <div
        className={classNames('flex flex-col gap-2 overflow-hidden', {
          'flex-1': isEdit,
        })}
      >
        <div
          className={classNames(styles['message-content'], styles[role], {
            [styles['message-edit']]: isEdit,
            [styles['message-item-error']]:
              !streaming &&
              !isEdit &&
              item.debug_id &&
              !content &&
              !reasoning_content &&
              !tool_calls?.length,
          })}
        >
          {reasoning_content ? (
            <Space vertical align="start">
              <Tag
                className="cursor-pointer"
                color="primary"
                onClick={() => setReasoningExpand(v => !v)}
                style={{ maxWidth: 'fit-content' }}
                suffixIcon={
                  <IconCozArrowDown
                    className={classNames(styles['function-chevron-icon'], {
                      [styles['function-chevron-icon-close']]: !reasoningExpand,
                    })}
                    fontSize={12}
                  />
                }
              >
                {content ? '已深度思考' : '深度思考中'}
              </Tag>
              {reasoningExpand ? (
                <MdBoxLazy
                  markDown={reasoning_content}
                  style={{
                    color: '#8b8b8b',
                    borderLeft: '2px solid #e5e5e5',
                    paddingLeft: 6,
                    fontSize: 12,
                  }}
                />
              ) : null}
            </Space>
          ) : null}
          {tool_calls?.length ? (
            <FunctionList
              toolCalls={tool_calls}
              stepDebuggingTrace={stepDebuggingTrace}
              setToolCalls={setToolCalls}
              tools={tools}
              streaming={streaming}
            />
          ) : null}
          <div
            className={classNames(styles['message-info'], {
              '!p-0': isEdit,
              hidden: !content && tool_calls?.length && streaming,
            })}
          >
            {isEdit ? (
              <TextArea
                rows={1}
                autosize
                autoFocus
                defaultValue={content}
                onChange={setEditMsg}
                className="min-w-[300px] !bg-white"
              />
            ) : !isMarkdown ? (
              <Typography.Paragraph
                className="whitespace-break-spaces"
                style={{ lineHeight: '21px' }}
              >
                {content || ''}
              </Typography.Paragraph>
            ) : (
              <MdBoxLazy
                markDown={
                  content ||
                  (isAI && streaming && !tool_calls?.length ? '...' : '')
                }
                imageOptions={{ forceHttps: true }}
                smooth={smooth}
                autoFixSyntax={{ autoFixEnding: smooth }}
              />
            )}
          </div>
          <div className={classNames(styles['message-footer-tools'])}>
            {(cost_ms || output_tokens || input_tokens) && !isEdit ? (
              <Typography.Text
                size="small"
                type="tertiary"
                className="flex-1 flex-shrink-0"
              >
                耗时: {formateMsToSeconds(cost_ms)} | Tokens:
                <Tooltip
                  theme="dark"
                  content={
                    <Space vertical align="start">
                      <Typography.Text style={{ color: '#fff' }}>
                        输入 Tokens: {input_tokens}
                      </Typography.Text>
                      <Typography.Text style={{ color: '#fff' }}>
                        输出 Tokens: {output_tokens}
                      </Typography.Text>
                    </Space>
                  }
                >
                  <span className="mx-1">
                    {`${
                      output_tokens || input_tokens
                        ? Number(output_tokens || 0) + Number(input_tokens || 0)
                        : '-'
                    } Tokens`}
                  </span>
                </Tooltip>
                {`| 字数: ${content.length}`}
              </Typography.Text>
            ) : null}

            {!streaming ? (
              <MessageToolBtns
                item={item}
                lastItem={lastItem}
                isMarkdown={isMarkdown}
                btnConfig={{ hideOptimize: !isAI, ...btnConfig }}
                setIsMarkdown={v => setIsMarkdown(v)}
                deleteChat={deleteChat}
                updateEditable={updateEditable}
                updateMessageItem={() => {
                  if (imgParts.length) {
                    const hasText = parts.some(
                      it => it.type === ContentType.Text,
                    );
                    let newParts: ContentPart[] = [];
                    if (hasText) {
                      newParts = parts.map(it => {
                        if (it.type === ContentType.ImageURL) {
                          return it;
                        }
                        return { ...it, text: editMsg };
                      });
                    } else {
                      newParts = [
                        ...parts,
                        {
                          text: editMsg,
                          type: ContentType.Text,
                        },
                      ];
                    }

                    updateMessageItem?.({
                      ...item,
                      role: item.role,
                      parts: newParts,
                      content: '',
                    });
                  } else {
                    updateMessageItem?.({
                      ...item,
                      role: item.role,
                      content: editMsg,
                      parts: undefined,
                    });
                  }
                }}
                rerunLLM={rerunLLM}
                canReRun={canReRun}
                canOptimize={isAI}
                promptInfo={promptInfo}
                variables={variables}
                optimizeSystemPrompt={optimizeSystemPrompt}
                setMessageList={setMessageList}
                setDebugId={setDebugId}
              />
            ) : null}
            {stepDebuggingTrace && stepDebugger && !isCompare ? (
              <div className="w-full text-right">
                <Button color="brand" size="mini" onClick={stepSendMessage}>
                  确认
                </Button>
              </div>
            ) : null}
          </div>
        </div>
        {imgParts.length ? (
          <ImagePreview closable className="flex gap-2 flex-wrap">
            {imgParts?.map(it => (
              <Image
                width={45}
                height={45}
                src={it.image_url?.url}
                imgStyle={{ objectFit: 'contain' }}
                key={it.image_url?.url}
              />
            ))}
          </ImagePreview>
        ) : null}
      </div>
    </div>
  );
}
