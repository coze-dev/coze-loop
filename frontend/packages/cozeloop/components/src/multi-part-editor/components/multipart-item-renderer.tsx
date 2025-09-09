// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { type Content, ContentType } from '@cozeloop/api-schema/evaluation';
import { IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { Button, TextArea } from '@coze-arch/coze-design';

import { ImageItemRenderer } from './image-item-renderer';

interface MultipartItemRendererProps {
  item: Content;
  readonly?: boolean;
  onChange: (item: Content) => void;
  onRemove: () => void;
}

export const MultipartItemRenderer: React.FC<MultipartItemRendererProps> = ({
  item,
  readonly,
  onChange,
  onRemove,
}) => {
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
            placeholder="请输入文本信息"
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
    default:
      return null;
  }
};
