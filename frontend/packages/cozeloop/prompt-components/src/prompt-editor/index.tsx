// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable security/detect-non-literal-regexp */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { forwardRef, type ReactNode, useEffect, useRef, useState } from 'react';

import cn from 'classnames';
import { TooltipWhenDisabled } from '@cozeloop/components';
import {
  type ContentPart,
  ContentType,
  type Message,
  Role,
} from '@cozeloop/api-schema/prompt';
import { updateRegexpDecorations } from '@coze-editor/extension-regexp-decorator';
import { IconCozHandle } from '@coze-arch/coze-design/icons';
import {
  Icon,
  IconButton,
  Input,
  Popconfirm,
  Space,
  Form,
  FormTextArea,
  type FormApi,
  Typography,
} from '@coze-arch/coze-design';

import {
  getMultimodalVariableText,
  getPlaceholderErrorContent,
  splitMultimodalContent,
} from '@/utils/prompt';
import { VARIABLE_MAX_LEN } from '@/consts';
import { ReactComponent as ModalVariableIcon } from '@/assets/modal-variable.svg';

import ModalVariableCompletion from './widgets/modal-variable';
import {
  PromptBasicEditor,
  type PromptBasicEditorRef,
  type PromptBasicEditorProps,
} from '../basic-editor';
import { MessageTypeSelect } from './message-type-select';

import styles from './index.module.less';

export type PromptMessage<R extends string | number> = Omit<Message, 'role'> & {
  role?: R;
  id?: string;
  key?: string;
  optimize_key?: string;
};

type BasicEditorProps = Pick<
  PromptBasicEditorProps,
  | 'variables'
  | 'height'
  | 'minHeight'
  | 'maxHeight'
  | 'forbidJinjaHighlight'
  | 'forbidVariables'
  | 'linePlaceholder'
  | 'isGoTemplate'
  | 'customExtensions'
  | 'canSearch'
  | 'isJinja2Template'
>;

export interface PromptEditorProps<R extends string | number>
  extends BasicEditorProps {
  className?: string;
  message?: PromptMessage<R>;
  dragBtnHidden?: boolean;
  messageTypeDisabled?: boolean;
  disabled?: boolean;
  isDrag?: boolean;
  placeholder?: string;
  messageTypeList?: Array<{ label: string; value: R }>;
  leftActionBtns?: ReactNode;
  rightActionBtns?: ReactNode;
  hideActionWrap?: boolean;
  isFullscreen?: boolean;
  placeholderRoleValue?: R;
  onMessageChange?: (v: PromptMessage<R>) => void;
  onMessageTypeChange?: (v: R) => void;
  children?: ReactNode;
  modalVariableEnable?: boolean;
  modalVariableBtnHidden?: boolean;
}

type PromptEditorType = <R extends string | number = Role>(
  props: PromptEditorProps<R> & {
    ref?: React.ForwardedRef<PromptBasicEditorRef>;
  },
) => JSX.Element;

