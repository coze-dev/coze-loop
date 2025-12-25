// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import classNames from 'classnames';
import { ContentType } from '@cozeloop/api-schema/prompt';

import { type ContentPartLoop, type FileItemStatus, ImageStatus } from './type';
import { VideoItemRenderer } from './components/video-item-renderer';
import { ImageItemRenderer } from './components/image-item-renderer';

interface MultiPartRenderProps {
  fileParts?: ContentPartLoop[];
  readonly?: boolean;
  className?: string;
  onDeleteFilePart?: (part: ContentPartLoop) => void;
  onFilePartsChange?: (part: ContentPartLoop) => void;
}

const convertStatus = (status: FileItemStatus) => {
  if (status === 'uploading') {
    return ImageStatus.Loading;
  }
  if (status === 'success') {
    return ImageStatus.Success;
  }

  return ImageStatus.Error;
};

export function MultiPartRender({
  fileParts,
  onDeleteFilePart,
  onFilePartsChange,
  readonly,
  className,
}: MultiPartRenderProps) {
  return (
    <div
      className={classNames(
        'flex gap-2 flex-wrap pt-0 pb-3 px-2 w-full border border-solid border-t-0 border-l-0 border-r-0 border-b-[rgba(68,83,130,.25)]',
        className,
      )}
    >
      {fileParts?.map(part =>
        part.type === ContentType.VideoURL ? (
          <VideoItemRenderer
            className="!w-[45px] !h-[45px]"
            item={{
              content_type: 'Video',
              uid: part.uid,
              video: part.video_url,
              sourceVideo: {
                status: convertStatus(part.status || 'success'),
              },
            }}
            onRemove={() => onDeleteFilePart?.(part)}
            onChange={file =>
              onFilePartsChange?.({
                ...file,
                type: ContentType.VideoURL,
              })
            }
            readonly={readonly}
          />
        ) : (
          <ImageItemRenderer
            className="!w-[45px] !h-[45px]"
            item={{
              content_type: 'Image' as any,
              uid: part.uid,
              image: part.image_url,
              sourceImage: {
                status: convertStatus(part.status || 'success'),
              },
            }}
            onRemove={() => onDeleteFilePart?.(part)}
            onChange={file =>
              onFilePartsChange?.({
                ...file,
                type: ContentType.ImageURL,
              })
            }
            readonly={readonly}
          />
        ),
      )}
    </div>
  );
}
