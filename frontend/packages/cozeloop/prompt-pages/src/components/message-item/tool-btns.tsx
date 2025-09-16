// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */

import { useState, type Dispatch, type SetStateAction } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { handleCopy } from '@cozeloop/components';
import { ContentType, type Role } from '@cozeloop/api-schema/prompt';
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

import { type DebugMessage } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';

import styles from './index.module.less';

interface ToolBtnsProps {
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
  updateType?: (type: Role) => void;
  updateMessage?: () => void;
  updateEditable?: (v: boolean) => void;
  deleteChat?: () => void;
  rerunLLM?: () => void;
  updateMessageItem?: (v: DebugMessage) => void;
  onOptimizeClick?: (debug_id?: Int64) => void;
  setIsMarkdown?: Dispatch<SetStateAction<boolean>>;
}

export function ToolBtns({
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
  onOptimizeClick,
  canOptimize,
}: ToolBtnsProps) {
  const { setDebugId } = useBasicStore(
    useShallow(state => ({ setDebugId: state.setDebugId })),
  );
  const [showPopconfirm, setShowPopconfirm] = useState(false);

  const { isEdit, parts } = item;

  if (streaming) {
    return null;
  }

  const content =
    parts?.find(it => it?.type === ContentType.Text)?.text || item.content;

  const copyBtn = !btnConfig?.hideCopy && (
    <Tooltip content="复制" theme="dark">
      <IconButton
        className={styles['icon-button']}
        icon={<IconCozCopy fontSize={14} />}
        disabled={!content}
        onClick={() => content && handleCopy(content)}
        size="mini"
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
      />
    </Tooltip>
  );

  const editBtn = !btnConfig?.hideEdit && (
    <Tooltip content="编辑" theme="dark">
      <IconButton
        className={styles['icon-button']}
        icon={<IconCozPencil fontSize={14} />}
        onClick={() => updateEditable?.(true)}
        size="mini"
      />
    </Tooltip>
  );

  const deleteBtn = !btnConfig?.hideDelete && (
    <Popconfirm
      trigger="custom"
      visible={showPopconfirm}
      title="删除消息"
      content="确认删除该消息吗？"
      cancelText="取消"
      okText="删除"
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
          <Tooltip content="删除" theme="dark">
            <IconButton
              className={styles['icon-button']}
              icon={<IconCozTrashCan fontSize={14} />}
              size="mini"
              onClick={() => setShowPopconfirm(true)}
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
      取消
    </Button>
  );

  const okEditBtn = !btnConfig?.hideOk && (
    <Button
      size="mini"
      disabled={saveDisabled}
      icon
      onClick={() => updateMessageItem?.({ ...item, isEdit: false })}
    >
      确认
    </Button>
  );

  const refreshBtn = !btnConfig?.hideRerun && (
    <Tooltip content="重新运行" theme="dark">
      <IconButton
        className={styles['icon-button']}
        icon={<IconCozRefresh fontSize={14} />}
        onClick={rerunLLM}
        size="mini"
      />
    </Tooltip>
  );

  const traceBtn = !btnConfig?.hideTrace && (
    <Tooltip content="Trace" theme="dark">
      <IconButton
        className={styles['icon-button']}
        icon={<IconCozNode fontSize={14} />}
        onClick={() => {
          setDebugId(item?.debug_id);
        }}
        size="mini"
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
