// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Result } from 'ahooks/lib/useRequest/src/types';
import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type CreateTagResponse, tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';

import { type FormValues } from '@/components/tags-form';

const { TagContentType, TagStatus } = tag;

const formatFormValuesToCreateTagParams: (values: FormValues) => unknown = (
  values: FormValues,
) => {
  if (
    values.content_type === TagContentType.FreeText ||
    values.content_type === TagContentType.ContinuousNumber
  ) {
    return {
      tag_key_name: values.tag_key_name,
      description: values.description,
      tag_content_type: values.content_type,
    };
  }

  if (values.content_type === TagContentType.Categorical) {
    return {
      tag_key_name: values.tag_key_name,
      description: values.description,
      tag_content_type: values.content_type,
      tag_values: values.tag_values.map(tagValue => {
        const { tag_status, ...rest } = tagValue;
        return {
          ...rest,
          status: TagStatus.Active,
        };
      }),
    };
  }

  return {
    tag_key_name: values.tag_key_name,
    description: values.description,
    tag_content_type: values.content_type,
    tag_content_spec: values.content_spec,
    tag_values: values.tag_values,
  };
};
export const useCreateTag = (): Result<CreateTagResponse, [FormValues]> => {
  const { spaceID } = useSpace();
  const service = useRequest(
    async (values: FormValues) => {
      const createTagParams = formatFormValuesToCreateTagParams(values) as Omit<
        Parameters<typeof DataApi.CreateTag>[0],
        'workspace_id'
      >;

      const result = await DataApi.CreateTag({
        workspace_id: spaceID,
        ...createTagParams,
      });
      return result;
    },
    {
      manual: true,
    },
  );
  return service;
};
