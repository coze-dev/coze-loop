// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/use-error-in-catch */
/* eslint-disable complexity */
import React, { useState } from 'react';

import { StorageProvider } from '@cozeloop/api-schema/data';
import {
  IconCozEye,
  IconCozImageBroken,
  IconCozRefresh,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import { Image, ImagePreview, Loading } from '@coze-arch/coze-design';

import { ImageStatus, type MultipartItem } from '../type';

interface ImageItemRendererProps {
  spaceID?: Int64;
  item: MultipartItem;
  readonly?: boolean;
  onRemove: () => void;
  onChange: (item: MultipartItem) => void;
  uploadFile?: (params: unknown) => Promise<string>;
}

export const ImageItemRenderer: React.FC<ImageItemRendererProps> = ({
  spaceID,
  item,
  onRemove,
  onChange,
  uploadFile,
  readonly,
}) => {
  const [visible, setVisible] = useState(false);
  const [fileLoadError, setFileLoadError] = useState(false);
  const status = item?.sourceImage?.status;
  const uri = item?.image?.uri;
  const url = item?.image?.url;
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
    <div className="flex flex-col ">
      <div className="w-[64px] h-[64px] relative group">
        <ImagePreview
          src={item?.image?.url}
          visible={visible}
          onVisibleChange={setVisible}
        />
        <Image
          src={item?.image?.url}
          className="rounded-[6px]"
          width={64}
          height={64}
          imgStyle={{ objectFit: 'contain' }}
          fallback={<IconCozImageBroken className="text-[24px]" />}
          onError={() => setFileLoadError(true)}
        />
        {status !== ImageStatus.Loading && (
          <div
            className={`absolute inset-0 flex gap-3 items-center rounded-[6px] justify-center bg-[rgba(0,0,0,0.4)] ${isError ? 'visible' : 'invisible'}  group-hover:visible`}
          >
            {isError && !readonly ? (
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
              <IconCozTrashCan
                className="text-white w-[16px] h-[16px] cursor-pointer"
                onClick={onRemove}
              />
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
        <div className="text-center text-sm text-red-500">上传失败</div>
      )}
    </div>
  );
};
