import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { DataApi } from '@cozeloop/api-schema';
import { logger } from '@coze-arch/logger';

import { MAX_TAG_NAME_LENGTH } from '@/const';

export type ValidateFn = (name: string) => Promise<string> | string;

export const tagNameValidate: ValidateFn = (name: string) => {
  const reg = new RegExp(/^[\u4e00-\u9fa5_a-zA-Z0-9]+$/);

  if (!name || name.length <= 0 || name.length > MAX_TAG_NAME_LENGTH) {
    return '标签名称必须为 1～50 字符长度';
  }

  if (!reg.test(name)) {
    return '标签名称仅支持输入中文、英文、数字和下划线';
  }

  return '';
};

export const tagEmptyValueValidate: ValidateFn = (
  value?: string | number,
) => {
  console.log('tagEmptyValueValidate', { value });
  if (!value || value.toString().trim() === '') {
    return '标签值不能为空';
  }

  return '';
};

export const tagLengthMaxLengthValidate: ValidateFn = (value: string) => {
  if (value && value.length > 200) {
    return '标签值长度不能超过 200 个字符';
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
        ? '同空间内标签名称不允许重复'
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
      return '一个标签内的标签值不允许重复';
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