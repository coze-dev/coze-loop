// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode } from 'react';

import { JsonViewer } from '@textea/json-viewer';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type FieldData,
  type Content,
  type FieldSchema,
  ContentType,
} from '@cozeloop/api-schema/evaluation';
import {
  FieldDisplayFormat,
  type MultiModalSpec,
} from '@cozeloop/api-schema/data';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Popover } from '@coze-arch/coze-design';

import { jsonViewerConfig } from './text/string/json/config';

export type { Content };

export enum InputType {
  Form = 'Form',
  JSON = 'JSON',
}
export interface DatasetItemBaseProps {
  fieldSchema?: FieldSchema;
  isEdit?: boolean;
  /**
   * 是否显示列名
   */
  showColumnKey?: boolean;
  /**
   * 是否展开
   */
  expand?: boolean;
  /**
   * 是否显示md,code,json的渲染格式
   */
  displayFormat?: boolean;

  /**
   * className
   */
  className?: string;

  /**
   * 是否显示空状态图标，默认不展示。
   */
  showEmpty?: boolean;

  /**
   * 数据集ID
   */
  datasetID?: string;

  /**
   * multipart config
   */
  multipartConfig?: MultiModalSpec;
}

export interface DatasetFieldItemRenderProps extends DatasetItemBaseProps {
  fieldData?: FieldData;
  onChange?: (fieldData: FieldData) => void;
}
export interface DatasetItemProps extends DatasetItemBaseProps {
  className?: string;
  containerClassName?: string;
  fieldContent?: Content;
  onChange?: (content: Content) => void;
}

export enum ImageStatus {
  Loading = 'loading',
  Success = 'success',
  Error = 'error',
}
export interface MultipartItem extends Content {
  uid: string;
  sourceImage?: {
    status: ImageStatus;
    file?: File;
  };
}

export enum DataType {
  String = 'string',
  Integer = 'integer',
  Float = 'number',
  Boolean = 'boolean',
  Object = 'object',
  ArrayString = 'array<string>',
  ArrayInteger = 'array<integer>',
  ArrayFloat = 'array<number>',
  ArrayBoolean = 'array<boolean>',
  ArrayObject = 'array<object>',
  Image = 'Image',
  MultiPart = 'MultiPart',
}

export const dataTypeMap = {
  [DataType.String]: 'String',
  [DataType.Integer]: 'Integer',
  [DataType.Float]: 'Float',
  [DataType.Boolean]: 'Boolean',
  [DataType.Object]: 'Object',
  [DataType.ArrayString]: 'Array<String>',
  [DataType.ArrayInteger]: 'Array<Integer>',
  [DataType.ArrayFloat]: 'Array<Float>',
  [DataType.ArrayBoolean]: 'Array<Boolean>',
  [DataType.ArrayObject]: 'Array<Object>',
  // pe 转换
  placeholder: 'String',
  [DataType.Image]: 'Image',
  [DataType.MultiPart]: I18n.t('multimodal'),
};

export const contentTypeToDataType = {
  [ContentType.Text]: DataType.String,
  [ContentType.Audio]: DataType.MultiPart,
  [ContentType.Image]: DataType.Image,
  [ContentType.MultiPart]: DataType.MultiPart,
};

export const dataTypeToContentType = {
  [DataType.MultiPart]: ContentType.MultiPart,
  [DataType.Image]: ContentType.Image,
};

export const displayFormatType = {
  [FieldDisplayFormat.PlainText]: 'PlainText',
  [FieldDisplayFormat.Code]: 'Code',
  [FieldDisplayFormat.JSON]: 'JSON',
  [FieldDisplayFormat.Markdown]: 'Markdown',
};

export { ContentType };

export const COLUMN_TYPE_MAP = {
  [ContentType.Text]: 'Text',
  [ContentType.Audio]: 'Audio',
  [ContentType.Image]: 'Image',
  [ContentType.MultiPart]: 'MultiPart',
};

export const DATA_TYPE_LIST = [
  {
    label: 'String',
    value: DataType.String,
  },
  {
    label: 'Integer',
    value: DataType.Integer,
  },
  {
    label: 'Float',
    value: DataType.Float,
  },
  {
    label: 'Boolean',
    value: DataType.Boolean,
  },
  {
    label: 'Object',
    value: DataType.Object,
  },
];

const ARRAY_TYPE_LIST = {
  label: 'Array',
  value: 'array',
  children: [
    {
      label: 'String',
      value: DataType.ArrayString,
    },
    {
      label: 'Integer',
      value: DataType.ArrayInteger,
    },
    {
      label: 'Float',
      value: DataType.ArrayFloat,
    },
    {
      label: 'Boolean',
      value: DataType.ArrayBoolean,
    },
    {
      label: 'Object',
      value: DataType.ArrayObject,
    },
  ],
};

