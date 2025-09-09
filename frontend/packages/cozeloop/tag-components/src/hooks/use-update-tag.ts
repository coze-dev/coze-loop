// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Result } from 'ahooks/lib/useRequest/src/types';
import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type UpdateTagResponse,
  type UpdateTagRequest,
} from '@cozeloop/api-schema/data';
import { tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';

import { type FormValues } from '@/components/tags-form';
const { TagStatus } = tag;

const formatFormValuesToUpdateTagParams = (params: FormValues) => {
  const transformParams: Omit<UpdateTagRequest, 'workspace_id'> = {
    tag_key_name: params.tag_key_name ?? '',
    tag_key_id: params.tag_key_id ?? '',
    tag_values: params.tag_values?.map(item => {
      const { tag_status, ...rest } = item;

      item.tag_value_id;
      return {
        ...rest,
        status: item.tag_value_id
          ? tag_status
            ? TagStatus.Active
            : TagStatus.Inactive
          : TagStatus.Active,
      };
    }),
    tag_content_type: params.content_type,
    description: params.description,
    tag_content_spec: params.content_spec,
    version: params.version,
  };

  return transformParams;
};

export const useUpdateTag: () => Result<
  UpdateTagResponse,
  [FormValues]
> = () => {
  const { spaceID } = useSpace();
  const service = useRequest(
    async (params: FormValues) => {
      const result = await DataApi.UpdateTag({
        workspace_id: spaceID,
        ...formatFormValuesToUpdateTagParams(params),
      });
      return result;
    },
    {
      manual: true,
    },
  );

  return service;
};
