// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */

import { useState, type Dispatch, type SetStateAction } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { handleCopy } from '@cozeloop/components';
import { ContentType, Role } from '@cozeloop/api-schema/prompt';
import {
  IconCozAutoHeight,
  IconCozCopy,
  IconCozNode,
  IconCozNodeExpand,
  IconCozPencil,
  IconCozRefresh,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  IconButton,
  Menu,
  Popconfirm,
  Space,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import { getButtonHiddenFromConfig } from '@/utils/base';
import { type DebugMessage } from '@/store/use-mockdata-store';

import { usePromptDevProviderContext } from '../../prompt-provider';

import styles from './index.module.less';

interface ToolBtnsProps {
  item: DebugMessage;
  streaming?: boolean;
  canReRun?: boolean;
  canFile?: boolean;
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
  };
  updateType?: (type: Role) => void;
  updateMessage?: () => void;
  updateEditable?: (v: boolean) => void;
  deleteChat?: () => void;
  rerunLLM?: () => void;
  updateMessageItem?: (v: DebugMessage) => void;
  setIsMarkdown?: Dispatch<SetStateAction<boolean>>;
  renderExtraActions?: (item?: DebugMessage) => React.ReactNode;
}

const RoleOptions = [
  { label: 'System', value: Role.System },
  { label: 'User', value: Role.User },
  { label: 'Assistant', value: Role.Assistant },
];

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
  renderExtraActions,
}: ToolBtnsProps) {
  const { buttonConfig } = usePromptDevProviderContext();
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
        data-btm="d42075"
        data-btm-title={I18n.t('copy')}
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
        data-btm="d27609"
        data-btm-title={I18n.t('prompt_text_md_toggle')}
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
        data-btm="d70959"
        data-btm-title={I18n.t('edit')}
      />
    </Tooltip>
  );

  const deleteBtn = !btnConfig?.hideDelete && (
    <Popconfirm
      trigger="custom"
      visible={showPopconfirm}
      title={I18n.t('delete_message')}
      content={I18n.t('confirm_delete_message')}
      cancelText={I18n.t('cancel')}
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
          data-btm="d27604"
          data-btm-title={I18n.t('delete')}
        />
      ) : (
        <span>
          <Tooltip content={I18n.t('delete')} theme="dark">
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
      {I18n.t('cancel')}
    </Button>
  );

  const okEditBtn = !btnConfig?.hideOk && (
    <Button
      size="mini"
      disabled={saveDisabled}
      icon
      onClick={() => updateMessageItem?.({ ...item, isEdit: false })}
    >
      {I18n.t('global_btn_confirm')}
    </Button>
  );

  const refreshBtn = !btnConfig?.hideRerun && (
    <Tooltip content={I18n.t('rerun')} theme="dark">
      <IconButton
        className={styles['icon-button']}
        icon={<IconCozRefresh fontSize={14} />}
        onClick={rerunLLM}
        size="mini"
        data-btm="d92738"
        data-btm-title={I18n.t('rerun')}
      />
    </Tooltip>
  );

  const traceBtn = !btnConfig?.hideTrace &&
    !getButtonHiddenFromConfig(buttonConfig?.traceLogButton) && (
      <Tooltip content="Trace" theme="dark">
        <IconButton
          className={styles['icon-button']}
          icon={<IconCozNode fontSize={14} />}
          onClick={() => {
            if (buttonConfig?.traceLogButton?.onClick) {
              buttonConfig?.traceLogButton?.onClick?.({
                debugId: item?.debug_id,
              });
            }
          }}
          size="mini"
          data-btm="d27574"
          data-btm-title="Trace"
        />
      </Tooltip>
    );

  const messageTypeButton = !btnConfig?.hideMessageTypeSelect && (
    <Menu
      trigger="click"
      position="bottomLeft"
      showTick={false}
      render={
        <Menu.SubMenu
          mode="selection"
          selectedKeys={[`${item.role}`]}
          onSelectionChange={v => {
            updateMessageItem?.({ ...item, role: v as Role });
          }}
        >
          {RoleOptions?.map(it => (
            <Menu.Item
              itemKey={`${it.value}`}
              key={it.value}
              className={classNames('!px-2', {
                'coz-mg-primary': `${it.value}` === `${item.role}`,
              })}
            >
              <Typography.Text className="variable-text">
                {it.label}
              </Typography.Text>
            </Menu.Item>
          ))}
        </Menu.SubMenu>
      }
      clickToHide
    >
      <Button size="mini" color="secondary">
        {(RoleOptions.find(it => it.value === item.role)?.label || item.role) ??
          '-'}
        <IconCozNodeExpand className="ml-0.5" />
      </Button>
    </Menu>
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
      {messageTypeButton}
      {txtMdBtn}
      <Divider layout="vertical" />
      {renderExtraActions?.(item)}
      {item?.debug_id ? traceBtn : null}
      {editBtn}
      {copyBtn}
      {canReRun ? refreshBtn : null}
      {deleteBtn}
    </div>
  );
}
