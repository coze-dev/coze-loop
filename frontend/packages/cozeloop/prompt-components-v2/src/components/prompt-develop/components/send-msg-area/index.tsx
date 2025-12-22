// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable @typescript-eslint/no-magic-numbers */
/* eslint-disable complexity */
import { useEffect, useMemo, useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import cn from 'classnames';
import { useLatest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type ContentPartLoop,
  DEFAULT_VIDEO_SUPPORTED_FORMATS,
  MultiPartRender,
  UploadButton,
  type UploadButtonRef,
  useReportEvent,
} from '@cozeloop/components';
import {
  ContentType,
  type Message,
  Role,
  VariableType,
} from '@cozeloop/api-schema/prompt';
import {
  IconCozBroom,
  IconCozInfoCircle,
  IconCozPlayCircle,
  IconCozStopCircle,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  IconButton,
  Space,
  Toast,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';
import { EditorView, keymap } from '@codemirror/view';
import { type Extension, Prec } from '@codemirror/state';

import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import {
  MAX_FILE_SIZE,
  MAX_FILE_SIZE_MB,
  MAX_IMAGE_FILE,
  MessageListGroupType,
} from '@/consts';
import { BasicPromptEditor } from '@/components/basic-prompt-editor';

import { usePromptDevProviderContext } from '../prompt-provider';
import { GroupSelect } from './group-select';

import styles from './index.module.less';

interface SendMsgAreaProps {
  streaming?: boolean;
  isInSubArea?: boolean;
  onMessageSend?: (queryMsg?: Message) => void;
  stopStreaming?: () => void;
  onClearHistory?: () => void;
}

export function SendMsgArea({
  streaming,
  onMessageSend,
  stopStreaming,
  onClearHistory,
  isInSubArea,
}: SendMsgAreaProps) {
  const sendEvent = useReportEvent();
  const {
    readonly: globalReadonly,
    uploadFile,
    spaceID,
  } = usePromptDevProviderContext();
  const [editorActive, setEditorActive] = useState(false);

  const [queryMsg, setQueryMsg] = useState<Message>({
    role: Role.User,
  });
  const [queryMsgKey, setQueryMsgKey] = useState<string>(nanoid());

  const { groupType, executeDisabled } = useBasicStore(
    useShallow(state => ({
      groupType: state.groupType,
      executeDisabled: state.executeDisabled,
    })),
  );

  const isMultiGroup = groupType === MessageListGroupType.Multi;

  const { variables, currentModel } = usePromptStore(
    useShallow(state => ({
      variables: state.variables,
      messageList: state.messageList,
      currentModel: state.currentModel,
    })),
  );

  const {
    setHistoricMessage,
    historicMessage,
    compareConfig,
    setHistoricMessageById,
  } = usePromptMockDataStore(
    useShallow(state => ({
      setHistoricMessage: state.setHistoricMessage,
      userDebugConfig: state.userDebugConfig,
      historicMessage: state.historicMessage,
      setHistoricMessageById: state.setHistoricMessageById,
      compareConfig: state.compareConfig,
    })),
  );

  const isCompare = Boolean(compareConfig?.groups?.length);

  const uploadRef = useRef<UploadButtonRef>(null);

  const historicFileParts =
    historicMessage
      ?.map(
        it => it?.parts?.filter(item => item.type !== ContentType.Text) || [],
      )
      ?.flat() || [];

  const fileParts: ContentPartLoop[] =
    queryMsg?.parts?.filter(it => it.type !== ContentType.Text) || [];

  const fileCount = historicFileParts.length + fileParts.length;
  const canUploadFileSize = MAX_IMAGE_FILE - historicFileParts.length;

  const maxImageSize = useMemo(() => {
    const imgSize =
      currentModel?.ability?.ability_multi_modal?.ability_image?.max_image_size;
    if (!imgSize || imgSize === '-1') {
      return MAX_FILE_SIZE_MB;
    }
    return Number(imgSize);
  }, [
    currentModel?.ability?.ability_multi_modal?.ability_image?.max_image_size,
  ]);

  const maxImageCount = useMemo(() => {
    const imgCount =
      currentModel?.ability?.ability_multi_modal?.ability_image
        ?.max_image_count;
    if (!imgCount || imgCount === '-1') {
      return MAX_IMAGE_FILE;
    }
    return Number(imgCount);
  }, [
    currentModel?.ability?.ability_multi_modal?.ability_image?.max_image_count,
  ]);

  const isMaxImgSize = Boolean(fileCount >= MAX_IMAGE_FILE);
  const isMaxImgSizeRef = useLatest(isMaxImgSize);

  const fileUploading = fileParts.some(it => it.status === 'uploading');

  const inputReadonly = streaming;

  const currentExecuteDisabled =
    streaming ||
    fileUploading ||
    !currentModel?.model_id ||
    globalReadonly ||
    executeDisabled;

  const isMultiModal = currentModel?.ability?.multi_modal;
  const isMultiModalRef = useLatest(isMultiModal);
  const isImageEnableRef = useLatest(
    currentModel?.ability?.ability_multi_modal?.image,
  );
  const isVideoEnableRef = useLatest(
    currentModel?.ability?.ability_multi_modal?.video,
  );
  const videoSupportedFormats = DEFAULT_VIDEO_SUPPORTED_FORMATS;
  const videoSupportedFormatsRef = useLatest(videoSupportedFormats);

  const removePart = (part: ContentPartLoop) => {
    setQueryMsg(v => ({
      ...v,
      parts: (v?.parts || []).filter(
        (it: ContentPartLoop) => it.uid !== part.uid,
      ),
    }));
  };

  const handleSendMessage = () => {
    if (currentExecuteDisabled) {
      return;
    }

    onMessageSend?.(queryMsg);
    setQueryMsg({ role: Role.User });
    setQueryMsgKey(nanoid());
  };
  const handleSendMessageRef = useLatest(handleSendMessage);

  const handleUploadImgByEditor = (items?: DataTransferItemList) => {
    if (items?.length && isMultiModalRef.current) {
      for (const item of Array.from(items)) {
        if (item.type.includes('image')) {
          if (!isImageEnableRef.current) {
            Toast.warning(I18n.t('prompt_model_not_support_multimodal_image'));
            return;
          }
          if (isMaxImgSizeRef.current) {
            Toast.warning(
              `${I18n.t('prompt_max_upload_MAX_IMAGE_FILE_images', { MAX_IMAGE_FILE })}`,
            );
            return;
          }
          const file = item.getAsFile();
          if (file) {
            if (file.size / 1024 > MAX_FILE_SIZE) {
              Toast.error(
                `${I18n.t('prompt_image_size_max_MAX_FILE_SIZE_MB_MB', { MAX_FILE_SIZE_MB })}`,
              );
              return;
            }
            const uploadImage = uploadRef.current?.getUploadImage();
            uploadImage?.insert([file], 0);
            uploadImage?.upload();
          }
        } else if (item.type.includes('video')) {
          if (!isVideoEnableRef.current) {
            Toast.warning(I18n.t('prompt_model_not_support_multimodal_video'));
            return;
          }
          const videoType = item.type.replace('video/', '');
          if (
            !videoSupportedFormatsRef.current.some(
              it =>
                it.toLocaleLowerCase() === `.${videoType}`.toLocaleLowerCase(),
            )
          ) {
            Toast.warning(I18n.t('prompt_model_not_support_this_video_type'));
            return;
          }
          const file = item.getAsFile();
          if (file) {
            const uploadVideo = uploadRef.current?.getUploadVideo();
            uploadVideo?.insert([file], 0);
            uploadVideo?.upload();
          }
        }
      }
    }
  };

  const clearHistoricChat = () => {
    setHistoricMessage([]);
    compareConfig?.groups?.forEach((_, idx) => setHistoricMessageById(idx, []));
    onClearHistory?.();
  };

  const extensions: Extension[] = useMemo(
    () => [
      EditorView.theme({
        '.cm-gutters': {
          backgroundColor: 'transparent',
          borderRight: 'none',
        },
        '.cm-scroller': {
          paddingLeft: '10px',
          paddingRight: '6px !important',
        },
      }),
      Prec.high(
        keymap.of([
          {
            key: 'Enter',
            run: () => {
              handleSendMessageRef?.current();
              sendEvent(I18n.t('run'), {}, 'd72221');
              return true;
            },
          },
        ]),
      ),
      EditorView.domEventObservers({
        drop(event) {
          const items = event?.dataTransfer?.items;
          const hasImg = Array.from(items || []).some(it =>
            it.type.includes('image'),
          );
          if (hasImg) {
            event.preventDefault();
          }
          handleUploadImgByEditor(items);
          return true;
        },
        paste(event) {
          const items = event.clipboardData?.items;
          handleUploadImgByEditor(items);
          return true;
        },
      }),
    ],

    [],
  );

  const onFilePartsChange = (part: ContentPartLoop) => {
    setQueryMsg(v => {
      const oldParts = v?.parts || [];
      const hasPart = oldParts.some(
        (it: ContentPartLoop) => it.uid === part.uid,
      );
      const newArr = hasPart ? oldParts : [...oldParts, part];
      return {
        ...v,
        parts: newArr.map((it: ContentPartLoop) => {
          if (it.uid === part.uid) {
            return part;
          }
          return it;
        }),
      };
    });
  };

  useEffect(() => {
    if (!isMultiModal) {
      setQueryMsg(prev => ({
        ...prev,
        parts: [],
      }));
    }
  }, [isMultiModal]);

  return (
    <div className={styles['send-msg-area']}>
      <div className="flex items-center justify-end">
        {isCompare ? null : <GroupSelect streaming={streaming} />}
        <div className="flex-1 flex items-center justify-center">
          {streaming && stopStreaming ? (
            <Space align="center">
              <Button
                color="primary"
                icon={<IconCozStopCircle />}
                size="mini"
                onClick={stopStreaming}
              >
                {isMultiGroup
                  ? I18n.t('prompt_stop_all_responses')
                  : I18n.t('stop_respond')}
              </Button>
            </Space>
          ) : null}
        </div>
        {isCompare ? null : (
          <Tooltip content={I18n.t('clear_history_messages')} theme="dark">
            <IconButton
              icon={<IconCozBroom />}
              onClick={clearHistoricChat}
              color="secondary"
              disabled={streaming}
            />
          </Tooltip>
        )}
        {isInSubArea ? (
          <Button
            icon={<IconCozPlayCircle />}
            onClick={handleSendMessage}
            disabled={currentExecuteDisabled}
            size="small"
            data-btm="d72221"
          >
            {I18n.t('run')}
          </Button>
        ) : null}
      </div>
      {isInSubArea ? null : (
        <div
          className={cn(styles['send-msg-area-content'], {
            [styles['editor-active']]: editorActive,
          })}
        >
          {fileParts?.length ? (
            <MultiPartRender
              fileParts={fileParts}
              onDeleteFilePart={removePart}
              onFilePartsChange={onFilePartsChange}
            />
          ) : null}
          <div className={cn('w-full flex-1 gap-0.5')}>
            <BasicPromptEditor
              key={queryMsgKey}
              defaultValue={queryMsg?.content}
              onChange={value =>
                setQueryMsg(v => ({
                  ...v,
                  content: value,
                }))
              }
              height={44}
              variables={variables?.filter(
                it => it.type === VariableType.String,
              )}
              readOnly={streaming || inputReadonly}
              linePlaceholder={I18n.t('input_question_tip')}
              customExtensions={extensions}
              onFocus={() => setEditorActive(true)}
              onBlur={() => setEditorActive(false)}
            />
          </div>
          <div className="flex items-center justify-between w-full gap-0.5 px-3">
            <div className="flex items-center gap-2">
              {isCompare ? (
                <Tooltip
                  content={I18n.t('clear_history_messages')}
                  theme="dark"
                >
                  <IconButton
                    icon={<IconCozBroom />}
                    onClick={clearHistoricChat}
                    color="secondary"
                    disabled={streaming}
                  />
                </Tooltip>
              ) : null}
              <UploadButton
                data-btm="d41383"
                ref={uploadRef}
                spaceID={spaceID}
                imageEnabled={currentModel?.ability?.ability_multi_modal?.image}
                videoEnabled={currentModel?.ability?.ability_multi_modal?.video}
                disabled={inputReadonly}
                maxImageSize={maxImageSize}
                maxVideoSize={50}
                maxFileCount={maxImageCount}
                fileLimit={canUploadFileSize}
                videoSupportedFormats={videoSupportedFormats}
                fileParts={fileParts}
                uploadFile={params => {
                  if (uploadFile) {
                    return uploadFile(params);
                  }
                  return Promise.resolve('');
                }}
                onFilePartsChange={onFilePartsChange}
              />

              {isMultiModal ? (
                <Typography.Text size="small" type="tertiary">
                  {fileCount} / 20
                </Typography.Text>
              ) : (
                <Typography.Text
                  size="small"
                  type="tertiary"
                  icon={<IconCozInfoCircle />}
                >
                  {I18n.t('model_not_support_picture')}
                </Typography.Text>
              )}
            </div>
            <Button
              icon={<IconCozPlayCircle />}
              onClick={handleSendMessage}
              disabled={currentExecuteDisabled}
              data-btm="d72221"
            >
              {I18n.t('run')}
            </Button>
          </div>
        </div>
      )}
      <Typography.Text size="small" type="tertiary" className="text-center">
        {I18n.t('generated_by_ai_tip')}
      </Typography.Text>
    </div>
  );
}
