// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/use-error-in-catch */
/* eslint-disable complexity */
import React, { useState } from 'react';

import classNames from 'classnames';
import { StorageProvider } from '@cozeloop/api-schema/data';
import {
  IconCozCross,
  IconCozPlayCircle,
  IconCozRefresh,
} from '@coze-arch/coze-design/icons';
import { Loading, Modal } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { ImageStatus, type MultipartItem } from '../type';

import styles from './index.module.less';

interface VideoItemRendererProps {
  className?: string;
  style?: React.CSSProperties;
  spaceID?: Int64;
  item: MultipartItem;
  readonly?: boolean;
  onRemove: () => void;
  onChange: (item: MultipartItem) => void;
  uploadFile?: (params: unknown) => Promise<string>;
}

export const VideoItemRenderer: React.FC<VideoItemRendererProps> = ({
  className,
  style,
  spaceID,
  item,
  onRemove,
  onChange,
  uploadFile,
  readonly,
}) => {
  const I18n = useI18n();
  const [visible, setVisible] = useState(false);
  const status = item?.sourceVideo?.status;
  const uri = item?.video?.uri;
  const url = item?.video?.url;
  const isError = status === ImageStatus.Error;
  const file = item?.sourceVideo?.file as File;
  const retryUpload = async () => {
    try {
      onChange({
        ...item,
        sourceVideo: {
          ...item.sourceVideo,
          status: ImageStatus.Loading,
        },
      });
      const newUri = await uploadFile?.({
        file,
        fileType: 'image',
        spaceID,
      });
      onChange({
        ...item,
        sourceVideo: {
          ...item.sourceVideo,
          status: ImageStatus.Success,
        },
        video: {
          ...item.video,
          uri: newUri,
          storage_provider: StorageProvider.ImageX,
        },
      });
    } catch (error) {
      onChange({
        ...item,
        sourceVideo: {
          ...item.sourceVideo,
          status: ImageStatus.Error,
        },
      });
    }
  };

  return (
    <div className="flex flex-col">
      <div
        className={classNames('w-[64px] h-[64px] relative group', className)}
        style={style}
      >
        <video
          src={url}
          className="w-full h-full rounded-[6px] border border-solid coz-stroke-plus"
        />

        {status === ImageStatus.Loading ? null : (
          <IconCozPlayCircle
            className="text-white w-[16px] h-[16px] absolute top-[50%] left-[50%] translate-x-[-50%] translate-y-[-50%]"
            onClick={() => setVisible(true)}
          />
        )}
        {status !== ImageStatus.Loading && (
          <div
            className={`absolute inset-0 flex gap-3 items-center rounded-[6px] justify-center bg-[rgba(0,0,0,0.4)] ${isError ? 'visible' : 'invisible'}  group-hover:visible`}
          >
            {isError && !readonly && file ? (
              <IconCozRefresh
                className="text-white w-[16px] h-[16px] cursor-pointer"
                onClick={retryUpload}
              />
            ) : null}
            {uri || url ? (
              <IconCozPlayCircle
                className="text-white w-[16px] h-[16px] cursor-pointer"
                onClick={() => setVisible(true)}
              />
            ) : null}
            {readonly ? null : (
              <div className="p-[2px] text-white cursor-pointer absolute right-[-4px] top-[-4px] z-10 bg-[var(--coz-fg-secondary)] rounded-[16px] flex items-end justify-center">
                <IconCozCross onClick={onRemove} className="text-[12px]" />
              </div>
            )}
          </div>
        )}
        {status === ImageStatus.Loading && (
          <div className="absolute inset-0 flex items-center rounded-[6px] justify-center bg-[rgba(0,0,0,0.4)] z-10">
            <Loading loading color="blue" />
          </div>
        )}
      </div>
      {status === ImageStatus.Error ? (
        <div className="text-sm text-red-500">{I18n.t('upload_fail')}</div>
      ) : null}
      <Modal
        title={I18n.t('video_preview')}
        footer={null}
        visible={visible}
        onCancel={() => setVisible(false)}
        footerFill={false}
        hasScroll={false}
        width={654}
        className={styles['video-preview-modal']}
      >
        <video src={url} className="w-full h-[368px]" controls autoPlay />
      </Modal>
    </div>
  );
};