export const TEMPLATE_MULTIPART_DATA = [
  {
    type: 'text',
    text: 'You are an assistant',
  },
  {
    type: 'image_url',
    image_url: {
      url: '',
    },
  },
];

export const MUTABLE_DATA_TYPE_LIST = [
  {
    label: (
      <div className="flex items-center gap-2">
        {I18n.t('multimodal')}
        <Popover
          content={
            <div className="w-[320px] py-2 px-3">
              <div>
                {I18n.t(
                  'cozeloop_open_evaluate_multi_modal_data_in_cell_usage',
                )}
              </div>
              <div>
                {I18n.t(
                  'cozeloop_open_evaluate_array_object_data_sample_multimodal',
                )}
              </div>
              <div className="whitespace-pre-wrap mt-2 p-2 coz-bg-plus border border-solid border-[var(--coz-stroke-primary)] rounded-[6px]">
                <JsonViewer
                  {...jsonViewerConfig}
                  value={TEMPLATE_MULTIPART_DATA}
                />
              </div>
            </div>
          }
        >
          <IconCozInfoCircle className="coz-fg-secondary cursor-pointer hover:coz-fg-primary" />
        </Popover>
      </div>
    ),

    value: DataType.MultiPart,
  },
];

export const DATA_TYPE_LIST_WITH_ARRAY = [...DATA_TYPE_LIST, ARRAY_TYPE_LIST];
export const MUTIPART_DATA_TYPE_LIST_WITH_ARRAY = [
  ...DATA_TYPE_LIST_WITH_ARRAY,
  ...MUTABLE_DATA_TYPE_LIST,
];

export const getDataTypeListWithArray = (
  disableObj: boolean,
  renderDisableLabel: (label: string) => ReactNode,
  disableMultiPart?: boolean,
) =>
  MUTIPART_DATA_TYPE_LIST_WITH_ARRAY?.map(item => {
    if (disableObj) {
      if (item.value === DataType.Object) {
        return {
          ...item,
          label: renderDisableLabel(item.label as string),
          disabled: true,
        };
      }
      if (disableMultiPart) {
        if (item.value === DataType.MultiPart) {
          return {
            ...item,
            label: renderDisableLabel(item.label as string),
            disabled: true,
          };
        }
      }
      if (item.value === 'array' && item?.children) {
        return {
          ...item,
          children: item?.children?.map(child => {
            if (child.value === DataType.ArrayObject) {
              return {
                ...child,
                label: renderDisableLabel(child.label),
                disabled: true,
              };
            }
            return child;
          }),
        };
      }
    }

    return item;
  });

export const DISPLAY_TYPE_MAP = {
  [DataType.String]: [
    FieldDisplayFormat.PlainText,
    FieldDisplayFormat.Code,
    FieldDisplayFormat.JSON,
    FieldDisplayFormat.Markdown,
  ],

  [DataType.MultiPart]: [
    FieldDisplayFormat.PlainText,
    FieldDisplayFormat.Code,
    FieldDisplayFormat.JSON,
    FieldDisplayFormat.Markdown,
  ],

  [DataType.Integer]: [FieldDisplayFormat.PlainText],
  [DataType.Float]: [FieldDisplayFormat.PlainText],
  [DataType.Boolean]: [FieldDisplayFormat.PlainText],
  [DataType.Object]: [FieldDisplayFormat.Code],
  [DataType.ArrayString]: [FieldDisplayFormat.Code],
  [DataType.ArrayInteger]: [FieldDisplayFormat.Code],
  [DataType.ArrayFloat]: [FieldDisplayFormat.Code],
  [DataType.ArrayBoolean]: [FieldDisplayFormat.Code],
  [DataType.ArrayObject]: [FieldDisplayFormat.Code],
};

export const DISPLAY_FORMAT_MAP = {
  [FieldDisplayFormat.PlainText]: 'PlainText',
  [FieldDisplayFormat.Code]: 'Code',
  [FieldDisplayFormat.JSON]: 'JSON',
  [FieldDisplayFormat.Markdown]: 'Markdown',
};

export interface FieldObjectSchema {
  key: string;
  type?: DataType;
  // 变量名称
  propertyKey?: string;
  isRequired?: boolean;
  // object对象的子数据
  children?: FieldObjectSchema[];
  additionalProperties?: boolean;
}
export interface ConvertFieldSchema extends FieldSchema {
  type?: DataType;
  children?: FieldObjectSchema[];
  schema?: string;
  isRequired?: boolean;
  additionalProperties?: boolean;
  inputType?: InputType;
}

export const DEFAULT_FILE_SIZE = 20 * 1024 * 1024;
export const DEFAULT_FILE_COUNT = 20;
export const DEFAULT_PART_COUNT = 50;
export const DEFAULT_SUPPORTED_FORMATS = [
  '.jpg',
  '.jpeg',
  '.png',
  '.gif',
  '.bmp',
  '.webp',
];
