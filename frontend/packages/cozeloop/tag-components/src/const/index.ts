// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
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
  [TagContentType.Categorical]: I18n.t('category'),
  [TagContentType.Boolean]: I18n.t('boolean'),
  [TagContentType.ContinuousNumber]: I18n.t('number'),
  [TagContentType.FreeText]: I18n.t('text'),
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
