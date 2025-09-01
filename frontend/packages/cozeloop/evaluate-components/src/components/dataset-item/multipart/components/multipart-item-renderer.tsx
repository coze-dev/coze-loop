import React from 'react';

import { ContentType } from '@cozeloop/api-schema/evaluation';

import { type MultipartItem } from '../../type';
import { TextItemRenderer } from './text-item-render';
import { ImageItemRenderer } from './image-item-renderer';

interface MultipartItemRendererProps {
  item: MultipartItem;
  onChange: (item: MultipartItem) => void;
  onRemove: () => void;
}

export const MultipartItemRenderer: React.FC<MultipartItemRendererProps> = ({
  item,
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
        <TextItemRenderer
          item={item}
          onChange={handleTextChange}
          onRemove={onRemove}
        />
      );
    case ContentType.Image:
      return (
        <ImageItemRenderer
          item={item}
          onRemove={onRemove}
          onChange={onChange}
        />
      );
    default:
      return null;
  }
};
