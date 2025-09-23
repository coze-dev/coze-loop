// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */

import { useState, type Dispatch, type SetStateAction } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { handleCopy } from '@cozeloop/components';
import {
  ContentType,
  type Prompt,
  type VariableVal,
  type DebugMessage as BasicDebugMessage,
  type Message,
} from '@cozeloop/api-schema/prompt';
import {
  IconCozAutoHeight,
  IconCozCopy,
  IconCozNode,
  IconCozPencil,
  IconCozRefresh,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  IconButton,
  Popconfirm,
  Space,
  Tooltip,
} from '@coze-arch/coze-design';

import { type PromptMessage } from '../editor-tools';

import styles from './index.module.less';

interface DebugMessage extends BasicDebugMessage {
  id?: string;
  isEdit?: boolean;
}

interface MessageToolBtnsProps<R> {
  item: DebugMessage;
  streaming?: boolean;
  canReRun?: boolean;
  canFile?: boolean;
  canOptimize?: boolean;
  saveDisabled?: boolean;
  isMarkdown?: boolean;
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
  updateMessage?: () => void;
  updateEditable?: (v: boolean) => void;
  deleteChat?: () => void;
  rerunLLM?: () => void;
  updateMessageItem?: (v: DebugMessage) => void;
  setIsMarkdown?: Dispatch<SetStateAction<boolean>>;
  setDebugId?: (v: string | number) => void;

  promptInfo?: Prompt;
  variables?: VariableVal[];
  optimizeSystemPrompt?: PromptMessage<R>;
  setMessageList?: SetStateAction<
    Array<Message & { key?: string }> | undefined
  >;
  lastItem?: DebugMessage;
}

export function MessageToolBtns<R>({
  item,
  streaming,
  updateEditable,
  deleteChat,
  rerunLLM,
  canReRun,
  updateMessageItem,
  saveDisabled,
  isMarkdown,
  setIsMarkdown,
  btnConfig,
  setDebugId,
}: MessageToolBtnsProps<R>) {
  const [showPopconfirm, setShowPopconfirm] = useState(false);

  const { isEdit, parts } = item;

  if (streaming) {
    return null;
  }

  const content =
    parts?.find(it => it?.type === ContentType.Text)?.text || item.content;

  const copyBtn = !btnConfig?.hideCopy && (
    <Tooltip content={I18n.t('copy')} theme="dark">
      <IconButton
        className={styles['icon-button']}
        icon={<IconCozCopy fontSize={14} />}
        disabled={!content}
        onClick={() => content && handleCopy(content)}
        size="mini"
        color="secondary"
      />
    </Tooltip>
  );

  const txtMdBtn = !btnConfig?.hideTypeChange && (
    <Tooltip content={isMarkdown ? 'TXT' : 'MARKDOWN'} theme="dark">
      <IconButton
        className={classNames(styles['icon-button'], '!hover:coz-mg-primary', {
          [styles['icon-button-active']]: !isMarkdown,
        })}
        icon={<IconCozAutoHeight fontSize={14} />}
        onClick={() => setIsMarkdown?.(v => !v)}
        size="mini"
        color="secondary"
      />
    </Tooltip>
  );

  const editBtn = !btnConfig?.hideEdit && (
    <Tooltip content={I18n.t('edit')} theme="dark">
      <IconButton
        className={styles['icon-button']}
        icon={<IconCozPencil fontSize={14} />}
        onClick={() => updateEditable?.(true)}
        size="mini"
        color="secondary"
      />
    </Tooltip>
  );

  const deleteBtn = !btnConfig?.hideDelete && (
    <Popconfirm
      trigger="custom"
      visible={showPopconfirm}
      title={I18n.t('delete_message')}
      content={I18n.t('confirm_delete_message')}
      cancelText={I18n.t('Cancel')}
      okText={I18n.t('delete')}
      okButtonProps={{ color: 'red' }}
      stopPropagation={true}
      onConfirm={() => {
        deleteChat?.();
        setShowPopconfirm(false);
      }}
      onCancel={() => setShowPopconfirm(false)}
    >
      {showPopconfirm ? (
        <IconButton
          className={styles['icon-button']}
          icon={<IconCozTrashCan fontSize={14} />}
          size="mini"
          onClick={() => setShowPopconfirm(false)}
        />
      ) : (
        <span>
          <Tooltip content={I18n.t('delete')} theme="dark">
            <IconButton
              className={styles['icon-button']}
              icon={<IconCozTrashCan fontSize={14} />}
              size="mini"
              onClick={() => setShowPopconfirm(true)}
              color="secondary"
            />
          </Tooltip>
        </span>
      )}
    </Popconfirm>
  );

  const cancelEditBtn = !btnConfig?.hideCancel && (
    <Button
      size="mini"
      color="primary"
      disabled={saveDisabled}
      className={styles['icon-button']}
      onClick={() => updateEditable?.(false)}
    >
      {I18n.t('Cancel')}
    </Button>
  );

  const okEditBtn = !btnConfig?.hideOk && (
    <Button
      size="mini"
      disabled={saveDisabled}
      icon
      onClick={() => updateMessageItem?.({ ...item, isEdit: false })}
    >
      {I18n.t('Confirm')}
    </Button>
  );

  const refreshBtn = !btnConfig?.hideRerun && (
    <Tooltip content={I18n.t('rerun')} theme="dark">
      <IconButton
        className={styles['icon-button']}
        icon={<IconCozRefresh fontSize={14} />}
        onClick={rerunLLM}
        size="mini"
        color="secondary"
      />
    </Tooltip>
  );

  const traceBtn = !btnConfig?.hideTrace && (
    <Tooltip content="Trace" theme="dark">
      <IconButton
        className={styles['icon-button']}
        icon={<IconCozNode fontSize={14} />}
        onClick={() => {
          setDebugId?.(item?.debug_id || '');
        }}
        size="mini"
        color="secondary"
      />
    </Tooltip>
  );

  if (isEdit) {
    return (
      <Space className="w-full justify-end" align="center">
        {cancelEditBtn}
        {okEditBtn}
      </Space>
    );
  }

  return (
    <div className={styles['tool-btns']}>
      {txtMdBtn}
      <Divider layout="vertical" />
      {item?.debug_id ? traceBtn : null}
      {editBtn}
      {copyBtn}
      {canReRun ? refreshBtn : null}
      {deleteBtn}
    </div>
  );
}
