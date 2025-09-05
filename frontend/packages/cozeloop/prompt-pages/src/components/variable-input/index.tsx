/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import cn from 'classnames';
import { MultipartEditor, TextWithCopy } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { uploadFile } from '@cozeloop/biz-components-adapter';
import { useModalData } from '@cozeloop/base-hooks';
import {
  ContentType,
  type Message,
  TemplateType,
  VariableType,
  type VariableVal,
} from '@cozeloop/api-schema/prompt';
import {
  IconCozArrowDown,
  IconCozEdit,
  IconCozPlus,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  IconButton,
  Popconfirm,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { messageId } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { useBasicStore } from '@/store/use-basic-store';
import { VARIABLE_TYPE_ARRAY_MAP } from '@/consts';

import { VariableValueInput } from '../variables-card/variable-modal';
import { PlaceholderModal } from '../variables-card/placeholder-modal';

import styles from './index.module.less';

interface VariableInputProps {
  variableType?: VariableType;
  readonly?: boolean;
  variableVal?: VariableVal;
  onValueChange?: (params: VariableVal) => void;
  onDelete?: (key?: string) => void;
  onVariableChange?: (val?: VariableVal) => void;
}
export function VariableInput({
  variableVal,
  variableType,
  onValueChange,
  onDelete,
  readonly,
  onVariableChange,
}: VariableInputProps) {
  const { spaceID } = useSpace();
  const {
    key: variableKey,
    value: variableValue,
    placeholder_messages: placeholderMessages,
    multi_part_values: multiPartValues,
  } = variableVal ?? {};
  const [editorActive, setEditorActive] = useState(false);
  const placeholderModal = useModalData<Message[]>();
  const { templateType } = usePromptStore(
    useShallow(state => ({
      templateType: state.templateType,
    })),
  );
  const { setExecuteDisabled } = useBasicStore(
    useShallow(state => ({
      setExecuteDisabled: state.setExecuteDisabled,
    })),
  );
  const [collapse, setCollapse] = useState(false);

  const isPlaceholder = variableType === VariableType.Placeholder;
  const isMultiPart = variableType === VariableType.MultiPart;
  const isNormal = templateType === TemplateType.Normal;

  return (
    <div
      className={cn(styles['variable-input'], {
        [styles['variable-input-active']]: editorActive,
        '!pb-1': collapse,
      })}
    >
      <div className="flex items-center justify-between h-8">
        <div className="flex items-center gap-2">
          <TextWithCopy
            content={variableKey}
            maxWidth={200}
            copyTooltipText="复制变量名"
            textClassName="variable-text"
          />
          {isNormal ? null : (
            <Tag
              color="primary"
              className={cn(
                '!border !border-solid !coz-stroke-primary !bg-white',
                {
                  'cursor-default':
                    readonly || isPlaceholder || isNormal || isMultiPart,
                },
              )}
              onClick={e => {
                if (!readonly && !isPlaceholder && !isNormal && !isMultiPart) {
                  onVariableChange?.({
                    key: variableKey,
                    value: variableValue,
                  });
                }

                e.stopPropagation();
              }}
            >
              {
                VARIABLE_TYPE_ARRAY_MAP[
                  (variableType ??
                    VariableType.String) as keyof typeof VARIABLE_TYPE_ARRAY_MAP
                ]
              }
              {readonly || isPlaceholder || isMultiPart ? null : (
                <IconCozEdit className="ml-1" />
              )}
            </Tag>
          )}
          <IconCozArrowDown
            className={cn('cursor-pointer', {
              '-rotate-90': collapse,
            })}
            onClick={() => setCollapse(!collapse)}
          />
        </div>
        {readonly ? (
          <IconButton
            className={styles['delete-btn']}
            icon={<IconCozTrashCan />}
            size="small"
            color="secondary"
            disabled={readonly}
          />
        ) : (
          <Popconfirm
            title="删除变量"
            content="将删除 Prompt 模板中的该变量。确认删除吗？"
            cancelText="取消"
            okText="删除"
            okButtonProps={{ color: 'red' }}
            onConfirm={() => onDelete?.(variableKey)}
          >
            <IconButton
              className={styles['delete-btn']}
              icon={<IconCozTrashCan />}
              size="mini"
              color="secondary"
              disabled={readonly}
            />
          </Popconfirm>
        )}
      </div>
      <div
        className={cn('h-fit', {
          hidden: collapse,
        })}
      >
        {isPlaceholder ? (
          <>
            {placeholderMessages?.length ? (
              <div className="flex flex-col gap-2">
                {placeholderMessages.map(message => (
                  <div className={styles['placeholder-message-wrap']}>
                    <div className={styles['placeholder-message-header']}>
                      {message.role ?? '-'}
                    </div>
                    <div className="px-3 py-1 min-h-[20px]">
                      <Typography.Text size="small">
                        {message.content}
                      </Typography.Text>
                    </div>
                  </div>
                ))}
              </div>
            ) : null}
            <div>
              <Button
                color="primary"
                className="mt-1"
                onClick={() => {
                  const messages = placeholderMessages?.map(
                    (item: Message & { id?: string }) => {
                      if (!item.id || item.id === '0') {
                        return {
                          ...item,
                          id: messageId(),
                        };
                      }
                      return item;
                    },
                  );
                  placeholderModal.open(messages as any);
                }}
                size="small"
                icon={<IconCozPlus />}
              >
                添加数据
              </Button>
            </div>
          </>
        ) : null}
        {isMultiPart ? (
          <MultipartEditor
            value={
              multiPartValues?.map(it => ({
                content_type:
                  it.type === ContentType.ImageURL ? 'Image' : 'Text',
                text: it.text,
                image: it.image_url,
              })) as any
            }
            uploadFile={(params: any) => {
              setExecuteDisabled(true);
              return uploadFile?.(params).finally(() => {
                setExecuteDisabled(false);
              });
            }}
            spaceID={spaceID}
            onChange={value => {
              onValueChange?.({
                key: variableKey,
                multi_part_values: value?.map(it => ({
                  type:
                    it.content_type === 'Image'
                      ? ContentType.ImageURL
                      : ContentType.Text,
                  text: it.text,
                  image_url: it.image,
                })),
              });
            }}
          />
        ) : null}
        {isPlaceholder || isMultiPart ? null : (
          <VariableValueInput
            typeValue={variableType}
            value={variableValue}
            onChange={value => onValueChange?.({ key: variableKey, value })}
            inputConfig={{
              borderless: true,
              inputClassName: styles['loop-variable-input'],
              onFocus: () => {
                setEditorActive(true);
              },
              onBlur: () => {
                setEditorActive(false);
              },
              size: 'small',
            }}
            minHeight={26}
            maxHeight={128}
          />
        )}
      </div>
      <PlaceholderModal
        visible={placeholderModal.visible}
        onCancel={placeholderModal.close}
        onOk={messageList => {
          onValueChange?.({
            key: variableKey,
            placeholder_messages: messageList as any,
          });
          placeholderModal.close();
        }}
        data={placeholderModal.data}
        variableKey={variableKey || ''}
      />
    </div>
  );
}
