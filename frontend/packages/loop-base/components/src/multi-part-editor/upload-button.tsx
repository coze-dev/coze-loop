// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { forwardRef, useCallback, useImperativeHandle, useRef } from 'react';

import { ContentType } from '@cozeloop/api-schema/prompt';
import {
  IconCozImage,
  IconCozImageArrowUp,
  IconCozVideo,
} from '@coze-arch/coze-design/icons';
import {
  IconButton,
  Menu,
  Toast,
  Upload,
  type UploadProps,
} from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { type ContentPartLoop } from './type';

/* eslint-disable @typescript-eslint/no-explicit-any */
interface UploadBtnProps extends React.HTMLAttributes<HTMLButtonElement> {
  spaceID: string;
  disabled?: boolean;
  maxImageSize?: number;
  maxVideoSize?: number;
  maxFileCount?: number;
  imageEnabled?: boolean;
  videoEnabled?: boolean;
  imageSupportedFormats?: string[];
  videoSupportedFormats?: string[];
  fileLimit?: number;
  fileParts?: ContentPartLoop[];
  uploadFile?: (params: any) => Promise<string>;
  onFilePartsChange?: (part: ContentPartLoop) => void;
}
export interface UploadButtonRef {
  getUploadImage: () => Upload | null;
  getUploadVideo: () => Upload | null;
}

export const UploadButton = forwardRef<UploadButtonRef, UploadBtnProps>(
  (
    {
      spaceID,
      disabled,
      maxImageSize,
      maxVideoSize,
      maxFileCount,
      uploadFile,
      imageEnabled,
      videoEnabled,
      imageSupportedFormats,
      videoSupportedFormats,
      fileLimit,
      fileParts = [],
      onFilePartsChange,
    }: UploadBtnProps,
    ref,
  ) => {
    const I18n = useI18n();
    const uploadImgRef = useRef<Upload>(null);
    const uploadVideoRef = useRef<Upload>(null);

    const createUploadHandler =
      (fileType: 'image' | 'video'): UploadProps['customRequest'] =>
      ({ fileInstance, file, onProgress, onSuccess, onError }) => {
        const currentFileType = fileType;
        const { uid } = file;

        const reader = new FileReader();
        reader.readAsDataURL(fileInstance as Blob);
        reader.onload = async () => {
          const base64Url = reader.result as string;
          const partInfo =
            currentFileType === 'image'
              ? { image_url: { url: file.url, thumb_url: base64Url } }
              : { video_url: { url: file.url, thumb_url: base64Url } };
          const part: ContentPartLoop = {
            type:
              currentFileType === 'image'
                ? ContentType.ImageURL
                : ContentType.VideoURL,
            ...partInfo,
            status: 'uploading',
            uid,
          };

          try {
            onFilePartsChange?.(part);
            const res = await uploadFile?.({
              spaceID,
              file: fileInstance,
              fileType: 'image', // 不管是视频还是图片，都走imageX上传，所以fileType必须都是image
              onProgress,
              onSuccess,
              onError,
            });

            const fileInfo =
              currentFileType === 'image'
                ? {
                    image_url: {
                      url: file.url,
                      uri: res,
                      thumb_url: base64Url,
                    },
                  }
                : {
                    video_url: {
                      url: file.url,
                      uri: res,
                      thumb_url: base64Url,
                    },
                  };

            const newPart: ContentPartLoop = {
              ...part,
              ...fileInfo,
              status: 'success',
            };
            onFilePartsChange?.(newPart);
          } catch (error) {
            console.info('error', error);
            Toast.error(I18n.t('image_upload_error'));
            onFilePartsChange?.({
              ...part,
              status: 'uploadFail',
            });
          }
        };
      };

    const renderImageUpload = useCallback(
      (children: React.ReactNode) => (
        <Upload
          ref={uploadImgRef}
          action=""
          customRequest={createUploadHandler('image')}
          accept={imageSupportedFormats?.join(',') || 'image/*'}
          showUploadList={false}
          maxSize={maxImageSize ? maxImageSize * 1024 * 1024 : 0}
          limit={fileLimit}
          onSizeError={() =>
            Toast.error(`${I18n.t('image_size_exceed_max', { maxImageSize })}`)
          }
          onExceed={() =>
            Toast.warning(
              `${I18n.t('max_upload_images_limit', { maxFileCount })}`,
            )
          }
          multiple
          fileList={fileParts.map(it => ({
            uid: it.uid || '',
            url: it.image_url?.url,
            status: it.status || 'success',
            name: it.uid || '',
            size: '0',
          }))}
        >
          {children}
        </Upload>
      ),

      [fileParts, maxImageSize, fileLimit, imageSupportedFormats],
    );

    const renderVideoUpload = useCallback(
      (children: React.ReactNode) => (
        <Upload
          ref={uploadVideoRef}
          action=""
          customRequest={createUploadHandler('video')}
          accept={videoSupportedFormats?.join(',') || 'video/*'}
          showUploadList={false}
          maxSize={maxVideoSize ? maxVideoSize * 1024 : 0}
          limit={fileLimit}
          onSizeError={() =>
            Toast.error(`${I18n.t('video_size_exceed_max', { maxVideoSize })}`)
          }
          onExceed={() =>
            Toast.warning(
              `${I18n.t('max_upload_videos_limit', { maxFileCount })}`,
            )
          }
          multiple
          fileList={fileParts.map(it => ({
            uid: it.uid || '',
            url: it.video_url?.url,
            status: it.status || 'success',
            name: it.uid || '',
            size: '0',
          }))}
        >
          {children}
        </Upload>
      ),

      [fileParts, maxVideoSize, fileLimit, videoSupportedFormats],
    );

    useImperativeHandle(ref, () => ({
      getUploadImage: () => uploadImgRef.current,
      getUploadVideo: () => uploadVideoRef.current,
    }));

    if (!imageEnabled && !videoEnabled) {
      return null;
    }

    if (imageEnabled && !videoEnabled) {
      return renderImageUpload(
        <IconButton
          icon={<IconCozImage />}
          color="primary"
          disabled={disabled}
          data-btm="d41383"
          data-btm-title={I18n.t('image_upload')}
        />,
      );
    }

    if (!imageEnabled && videoEnabled) {
      return renderVideoUpload(
        <IconButton
          icon={<IconCozVideo />}
          color="primary"
          disabled={disabled}
          data-btm="d27622"
          data-btm-title={I18n.t('video_upload')}
        />,
      );
    }

    return (
      <>
        <Menu
          render={
            <Menu.SubMenu mode="menu">
              <Menu.Item
                onClick={() => {
                  uploadImgRef.current?.openFileDialog();
                }}
                data-btm="d41383"
                data-btm-title={I18n.t('image_upload')}
              >
                {I18n.t('image_upload')}
              </Menu.Item>
              <Menu.Item
                onClick={() => {
                  uploadVideoRef.current?.openFileDialog();
                }}
                data-btm="d27622"
                data-btm-title={I18n.t('video_upload')}
              >
                {I18n.t('video_upload')}
              </Menu.Item>
            </Menu.SubMenu>
          }
        >
          <IconButton
            icon={<IconCozImageArrowUp />}
            color="primary"
            disabled={disabled}
          />
        </Menu>
        {renderImageUpload(null)}
        {renderVideoUpload(null)}
      </>
    );
  },
);
