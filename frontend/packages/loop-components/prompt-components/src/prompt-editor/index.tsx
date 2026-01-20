// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable security/detect-non-literal-regexp */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import {
  forwardRef,
  type ReactNode,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import cn from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
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
      placeholderRoleValue,
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
                      modalVariableEnable ? (
                        <div className="flex flex-col">
                          <Typography.Text className="text-white">
                            {I18n.t('prompt_add_new_multi_modal_variable')}
                          </Typography.Text>
                          <Typography.Text
                            style={{ color: 'rgba(227, 232, 250, 0.46)' }}
                            type="secondary"
                            size="small"
                          >
                            {I18n.t(
                              'prompt_support_multi_modal_in_prompt_via_variable',
                            )}
                          </Typography.Text>
                        </div>
                      ) : (
                        I18n.t('selected_model_not_support_multi_modal')
                      )
                    }
                    theme="dark"
                  >
                    <span>
                      <Popconfirm
                        className="w-[300px]"
                        title={I18n.t('prompt_add_multi_modal_variable')}
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
                              placeholder={I18n.t(
                                'prompt_input_multi_modal_variable_name',
                              )}
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
                                          I18n.t(
                                            'prompt_variable_name_rule_letters_numbers_underscore',
                                          ),
                                        );
                                        return false;
                                      }
                                      if (regex.test(value)) {
                                        if (
                                          variables?.some(v => v.key === value)
                                        ) {
                                          callback(
                                            I18n.t(
                                              'prompt_variable_name_duplicate',
                                            ),
                                          );
                                          return false;
                                        }
                                        return true;
                                      } else {
                                        callback(
                                          I18n.t(
                                            'prompt_variable_name_rule_letters_numbers_underscore',
                                          ),
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
                        okText={I18n.t('confirm')}
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
