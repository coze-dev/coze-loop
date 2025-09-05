import React from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { Input, Button } from '@coze-arch/coze-design';

import { type MultipartItem } from '../../type';

interface TextItemRendererProps {
  item: MultipartItem;
  onChange: (text: string) => void;
  onRemove: () => void;
  readonly?: boolean;
}

export const TextItemRenderer: React.FC<TextItemRendererProps> = ({
  item,
  onChange,
  onRemove,
  readonly = false,
}) => (
  <div className="flex items-center gap-2 mb-2 group">
    <div className="flex-1">
      <Input
        value={item.text || ''}
        onChange={onChange}
        placeholder={I18n.t('cozeloop_open_evaluate_enter_text_content')}
        disabled={readonly}
      />
    </div>
    {!readonly && (
      <Button
        icon={<IconCozTrashCan />}
        color="secondary"
        size="small"
        className="invisible group-hover:visible"
        onClick={onRemove}
      />
    )}
  </div>
);