export const PromptEditor = forwardRef(
  <R extends string | number>(
    props: PromptEditorProps<R>,
    ref?: React.ForwardedRef<PromptBasicEditorRef>,
  ) => {
    const editorRef = ref ?? useRef<PromptBasicEditorRef>(null);

    const {
      className,
      message,
      dragBtnHidden,
      messageTypeDisabled,
      variables,
      disabled,
      isDrag,
      onMessageChange,
      onMessageTypeChange,
      placeholder,
      messageTypeList,
      leftActionBtns,
      rightActionBtns,
      hideActionWrap,
      isFullscreen,
      placeholderRoleValue = Role.Placeholder as R,
      modalVariableEnable,
      children,
      modalVariableBtnHidden,
      ...rest
    } = props;
    const [editorActive, setEditorActive] = useState(false);
    const handleMessageContentChange = (v: string) => {
      const parts = splitMultimodalContent(v) as ContentPart[];
      onMessageChange?.({ ...message, content: parts.length ? '' : v, parts });
    };

    const readonly = disabled || isDrag;
    const [modalVariableVisible, setModalVariableVisible] = useState(false);
    const [modalVariableCanAdd, setModalVariableCanAdd] = useState(false);
    const formApiRef = useRef<FormApi>();

    const defaultValue = message?.parts?.length
      ? message?.parts
          ?.map(it => {
            if (it.type === ContentType.MultiPartVariable && it?.text) {
              return getMultimodalVariableText(it.text);
            }
            return it.text;
          })
          .join('')
      : message?.content;

    const placeholderError = getPlaceholderErrorContent(
      message as Message,
      variables,
    );

    const handleModalVariableConfirm = () => {
      const value = formApiRef.current?.getValues();
      if (value?.content) {
        setModalVariableVisible(false);
        setModalVariableCanAdd(false);
        const content = `<multimodal-variable>${value?.content}</multimodal-variable>`;
        (editorRef as any)?.current?.insertText(content);
      }
    };

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
                {!readonly &&
                !modalVariableBtnHidden &&
                message?.role !== placeholderRoleValue ? (
                  <TooltipWhenDisabled
                    disabled={!modalVariableVisible}
                    content={
                      modalVariableEnable
                        ? '多模态变量'
                        : '所选模型不支持多模态，请调整变量类型或更换模型'
                    }
                    theme="dark"
                  >
                    <span>
                      <Popconfirm
                        className="w-[300px]"
                        title="添加多模态变量"
                        content={
                          <Form
                            getFormApi={formApi =>
                              (formApiRef.current = formApi)
                            }
                            showValidateIcon={false}
                            onValueChange={values => {
                              setTimeout(() => {
                                const error =
                                  formApiRef.current?.getError('content');
                                if (values?.content && !error) {
                                  setModalVariableCanAdd(true);
                                } else {
                                  setModalVariableCanAdd(false);
                                }
                              }, 100);
                            }}
                          >
                            <FormTextArea
                              noLabel
                              field="content"
                              placeholder="输入多模态变量名称"
                              maxCount={50}
                              maxLength={50}
                              rules={[
                                {
                                  validator: (_rules, value, callback) => {
                                    const regex = new RegExp(
                                      `^[a-zA-Z][\\w]{0,${VARIABLE_MAX_LEN - 1}}$`,
                                      'gm',
                                    );

                                    if (value) {
                                      // 检查是否包含换行符
                                      if (
                                        value.includes('\n') ||
                                        value.includes('\r')
                                      ) {
                                        callback(
                                          '只能包含字母、数字或下划线，并且以字母开头',
                                        );
                                        return false;
                                      }
                                      if (regex.test(value)) {
                                        if (
                                          variables?.some(v => v.key === value)
                                        ) {
                                          callback('变量名重复');
                                          return false;
                                        }
                                        return true;
                                      } else {
                                        callback(
                                          '只能包含字母、数字或下划线，并且以字母开头',
                                        );
                                        return false;
                                      }
                                    }
                                    return true;
                                  },
                                },
                              ]}
                              rows={2}
                              showCounter
                              fieldClassName="!p-0"
                              autoFocus
                            />
                          </Form>
                        }
                        okText="确定"
                        okButtonProps={{
                          disabled: !modalVariableCanAdd,
                        }}
                        trigger="custom"
                        visible={modalVariableVisible}
                        onConfirm={handleModalVariableConfirm}
                        onClickOutSide={() => {
                          setModalVariableVisible(false);
                          setModalVariableCanAdd(false);
                        }}
                      >
                        <IconButton
                          color={
                            modalVariableVisible ? 'highlight' : 'secondary'
                          }
                          size="mini"
                          icon={
                            <Icon svg={<ModalVariableIcon fontSize={12} />} />
                          }
                          onClick={() => setModalVariableVisible(v => !v)}
                          disabled={!modalVariableEnable}
                        />
                      </Popconfirm>
                    </span>
                  </TooltipWhenDisabled>
                ) : null}
                {rightActionBtns}
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
                onInput={event => {
                  // 获取当前输入的值
                  const target = event.target as HTMLInputElement;
                  if (target) {
                    let { value } = target;
                    // 如果输入为空，不做处理
                    if (value === '') {
                      return;
                    }
                    // 确保首字母是字母
                    if (!/^[A-Za-z]/.test(value)) {
                      // 如果首字母不是字母，去掉首字母
                      value = value.slice(1);
                    }

                    // 确保其余部分只包含字母、数字和下划线
                    value = value.replace(/[^A-Za-z0-9_]/g, '');

                    // 更新输入框的值
                    target.value = value;
                  }
                }}
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
                placeholder="支持输入英文字母和下划线，且首字母必须是字母"
              />
            ) : (
              <PromptBasicEditor
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
              >
                <ModalVariableCompletion
                  isMultimodal={modalVariableEnable}
                  variableKeys={variables?.map(it => it.key || '')}
                  disabled={modalVariableBtnHidden}
                />
                {children}
              </PromptBasicEditor>
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
) as PromptEditorType;
