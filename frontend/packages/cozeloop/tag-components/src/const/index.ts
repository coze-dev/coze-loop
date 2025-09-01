import { tag } from '@cozeloop/api-schema/data';

const { TagContentType } = tag;
export enum TagType {
  Number = 'number',
  Text = 'text',
  Category = 'category',
  Boolean = 'boolean',
}

export const MAX_TAG_LENGTH = 50;
export const MAX_TAG_NAME_LENGTH = 50;
export const MAX_TAG_DESC_LENGTH = 200;

export const TAG_TYPE_TO_NAME_MAP = {
  [TagContentType.Categorical]: '分类',
  [TagContentType.Boolean]: '布尔值',
  [TagContentType.ContinuousNumber]: '数字',
  [TagContentType.FreeText]: '文本',
};

export const TAG_TYPE_OPTIONS = [
  {
    label: TAG_TYPE_TO_NAME_MAP[TagContentType.Categorical],
    value: TagContentType.Categorical,
  },
  {
    label: TAG_TYPE_TO_NAME_MAP[TagContentType.Boolean],
    value: TagContentType.Boolean,
  },
  {
    label: TAG_TYPE_TO_NAME_MAP[TagContentType.ContinuousNumber],
    value: TagContentType.ContinuousNumber,
  },
  {
    label: TAG_TYPE_TO_NAME_MAP[TagContentType.FreeText],
    value: TagContentType.FreeText,
  },
];
