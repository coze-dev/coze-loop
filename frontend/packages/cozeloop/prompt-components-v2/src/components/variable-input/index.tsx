/* eslint-disable security/detect-object-injection */
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useMemo, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import cn from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { useModalData } from '@cozeloop/hooks';
import {
  MultipartEditor,
  TextWithCopy,
  TooltipWhenDisabled,
} from '@cozeloop/components';
import {
  type ContentPart,
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
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import { messageId } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { useBasicStore } from '@/store/use-basic-store';
import { VARIABLE_TYPE_ARRAY_MAP } from '@/consts';

import { VariableValueInput } from '../prompt-develop/components/variables-card/variable-modal';
import { PlaceholderModal } from '../prompt-develop/components/variables-card/placeholder-modal';
import { usePromptDevProviderContext } from '../prompt-develop/components/prompt-provider';
import { VideoConfig } from './video-config';

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
  const { spaceID, uploadFile, multiModalConfig } =
    usePromptDevProviderContext();
  const {
    key: variableKey,
    value: variableValue,
    placeholder_messages: placeholderMessages,
    multi_part_values: multiPartValues,
  } = variableVal ?? {};
  const [editorActive, setEditorActive] = useState(false);
  const placeholderModal = useModalData<Message[]>();
  const {
    templateType,
    currentModel,
    variablesVersionMap,
    snippetMap,
    promptInfo,
  } = usePromptStore(
    useShallow(state => ({
      templateType: state.templateType,
      currentModel: state.currentModel,
      variablesVersionMap: state.variablesVersionMap,
      snippetMap: state.snippetMap,
      promptInfo: state.promptInfo,
    })),
  );

  const hasSnippet = Boolean(
    (promptInfo?.prompt_draft || promptInfo?.prompt_commit)?.detail
      ?.prompt_template?.has_snippet,
  );

  const { setExecuteDisabled } = useBasicStore(
    useShallow(state => ({
      setExecuteDisabled: state.setExecuteDisabled,
    })),
  );
  const [collapse, setCollapse] = useState(false);

  const isPlaceholder = variableType === VariableType.Placeholder;
  const isMultiPart = variableType === VariableType.MultiPart;
  const isNormal = templateType?.type === TemplateType.Normal;
  const isMultiModal = Boolean(currentModel?.ability?.multi_modal);
  const imageEnabled = Boolean(
    isMultiModal && currentModel?.ability?.ability_multi_modal?.image,
  );
  const videoEnabled = Boolean(
    isMultiModal && currentModel?.ability?.ability_multi_modal?.video,
  );

  const inCurrentPrompt = Boolean(
    variableKey && variablesVersionMap?.[variableKey]?.includes('Prompt'),
  );
  const inSnippetArray =
    (variableKey &&
      variablesVersionMap?.[variableKey]?.filter(it => it !== 'Prompt')) ||
    [];
  const snippetNames = inSnippetArray.map(it => {
    const snippet = snippetMap?.[it];
    return (
      <div className="inline-flex items-center gap-1 flex-1 px-0.5">
        <Typography.Text
          ellipsis={{ showTooltip: true }}
          className="!text-white"
        >
          {snippet?.prompt_basic?.display_name}
        </Typography.Text>
        <Tag size="mini" color="brand" className="flex-shrink-0">
          {snippet?.prompt_commit?.commit_info?.version}
        </Tag>
      </div>
    );
  });

  const deleteAble =
    !readonly &&
    ((inCurrentPrompt && !inSnippetArray?.length) ||
      (!isNormal && !inCurrentPrompt && !inSnippetArray?.length));

  const disableTip = useMemo(() => {
    if (inSnippetArray?.length) {
      if (inCurrentPrompt) {
        return I18n.t('prompt_cannot_delete_snippet_variables');
      } else {
        return I18n.t('prompt_snippet_variables_no_delete');
      }
    }
    return '';
  }, [inCurrentPrompt, inSnippetArray?.length]);

  const [multiPartConfig, setMultiPartConfig] = useState<ContentPart>();

  useEffect(() => {
    if (isMultiPart && videoEnabled && multiPartValues?.length) {
      const firstVideo = multiPartValues.find(
        it => it.type === ContentType.VideoURL,
      );

      setMultiPartConfig(firstVideo);
    }
  }, [isMultiPart, videoEnabled, multiPartValues]);

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
            copyTooltipText={I18n.t('copy_variable_name')}
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

          {hasSnippet ? (
            <>
              {inSnippetArray?.length ? (
                <Tooltip
                  content={
                    <div className="max-h-[350px] overflow-auto">
                      {I18n.t('prompt_variable_referenced_in_snippets', {
                        snippetNames,
                      })}
                    </div>
                  }
                >
                  <Tag color="primary" size="mini">
                    {I18n.t('prompt_number_of_snippets', {
                      placeholder1: inSnippetArray.length,
                    })}
                  </Tag>
                </Tooltip>
              ) : null}
              {inCurrentPrompt ? (
                <Tag color="primary" size="mini">
                  Prompt
                </Tag>
              ) : null}
            </>
          ) : null}
        </div>
        <Space spacing={4}>
          {videoEnabled && isMultiPart ? (
            <VideoConfig
              value={multiPartConfig}
              onChange={v => {
                setMultiPartConfig(v);
                const newMultiPartValues = multiPartValues?.map(it => {
                  if (it.type === ContentType.VideoURL) {
                    return {
                      ...it,
                      video_url: {
                        ...it.video_url,
                      },
                      media_config: v.media_config,
                    };
                  }
                  return it;
                });
                onValueChange?.({
                  key: variableKey,
                  multi_part_values: newMultiPartValues,
                });
              }}
            />
          ) : null}
          {!deleteAble ? (
            <TooltipWhenDisabled
              content={disableTip}
              disabled={Boolean(disableTip)}
            >
              <IconButton
                className={styles['delete-btn']}
                icon={<IconCozTrashCan />}
                size="small"
                color="secondary"
                disabled
              />
            </TooltipWhenDisabled>
          ) : (
            <Popconfirm
              title={I18n.t('delete_variable')}
              content={I18n.t('confirm_delete_var_in_tpl')}
              cancelText={I18n.t('cancel')}
              okText={I18n.t('delete')}
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
        </Space>
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
                {I18n.t('add_data')}
              </Button>
            </div>
          </>
        ) : null}
        {isMultiPart ? (
          <MultipartEditor
            value={
              multiPartValues?.map(it => ({
                content_type:
                  it.type === ContentType.ImageURL
                    ? 'Image'
                    : it.type === ContentType.VideoURL
                      ? 'Video'
                      : 'Text',
                text: it.text,
                image: it.image_url,
                video: it.video_url,
                media_config: it.media_config,
              })) as any
            }
            uploadFile={(params: any) => {
              setExecuteDisabled(true);
              if (uploadFile) {
                return uploadFile(params).finally(() => {
                  setExecuteDisabled(false);
                });
              }
              return Promise.resolve('');
            }}
            spaceID={spaceID}
            onChange={value => {
              onValueChange?.({
                key: variableKey,
                multi_part_values: value?.map(it => ({
                  ...it,
                  type:
                    it.content_type === 'Image'
                      ? ContentType.ImageURL
                      : it.content_type === 'Video'
                        ? ContentType.VideoURL
                        : ContentType.Text,
                  text: it.text,
                  image_url: it.image,
                  video_url: it.video,
                })),
              });
            }}
            multipartConfig={{
              imageEnabled: isMultiModal || imageEnabled,
              videoEnabled: isMultiModal && videoEnabled,
            }}
            readonly={!isMultiModal}
            intranetUrlValidator={
              multiModalConfig?.intranetUrlValidator ??
              (url => url.includes('localhost'))
            }
            imageHidden={!multiModalConfig?.imageSupported}
            videoHidden={!multiModalConfig?.videoSupported}
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
