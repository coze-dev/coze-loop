// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/use-error-in-catch */
/* eslint-disable complexity */
import React, { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { uploadFile } from '@cozeloop/biz-components-adapter';
import { StorageProvider } from '@cozeloop/api-schema/data';
import {
  IconCozEye,
  IconCozRefresh,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import { Image, ImagePreview, Loading } from '@coze-arch/coze-design';

import { ImageStatus, type MultipartItem } from '../../type';

interface ImageItemRendererProps {
  item: MultipartItem;
  onRemove: () => void;
  onChange: (item: MultipartItem) => void;
}

export const ImageItemRenderer: React.FC<ImageItemRendererProps> = ({
  item,
  onRemove,
  onChange,
}) => {
  const { spaceID } = useSpace();
  const [visible, setVisible] = useState(false);
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
      const newUri = await uploadFile({
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
    <div className="flex  flex-col ">
      <div className="w-[80px] h-[80px] relative group">
        <ImagePreview
          src={item?.image?.url}
          visible={visible}
          onVisibleChange={setVisible}
        />

        <Image
          src={item?.image?.url}
          className="rounded-[6px]"
          width={80}
          height={80}
        />

        {status !== ImageStatus.Loading && (
          <div
            className={`absolute inset-0 flex  items-center rounded-[6px] justify-center bg-[rgba(0,0,0,0.4)] ${isError ? 'visible' : 'invisible'}  group-hover:visible`}
          >
            {isError ? (
              <div
                className="w-[32px] h-[32px] flex items-center justify-center cursor-pointer "
                onClick={retryUpload}
              >
                <IconCozRefresh className="text-white w-[18px] h-[18px]" />
              </div>
            ) : null}
            {uri && url ? (
              <div
                className="w-[32px] h-[32px] flex items-center justify-center  cursor-pointer"
                onClick={() => setVisible(true)}
              >
                <IconCozEye className="text-white w-[18px] h-[18px]" />
              </div>
            ) : null}
            <div
              className="w-[32px] h-[32px] flex items-center justify-center  cursor-pointer"
              onClick={onRemove}
            >
              <IconCozTrashCan className="text-white w-[18px] h-[18px]" />
            </div>
          </div>
        )}
        {status === ImageStatus.Loading && (
          <div className="absolute inset-0 flex items-center rounded-[6px] justify-center bg-[rgba(0,0,0,0.4)] z-10">
            <Loading loading color="blue" />
          </div>
        )}
      </div>
      {status === ImageStatus.Error && (
        <div className="text-red-500">{I18n.t('upload_fail')}</div>
      )}
    </div>
  );
};
