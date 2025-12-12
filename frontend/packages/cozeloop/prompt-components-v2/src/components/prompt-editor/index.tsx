/* eslint-disable max-lines-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */

/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { forwardRef, useEffect, useMemo, useRef, useState } from 'react';

import cn from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { handleCopy, TooltipWhenDisabled } from '@cozeloop/components';
import {
  type ContentPart,
  ContentType,
  type Message,
  Role,
  type VariableDef,
} from '@cozeloop/api-schema/prompt';
import { updateRegexpDecorations } from '@coze-editor/extension-regexp-decorator';
import {
  IconCozCopy,
  IconCozExpand,
  IconCozHandle,
  IconCozMinimize,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  IconButton,
  Input,
  Popconfirm,
  Space,
  Typography,
  Tooltip,
} from '@coze-arch/coze-design';

import {
  getMultimodalVariableText,
  getPlaceholderErrorContent,
  splitMultimodalContent,
} from '@/utils/prompt';
import { DEFAULT_MESSAGE_TYPE_ARRAY, VARIABLE_MAX_LEN } from '@/consts';

import LibraryBlockWidget from './widgets/skill';
import { SegmentEditorAction } from './widgets/sgement/segment-editor-action';
import SegmentCompletion from './widgets/sgement/segment-completion';
import ModalVariableCompletion from './widgets/modal-variable';
import {
  BasicPromptEditor,
  type BasicPromptEditorRef,
} from '../basic-prompt-editor';
import { type PromptEditorProps } from './type';
import { MultiModalBtn } from './tools/multi-modal-btn';
import { MessageTypeSelect } from './tools/message-type-select';

import styles from './index.module.less';

