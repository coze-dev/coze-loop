// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/use-error-in-catch */
/* eslint-disable complexity */
import React, { useState } from 'react';

import classNames from 'classnames';
import { StorageProvider } from '@cozeloop/api-schema/data';
import {
  IconCozCross,
  IconCozEye,
  IconCozImageBroken,
  IconCozRefresh,
} from '@coze-arch/coze-design/icons';
import { Image, ImagePreview, Loading } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { ImageStatus, type MultipartItem } from '../type';

interface ImageItemRendererProps {
  className?: string;
  style?: React.CSSProperties;
  spaceID?: Int64;
  item: MultipartItem;
  readonly?: boolean;
  onRemove: () => void;
  onChange: (item: MultipartItem) => void;
  uploadFile?: (params: unknown) => Promise<string>;
}

export const ImageItemRenderer: React.FC<ImageItemRendererProps> = ({
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
  const [fileLoadError, setFileLoadError] = useState(false);
  const status = item?.sourceImage?.status;
  const uri = item?.image?.uri;
  const url = item?.image?.thumb_url || item?.image?.url;
  const isError = status === ImageStatus.Error;
  const file = item?.sourceImage?.file as File;
  const retryUpload = async () => {
    try {
      onChange({
        ...item,
        sourceImage: {
          ...item.sourceImage,
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
        sourceImage: {
          ...item.sourceImage,
          status: ImageStatus.Success,
        },
        image: {
          ...item.image,
          uri: newUri,
          storage_provider: StorageProvider.ImageX,
        },
      });
    } catch (error) {
      onChange({
        ...item,
        sourceImage: {
          ...item.sourceImage,
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
        <ImagePreview
          src={url}
          visible={visible}
          onVisibleChange={setVisible}
        />

        <Image
          src={url}
          className="w-full h-full rounded-[6px] border border-solid coz-stroke-plus"
          imgStyle={{ objectFit: 'contain', width: '100%', height: '100%' }}
          fallback={<IconCozImageBroken className="text-[24px]" />}
          onError={() => setFileLoadError(true)}
        />

        {status !== ImageStatus.Loading && (
          <div
            className={`absolute inset-0 flex gap-3 items-center rounded-[6px] justify-center coz-mg-mask ${isError ? 'visible' : 'invisible'}  group-hover:visible`}
          >
            {isError && !readonly && file ? (
              <IconCozRefresh
                className="text-white w-[16px] h-[16px] cursor-pointer"
                onClick={retryUpload}
              />
            ) : null}
            {(uri || url) && !fileLoadError ? (
              <IconCozEye
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
      {status === ImageStatus.Error && (
        <div className="text-sm text-red-500">{I18n.t('upload_fail')}</div>
      )}
    </div>
  );
};
