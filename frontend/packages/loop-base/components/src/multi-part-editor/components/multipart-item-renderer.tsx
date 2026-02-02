// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { ContentType } from '@cozeloop/api-schema/evaluation';
import { IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { Button, TextArea } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { type MultipartItem } from '../type';
import { VideoItemRenderer } from './video-item-renderer';
import { ImageItemRenderer } from './image-item-renderer';

interface MultipartItemRendererProps {
  item: MultipartItem;
  readonly?: boolean;
  onChange: (item: MultipartItem) => void;
  onRemove: () => void;
}

export const MultipartItemRenderer: React.FC<MultipartItemRendererProps> = ({
  item,
  readonly,
  onChange,
  onRemove,
}) => {
  const I18n = useI18n();
  const handleTextChange = (text: string) => {
    onChange({
      ...item,
      text,
    });
  };

  switch (item.content_type) {
    case ContentType.Text:
      return (
        <div className="flex items-center gap-1">
          <TextArea
            value={item.text}
            onChange={handleTextChange}
            autosize={{ minRows: 1, maxRows: 3 }}
            disabled={readonly}
            placeholder={I18n.t('please_input_text')}
          />

          {readonly ? null : (
            <Button
              icon={<IconCozTrashCan />}
              color="secondary"
              size="small"
              onClick={onRemove}
            />
          )}
        </div>
      );

    case ContentType.Image:
      return (
        <ImageItemRenderer
          item={item}
          onRemove={onRemove}
          onChange={onChange}
          readonly={readonly}
        />
      );

    case 'Video':
      return (
        <VideoItemRenderer
          item={item}
          onRemove={onRemove}
          onChange={onChange}
          readonly={readonly}
        />
      );

    default:
      return null;
  }
};