export const PromptEditor = forwardRef(
  (
    props: PromptEditorProps,
    ref?: React.ForwardedRef<BasicPromptEditorRef>,
  ) => {
    const editorRef = ref ?? useRef<BasicPromptEditorRef>(null);

    const {
      className,
      message,
      dragBtnHidden,
      messageTypeDisabled,
      variables,
      disabled,
      isInDrag,
      onMessageChange,
      onMessageTypeChange,
      placeholder,
      messageTypeList = DEFAULT_MESSAGE_TYPE_ARRAY,
      leftActionBtns,
      rightActionBtns,
      hideActionWrap,
      placeholderRoleValue = Role.Placeholder,
      modalVariableEnable,
      children,
      modalVariableBtnHidden,
      cozeLibrarys,
      onDelete,
      height,
      isFullscreen: propsIsFullscreen,
      onIsFullscreenChange,
      snippetBtnHidden,
      insertSnippetVariables,
      ...rest
    } = props;

    // 使用外部传入的 isFullscreen（如果有），否则使用内部状态
    const [internalIsFullscreen, setInternalIsFullscreen] = useState(false);
    // 最终的全屏状态：优先使用外部 props，其次使用内部状态
    const isFullscreen = propsIsFullscreen ?? internalIsFullscreen;
    const [editorActive, setEditorActive] = useState(false);
    const handleMessageContentChange = (v: string) => {
      const parts = splitMultimodalContent(v) as ContentPart[];
      onMessageChange?.({ ...message, content: parts.length ? '' : v, parts });
    };

    const readonly = disabled || isInDrag;
    const [promptTemplateInDelete, setPromptTemplateInDelete] = useState(false);

    const defaultValue = useMemo(
      () =>
        message?.parts?.length
          ? message?.parts
              ?.map(it => {
                if (it.type === ContentType.MultiPartVariable && it?.text) {
                  return getMultimodalVariableText(it.text);
                }
                return it.text;
              })
              .join('')
          : message?.content,
      [message?.id, message?.key],
    );

    const placeholderError = getPlaceholderErrorContent(
      message as Message,
      variables,
    );

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

    const insterTextToEditor = (v?: string, newVariables?: VariableDef[]) => {
      v && (editorRef as any)?.current?.insertText?.(v);
      if (newVariables?.length) {
        insertSnippetVariables?.(newVariables);
      }
    };

    useEffect(() => {
      const exitFullscreen = e => {
        if (e.key === 'Escape') {
          onIsFullscreenChange?.(false);
          setInternalIsFullscreen(false);
        }
      };
      document.addEventListener('keydown', exitFullscreen);
      return () => {
        document.removeEventListener('keydown', exitFullscreen);
      };
    }, []);

    useEffect(() => {
      const editor = (editorRef as any)?.current?.getEditor();
      if (editor?.$view) {
        updateRegexpDecorations(editor.$view);
      }
    }, [
      modalVariableEnable,
      modalVariableBtnHidden,
      JSON.stringify(variables),
    ]);

    useEffect(() => {
      if (message?.key) {
        if (!window.optimizeEditorMap) {
          window.optimizeEditorMap = {};
        }
        window.optimizeEditorMap[message.key] = (editorRef as any)?.current;
      }
    }, [message?.key]);

    return (
      <>
        <div
          className={cn(
            styles['prompt-editor-container'],
            {
              [styles['prompt-editor-container-error']]: placeholderError,
              [styles['prompt-editor-container-active']]: editorActive,
              [styles['prompt-editor-container-disabled']]: disabled,
              [styles['full-screen']]: isFullscreen,
              'mb-5':
                message?.role === placeholderRoleValue && placeholderError,
            },
            className,
          )}
          onClick={() => {
            (editorRef as any)?.current?.getEditor()?.focus();
          }}
        >
          {hideActionWrap ? null : (
            <div className={styles.header}>
              <Space spacing={2}>
                {dragBtnHidden || readonly ? null : (
                  <IconButton
                    color="secondary"
                    size="mini"
                    icon={<IconCozHandle fontSize={14} />}
                    className={cn('drag !w-[14px]', styles['drag-btn'])}
                  />
                )}
                {message?.role ? (
                  <MessageTypeSelect
                    value={message.role}
                    onChange={onMessageTypeChange}
                    disabled={messageTypeDisabled || readonly}
                    messageTypeList={messageTypeList}
                  />
                ) : null}
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
                {!readonly &&
                !modalVariableBtnHidden &&
                message?.role !== placeholderRoleValue ? (
                  <MultiModalBtn
                    modalVariableEnable={modalVariableEnable}
                    variables={variables}
                    disabled={disabled}
                    afterInsert={insterTextToEditor}
                  />
                ) : null}
                <Tooltip
                  content={
                    isFullscreen
                      ? I18n.t('prompt_exit_fullscreen')
                      : I18n.t('evaluate_full_screen')
                  }
                  theme="dark"
                >
                  <IconButton
                    icon={
                      isFullscreen ? (
                        <IconCozMinimize fontSize={12} />
                      ) : (
                        <IconCozExpand fontSize={12} />
                      )
                    }
                    color="secondary"
                    size="mini"
                    onClick={() => {
                      // 计算新的全屏状态
                      const newFullscreenState = !isFullscreen;
                      // 通知外部状态变化
                      onIsFullscreenChange?.(newFullscreenState);
                      // 如果没有外部控制，则更新内部状态
                      if (propsIsFullscreen === undefined) {
                        setInternalIsFullscreen(newFullscreenState);
                      }
                    }}
                    data-btm={isFullscreen ? 'd53025' : 'd27365'}
                    data-btm-title={
                      isFullscreen
                        ? I18n.t('prompt_exit_fullscreen')
                        : I18n.t('prompt_enter_fullscreen')
                    }
                  />
                </Tooltip>
                <Tooltip content={I18n.t('copy')} theme="dark">
                  <IconButton
                    icon={<IconCozCopy />}
                    color="secondary"
                    size="mini"
                    onClick={() => {
                      const info = getCopyContent() ?? '';
                      handleCopy(info);
                    }}
                    data-btm="d39312"
                    data-btm-title={I18n.t('prompt_copy_prompt')}
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
                    onVisibleChange={v => {
                      if (!v) {
                        setPromptTemplateInDelete(false);
                      }
                    }}
                  >
                    <span>
                      <TooltipWhenDisabled
                        content={I18n.t('delete_prompt_template')}
                        theme="dark"
                        disabled={!promptTemplateInDelete}
                      >
                        <IconButton
                          icon={<IconCozTrashCan />}
                          color="secondary"
                          size="mini"
                          onClick={() => setPromptTemplateInDelete(true)}
                          data-btm="d27362"
                          data-btm-title={I18n.t('prompt_delete_prompt')}
                        />
                      </TooltipWhenDisabled>
                    </span>
                  </Popconfirm>
                )}
              </Space>
            </div>
          )}
          <div
            className={cn('w-full overflow-y-auto styled-scrollbar', {
              'py-1': message?.role !== placeholderRoleValue,
            })}
          >
            {message?.role === placeholderRoleValue ? (
              <Input
                key={message.key || message.id}
                value={message.content}
                onChange={handleMessageContentChange}
                borderless
                disabled={readonly}
                style={{ border: 0, borderRadius: 0 }}
                maxLength={VARIABLE_MAX_LEN}
                max={50}
                className="!pl-3 font-sm"
                inputStyle={{
                  fontSize: 13,
                  color: 'var(--Green-COZColorGreen7, #00A136)',
                  fontFamily: 'JetBrainsMonoRegular',
                }}
                onFocus={() => setEditorActive(true)}
                onBlur={() => setEditorActive(false)}
                placeholder={I18n.t('prompt_var_format')}
              />
            ) : (
              <BasicPromptEditor
                key={message?.key || message?.id}
                {...rest}
                defaultValue={defaultValue}
                onChange={handleMessageContentChange}
                variables={variables}
                readOnly={readonly}
                linePlaceholder={placeholder}
                onFocus={() => setEditorActive(true)}
                onBlur={() => setEditorActive(false)}
                ref={editorRef}
                height={isFullscreen ? undefined : height}
              >
                <ModalVariableCompletion
                  isMultimodal={modalVariableEnable}
                  variableKeys={variables?.map(it => it.key || '')}
                  disabled={modalVariableBtnHidden || disabled}
                />

                <SegmentCompletion />
                {cozeLibrarys?.length ? (
                  <LibraryBlockWidget librarys={cozeLibrarys} />
                ) : null}
                {children}
              </BasicPromptEditor>
            )}
          </div>
          <Typography.Text
            size="small"
            type="danger"
            className="absolute bottom-[-20px] left-0"
          >
            {placeholderError}
          </Typography.Text>
        </div>
      </>
    );
  },
);
