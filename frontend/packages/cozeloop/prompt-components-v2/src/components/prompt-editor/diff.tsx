/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
import { useMemo, useRef, useState } from 'react';

import cn from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { handleCopy } from '@cozeloop/components';
import {
  type ContentPart,
  ContentType,
  type Message,
  Role,
} from '@cozeloop/api-schema/prompt';
import { IconCozCopy, IconCozTrashCan } from '@coze-arch/coze-design/icons';
import {
  IconButton,
  Popconfirm,
  Space,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import {
  getMultimodalVariableText,
  getPlaceholderErrorContent,
  splitMultimodalContent,
} from '@/utils/prompt';
import { DEFAULT_MESSAGE_TYPE_ARRAY } from '@/consts';

import {
  BasicPromptDiffEditor,
  type BasicPromptDiffEditorRef,
} from '../basic-prompt-editor/diff';
import LibraryBlockWidget from './widgets/skill';
import { SegmentEditorAction } from './widgets/sgement/segment-editor-action';
import SegmentCompletion from './widgets/sgement/segment-completion';
import ModalVariableCompletion from './widgets/modal-variable';
import { type PromptDiffEditorProps } from './type';
import { MultiModalBtn } from './tools/multi-modal-btn';
import { MessageTypeSelect } from './tools/message-type-select';

import styles from './index.module.less';

export function PromptDiffEditor({
  className,
  preMessage,
  message,
  cozeLibrarys,
  variables,
  disabled,
  modalVariableEnable,
  modalVariableBtnHidden,
  placeholderRoleValue,
  onDelete,
  hideActionWrap,
  messageTypeList = DEFAULT_MESSAGE_TYPE_ARRAY,
  leftActionBtns,
  snippetBtnHidden,
  rightActionBtns,
  onMessageChange,
  onMessageTypeChange,
}: PromptDiffEditorProps) {
  const editorRef = useRef<BasicPromptDiffEditorRef>(null);
  const [editorActive, setEditorActive] = useState(false);
  const placeholderError = getPlaceholderErrorContent(
    message as Message,
    variables,
  );
  const preDefaultValue = useMemo(
    () =>
      (preMessage?.parts?.length
        ? preMessage?.parts
            ?.map(it => {
              if (it.type === ContentType.MultiPartVariable && it?.text) {
                return getMultimodalVariableText(it.text);
              }
              return it.text;
            })
            .join('')
        : preMessage?.content) ?? '',
    [preMessage?.id, preMessage?.key],
  );

  const currentDefaultValue = useMemo(
    () =>
      (message?.parts?.length
        ? message?.parts
            ?.map(it => {
              if (it.type === ContentType.MultiPartVariable && it?.text) {
                return getMultimodalVariableText(it.text);
              }
              return it.text;
            })
            .join('')
        : message?.content) ?? '',
    [message?.id, message?.key],
  );

  const insterTextToEditor = (v?: string) => {
    v && editorRef?.current?.insertText?.(v);
  };

  const getCopyContent = () => {
    if (message?.parts?.length) {
      return message?.parts
        ?.map(it => {
          if (it.type === ContentType.MultiPartVariable && it?.text) {
            return `<multimodal-variable>${it.text}</multimodal-variable>`;
          }
          return it.text;
        })
        .join('');
    }
    return message?.content;
  };

  const handleMessageContentChange = (v: string) => {
    const parts = splitMultimodalContent(v) as ContentPart[];
    onMessageChange?.({ ...message, content: parts.length ? '' : v, parts });
  };

  const emptyRole = (
    <Typography.Text
      size="small"
      type="tertiary"
      className={cn('px-[10px]', styles['role-display'], 'variable-text')}
    >
      empty
    </Typography.Text>
  );

  return (
    <div
      className={cn(styles['prompt-editor-container'], className, {
        '!coz-fg-hglt-red !border': placeholderError,
        '!coz-stroke-hglt !border-l !border-r !border-b': editorActive,
        [styles['prompt-editor-container-disabled']]: disabled,
        'mb-5': message?.role === placeholderRoleValue && placeholderError,
      })}
    >
      {hideActionWrap ? null : (
        <div className="flex items-center w-full">
          <div className="flex items-center w-full h-full bg-[#fcfcff] pt-2">
            {preMessage?.role ? (
              <MessageTypeSelect
                value={preMessage?.role || Role.System}
                disabled
                messageTypeList={messageTypeList}
              />
            ) : (
              emptyRole
            )}
          </div>
          <div className="w-[8px] h-full bg-[#fcfcff] border-0 border-solid !border-r coz-stroke-primary flex-shrink-0"></div>
          <div className="flex justify-between items-center w-full h-full pt-2">
            {message?.role ? (
              <>
                <Space spacing={2} className="pl-0.5">
                  <MessageTypeSelect
                    value={message.role}
                    onChange={onMessageTypeChange}
                    disabled={disabled}
                    messageTypeList={messageTypeList}
                  />

                  {leftActionBtns}
                </Space>
                <Space spacing={8}>
                  {!snippetBtnHidden ? (
                    <SegmentEditorAction
                      disabled={disabled}
                      afterInsert={insterTextToEditor}
                    />
                  ) : null}
                  {rightActionBtns}
                  {!disabled &&
                  !modalVariableBtnHidden &&
                  message?.role !== placeholderRoleValue ? (
                    <MultiModalBtn
                      modalVariableEnable={modalVariableEnable}
                      variables={variables}
                      disabled={disabled}
                      afterInsert={insterTextToEditor}
                    />
                  ) : null}
                  <Tooltip content={I18n.t('copy')}>
                    <IconButton
                      icon={<IconCozCopy />}
                      color="secondary"
                      size="mini"
                      onClick={() => {
                        const info = getCopyContent() ?? '';
                        handleCopy(info);
                      }}
                    />
                  </Tooltip>
                  {!onDelete ? null : disabled ? (
                    <IconButton
                      icon={<IconCozTrashCan />}
                      color="secondary"
                      size="mini"
                      disabled={disabled}
                    />
                  ) : (
                    <Popconfirm
                      title={I18n.t('delete_prompt_template')}
                      content={I18n.t('confirm_delete_current_prompt_template')}
                      cancelText={I18n.t('cancel')}
                      okText={I18n.t('delete')}
                      okButtonProps={{ color: 'red' }}
                      onConfirm={() => onDelete?.(message)}
                    >
                      <IconButton
                        icon={<IconCozTrashCan />}
                        color="secondary"
                        size="mini"
                      />
                    </Popconfirm>
                  )}
                </Space>
              </>
            ) : (
              emptyRole
            )}
          </div>
        </div>
      )}
      <BasicPromptDiffEditor
        ref={editorRef}
        oldValue={preDefaultValue || (!preMessage?.role ? undefined : '')}
        newValue={currentDefaultValue || (!message?.role ? undefined : '')}
        editorAble={!disabled}
        onFocus={() => {
          setEditorActive(true);
        }}
        onBlur={() => setEditorActive(false)}
        onChange={handleMessageContentChange}
      >
        <ModalVariableCompletion
          isMultimodal={modalVariableEnable}
          variableKeys={variables?.map(it => it.key || '')}
          disabled={disabled}
          disabledTip={
            modalVariableBtnHidden
              ? I18n.t('prompt_current_message_not_support_multi_modal')
              : undefined
          }
        />

        <SegmentCompletion />
        {cozeLibrarys?.length ? (
          <LibraryBlockWidget librarys={cozeLibrarys} />
        ) : null}
      </BasicPromptDiffEditor>
    </div>
  );
}
