import classNames from 'classnames';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { IconCozEqual } from '@coze-arch/coze-design/icons';
import { Tag, type TooltipProps, Typography } from '@coze-arch/coze-design';

import { getColumnType } from '../dataset-item/util';

export enum PeDataType {
  String = 'string',
  Integer = 'integer',
  Number = 'number',
  Float = 'float',
  Boolean = 'boolean',
  Object = 'object',
  List = 'list',
  ArrayString = 'array<string>',
  ArrayInteger = 'array<integer>',
  ArrayFloat = 'array<float>',
  ArrayNumber = 'array<number>',
  ArrayBoolean = 'array<boolean>',
  ArrayObject = 'array<object>',
  Placeholder = 'placeholder',
  Multipart = 'MultiPart',
  Image = 'Image',
}

export const peDataTypeMap = {
  [PeDataType.String]: 'String',
  [PeDataType.Placeholder]: 'String',
  [PeDataType.Boolean]: 'Boolean',
  [PeDataType.Object]: 'Object',
  [PeDataType.ArrayString]: 'Array<String>',
  [PeDataType.ArrayInteger]: 'Array<Integer>',
  [PeDataType.ArrayFloat]: 'Array<Float>',
  [PeDataType.ArrayNumber]: 'Array<Float>',
  [PeDataType.ArrayBoolean]: 'Array<Boolean>',
  [PeDataType.ArrayObject]: 'Array<Object>',
  [PeDataType.Integer]: 'Integer',
  [PeDataType.Number]: 'Float',
  [PeDataType.Float]: 'Float',
  // 多模态
  [PeDataType.Image]: 'Image',
  [PeDataType.Multipart]: '多模态',
};

export function ReadonlyItem({
  title,
  value,
  typeText,
  className,
  showType = true,
  tooltipProps,
  isRequired = false,
  titleClassName,
}: {
  title?: string;
  value?: React.ReactNode;
  typeText?: string;
  className?: string;
  showType?: boolean;
  tooltipProps?: Omit<TooltipProps, 'showArrow'>;
  isRequired?: boolean;
  titleClassName?: string;
}) {
  return (
    <div
      className={classNames(
        'flex flex-row items-center h-8 gap-[6px] border border-solid coz-stroke-plus rounded-[6px] text-sm font-normal',
        className,
      )}
    >
      <div
        className={classNames(
          'flex-shrink-0 coz-fg-secondary ml-[10px]',
          titleClassName,
        )}
      >
        {title}
      </div>
      {isRequired ? (
        <div className="text-[#E53241] text-center text-sm font-medium leading-5 absolute left-[54px]">
          *
        </div>
      ) : null}
      <Typography.Text
        className="flex-1 !coz-fg-primary overflow-hidden"
        ellipsis={{
          showTooltip: {
            opts: {
              theme: 'dark',
              ...(tooltipProps ?? {}),
            },
          },
        }}
      >
        {value}
      </Typography.Text>
      {showType && typeText ? (
        <Tag
          className="flex-shrink-0 mr-[10px] font-semibold leading-[14px] text-[10px] text-[var(--coz-fg-secondary)]"
          size="mini"
          color="primary"
        >
          {typeText}
        </Tag>
      ) : null}
    </div>
  );
}

export function EqualItem() {
  return (
    <div className="w-8 h-8 border border-solid coz-stroke-plus rounded-[6px] coz-fg-primary flex items-center justify-center shrink-0">
      <IconCozEqual className="w-4 h-4 coz-fg-primary" />
    </div>
  );
}

// 使用最开始的数据集的模式，从 text_schema 解析
export function getTypeText(item?: FieldSchema & { type?: string }) {
  // 兼容 type 字段
  const type = getColumnType(item);
  return peDataTypeMap[type as unknown as keyof typeof peDataTypeMap];
}

// 直接使用type类型
export function getSchemaTypeText(item?: FieldSchema & { type?: string }) {
  const type = item?.type || getColumnType(item);
  return peDataTypeMap[type as keyof typeof peDataTypeMap];
}

export interface GetInputTypeTextParams {
  type?: string;
  content_type?: string;
  text_schema: {
    items?: { type: string };
    type: string;
  };
}

// array<string>
export function getInputTypeText(item?: GetInputTypeTextParams) {
  const itemType = item?.content_type || item?.type;
  const targetType =
    itemType === 'array'
      ? `array<${item?.text_schema?.items?.type}>`
      : itemType;

  if (targetType?.toLocaleLowerCase() === 'float') {
    return 'Float';
  }

  if (targetType?.toLocaleLowerCase() === 'array<float>') {
    return 'Array<Float>';
  }

  return (
    peDataTypeMap[targetType as keyof typeof peDataTypeMap] ||
    getTypeText(item as unknown as FieldSchema & { type?: string })
  );
}
