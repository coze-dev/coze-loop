// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type PlatformType,
  type SpanListType,
} from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';
import { Toast } from '@coze-arch/coze-design';

interface UseGetMetaInfoParams {
  selectedPlatform: string | number;
  selectedSpanType: string | number;
  spaceID: string;
}

export async function fetchMetaInfo({
  selectedPlatform,
  selectedSpanType,
  spaceID,
}: UseGetMetaInfoParams) {
  try {
    const result = await observabilityTrace.GetTracesMetaInfo(
      {
        platform_type: selectedPlatform as PlatformType,
        span_list_type: selectedSpanType as SpanListType,
        workspace_id: spaceID,
      },
      {
        __disableErrorToast: true,
      },
    );

    const { msg, code } = result as unknown as { msg: string; code: number };

    if (code === 0) {
      return result.field_metas || {};
    } else {
      Toast.error(
        I18n.t('analytics_fetch_meta_error', {
          msg: msg || '',
        }),
      );
      return {};
    }
  } catch (e) {
    Toast.error(
      I18n.t('analytics_fetch_meta_error', {
        msg: (e as unknown as { message: string }).message || '',
      }),
    );
  }
}

export const useGetMetaInfo = ({
  selectedPlatform,
  selectedSpanType,
}: UseGetMetaInfoParams) => {
  const { spaceID } = useSpace();
  const { data: metaInfo, loading } = useRequest(
    () =>
      fetchMetaInfo({
        selectedPlatform,
        selectedSpanType,
        spaceID,
      }),
    {
      refreshDeps: [selectedPlatform, selectedSpanType],
    },
  );

  return {
    metaInfo,
    loading,
  };
};
