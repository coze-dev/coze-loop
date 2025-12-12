/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */

/* eslint-disable complexity */

import { useMemo, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { formateMsToSeconds } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { MultiPartRender } from '@cozeloop/components';
import {
  type ContentPart,
  ContentType,
  type DebugToolCall,
  type MockTool,
  Role,
} from '@cozeloop/api-schema/prompt';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import {
  Avatar,
  Button,
  Loading,
  Space,
  Tag,
  TextArea,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';
import { CalypsoLazy } from '@bytedance/calypso';

import {
  usePromptMockDataStore,
  type DebugMessage,
} from '@/store/use-mockdata-store';
import IconLogo from '@/assets/mini-logo.svg';

import { usePromptDevProviderContext } from '../../prompt-provider';
import { ToolBtns } from './tool-btns';
import { MultiModalItem } from './multi-modal-item';
import { FunctionList } from './function-list';

import styles from './index.module.less';

export interface MessageItemProps {
  item: DebugMessage;
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
  };
  updateType?: (type: Role) => void;
  updateMessage?: (msg?: string) => void;
  updateEditable?: (v: boolean) => void;
  updateMessageItem?: (v: DebugMessage) => void;
  deleteChat?: () => void;
  rerunLLM?: () => void;
  setToolCalls?: React.Dispatch<React.SetStateAction<DebugToolCall[]>>;
  streaming?: boolean;
  tools?: MockTool[];
  stepSendMessage?: () => void;
  renderExtraActions?: (item?: DebugMessage) => React.ReactNode;
}

export function MessageItem({
  item,
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
  btnConfig = {},
  renderExtraActions,
}: MessageItemProps) {
  const { userInfo, debugAreaConfig } = usePromptDevProviderContext();
  const [reasoningExpand, setReasoningExpand] = useState(true);

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
  const content =
    parts?.find(it => it?.type === ContentType.Text)?.text ?? oldContent;

  const fileParts = parts?.filter(it => it.type !== ContentType.Text);

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
          src={debugAreaConfig?.assistantLogoUrl || IconLogo}
          size="default"
        ></Avatar>
      );
    }
    if (role === Role.System) {
      return (
        <Avatar
          className={styles['message-avatar']}
          size="default"
          src={debugAreaConfig?.systemLogoUrl}
        >
          {debugAreaConfig?.systemLogoUrl ? null : 'S'}
        </Avatar>
      );
    }
  }, [role, userInfo?.avatar_url]);

  const isInAiPending = isAI && streaming && !tool_calls?.length && !content;
  const aiResIsMultiModal =
    isAI && parts?.some(it => it.type !== ContentType.Text);

  return (
    <div key={item.id} className={styles['message-item']}>
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
              !tool_calls?.length &&
              !aiResIsMultiModal,
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
                {content ? I18n.t('deeply_thought') : I18n.t('deep_thinking')}
              </Tag>
              {reasoningExpand ? (
                <CalypsoLazy
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
            {isInAiPending ? (
              <Loading loading color="blue" size="mini" />
            ) : aiResIsMultiModal ? (
              <MultiModalItem
                parts={parts}
                isMarkdown={isMarkdown}
                streaming={streaming}
                smooth={smooth}
              />
            ) : isEdit ? (
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
              debugAreaConfig?.renderMessageItem?.({
                message: { ...item, content },
                streaming,
              }) || (
                <CalypsoLazy
                  markDown={content}
                  imageOptions={{ forceHttps: true }}
                  smooth={smooth}
                  autoFixSyntax={{ autoFixEnding: smooth }}
                />
              )
            )}
          </div>
          <div className={classNames(styles['message-footer-tools'])}>
            {(cost_ms || output_tokens || input_tokens) && !isEdit ? (
              <Typography.Text
                size="small"
                type="tertiary"
                className="flex-1 flex-shrink-0"
              >
                {I18n.t('prompt_time_and_tokens_info', {
                  placeholder1: formateMsToSeconds(cost_ms),
                })}
                <Tooltip
                  theme="dark"
                  content={
                    <Space vertical align="start">
                      <Typography.Text style={{ color: '#fff' }}>
                        {I18n.t('input')} Tokens: {input_tokens}
                      </Typography.Text>
                      <Typography.Text style={{ color: '#fff' }}>
                        {I18n.t('output')} Tokens: {output_tokens}
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
                {`| ${I18n.t('num_words', { num: content.length })}`}
              </Typography.Text>
            ) : null}

            {!streaming ? (
              <ToolBtns
                item={item}
                isMarkdown={isMarkdown}
                btnConfig={{
                  ...btnConfig,
                  hideEdit: aiResIsMultiModal,
                  hideCopy: aiResIsMultiModal,
                }}
                setIsMarkdown={v => setIsMarkdown(v)}
                deleteChat={deleteChat}
                updateEditable={updateEditable}
                updateMessageItem={v => {
                  if (fileParts.length) {
                    const hasText = parts.some(
                      it => it.type === ContentType.Text,
                    );
                    let newParts: ContentPart[] = [];
                    if (hasText) {
                      newParts = parts.map(it => {
                        if (it.type === ContentType.ImageURL) {
                          return it;
                        }
                        return { ...it, ...v, text: editMsg };
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
                      ...v,
                      parts: newParts,
                      content: '',
                    });
                  } else {
                    updateMessageItem?.({
                      ...item,
                      ...v,
                      content: editMsg,
                      parts: undefined,
                    });
                  }
                }}
                rerunLLM={rerunLLM}
                canReRun={canReRun}
                renderExtraActions={cItem => renderExtraActions?.(cItem)}
              />
            ) : null}
            {stepDebuggingTrace && stepDebugger && !isCompare ? (
              <div className="w-full text-right">
                <Button color="brand" size="mini" onClick={stepSendMessage}>
                  {I18n.t('global_btn_confirm')}
                </Button>
              </div>
            ) : null}
          </div>
        </div>
        {fileParts.length && !aiResIsMultiModal ? (
          <MultiPartRender
            fileParts={fileParts}
            readonly
            className="!p-0 !border-0"
          />
        ) : null}
      </div>
    </div>
  );
}
