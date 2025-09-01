import React, { useState } from 'react';

import cs from 'classnames';
import { FieldDisplayFormat } from '@cozeloop/api-schema/data';
import {
  IconCozArrowDown,
  IconCozArrowRight,
  IconCozText,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import { Button, Collapse, Divider, Typography } from '@coze-arch/coze-design';

import { StringDatasetItem } from '@/components/dataset-item/text/string';
import { ChipSelect } from '@/components/common/chip-select';

import {
  type Content,
  DataType,
  DISPLAY_FORMAT_MAP,
  DISPLAY_TYPE_MAP,
  type MultipartItem,
} from '../../../type';

import styles from './index.module.less';
export const ICON_ID = 'text-item-renderer-icon';
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
}) => {
  const [activeKey, setActiveKey] = useState<string[]>(['1']);
  const [format, setFormat] = useState<FieldDisplayFormat>(
    FieldDisplayFormat.PlainText,
  );
  const getHeader = () => (
    <div className="flex items-center gap-2 w-full">
      <div className="flex-1 flex gap-1 items-center">
        <Button
          icon={
            activeKey.length === 0 ? (
              <IconCozArrowRight className="w-[14px] h-[14px]" />
            ) : (
              <IconCozArrowDown className="w-[14px] h-[14px]" />
            )
          }
          color="secondary"
          size="mini"
          className="coz-fg-secondary"
          onClick={() => setActiveKey(activeKey.length === 0 ? ['1'] : [])}
        />
        <IconCozText
          className={cs(
            ICON_ID,
            'coz-fg-secondary w-[14px] h-[14px]',
            activeKey.length === 0 ? 'visible' : 'invisible',
          )}
        />
      </div>
      <div className="flex items-center gap-[2px]">
        <ChipSelect
          chipRender="selectedItem"
          value={format}
          size="small"
          onChange={newFormat => {
            setFormat(newFormat as FieldDisplayFormat);
          }}
          optionList={DISPLAY_TYPE_MAP[DataType.String].map(type => ({
            label: (
              <Typography.Text className="!coz-fg-primary !text-[12px]">
                {DISPLAY_FORMAT_MAP[type]}
              </Typography.Text>
            ),
            value: type,
            chipColor: 'secondary',
          }))}
        ></ChipSelect>
        <Divider layout="vertical" className="h-[12px] coz-fg-secondary" />
        <Button
          icon={
            <IconCozTrashCan className="w-[14px] h-[14px] coz-fg-primary" />
          }
          color="secondary"
          size="mini"
          onClick={onRemove}
        />
      </div>
    </div>
  );
  const onContentChange = (content?: Content) => {
    onChange(content?.text || '');
  };
  return (
    <Collapse
      activeKey={activeKey}
      className={cs(styles.collapse, 'group')}
      defaultActiveKey={['1']}
      clickHeaderToExpand={false}
    >
      <Collapse.Panel itemKey="1" header={getHeader()} showArrow={false}>
        <StringDatasetItem
          isEdit
          fieldContent={{
            format,
            text: item.text,
          }}
          className="!rounded-t-none"
          onChange={onContentChange}
          displayFormat={true}
        />
      </Collapse.Panel>
    </Collapse>
  );
};
