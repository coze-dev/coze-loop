// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { DataApi } from '@cozeloop/api-schema';
import { logger } from '@coze-arch/logger';

import { MAX_TAG_NAME_LENGTH } from '@/const';

export type ValidateFn = (name: string) => Promise<string> | string;

export const tagNameValidate: ValidateFn = (name: string) => {
  const reg = new RegExp(/^[\u4e00-\u9fa5_a-zA-Z0-9]+$/);

  if (!name || name.length <= 0 || name.length > MAX_TAG_NAME_LENGTH) {
    return I18n.t('tag_name_length_limit');
  }

  if (!reg.test(name)) {
    return I18n.t('tag_name_valid_chars');
  }

  return '';
};

export const tagEmptyValueValidate: ValidateFn = (value?: string | number) => {
  console.log('tagEmptyValueValidate', { value });
  if (!value || value.toString().trim() === '') {
    return I18n.t('tag_value_not_empty');
  }

  return '';
};

export const tagLengthMaxLengthValidate: ValidateFn = (value: string) => {
  if (value && value.length > 200) {
    return I18n.t('tag_value_length_limit');
  }

  return '';
};

export const useTagNameValidateUniqBySpace = (tagKeyId?: string) => {
  const { spaceID } = useSpace();

  return async (name: string): Promise<string> => {
    try {
      const { tagInfos } = await DataApi.SearchTags({
        workspace_id: spaceID,
        tag_key_name: name,
      });

      return tagInfos &&
        tagInfos?.length > 0 &&
        tagInfos.findIndex(item => item.tag_key_id === tagKeyId) === -1
        ? I18n.t('tag_name_no_duplicate_space')
        : '';
    } catch (error) {
      logger.error({
        error: error as Error,
        eventName: 'useTagNameValidateUniqBySpace',
      });
      return '';
    }
  };
};

export const tagValidateNameUniqByOptions = (
  options: string[],
  index: number,
) =>
  ((name: string) => {
    if (options.includes(name) && options.indexOf(name) !== index) {
      return I18n.t('tag_value_no_duplicate');
    }
    return '';
  }) as ValidateFn;

export const composeValidate = (fns: ValidateFn[]) => async (value: string) => {
  for (const fn of fns) {
    const result = await fn(value);
    if (result) {
      return result;
    }
  }
  return '';
};
