/* eslint-disable prefer-destructuring */
import { useRequest } from 'ahooks';
import {
  type PlatformType,
  type SpanListType,
} from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';
import { Toast } from '@coze-arch/coze-design';

import { i18nService } from '@/i18n';

export interface UseGetMetaInfoParams {
  selectedPlatform: string | number | undefined;
  selectedSpanType: string | number | undefined;
  spaceID: string;
}

export interface Result {
  msg: string;
  code: number;
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

    const code = (result as Result).code;
    const msg = (result as Result).msg;

    if (code === 0) {
      return result.field_metas || {};
    } else {
      Toast.error(
        i18nService.t('observation_fetch_meta_error', {
          msg: msg || '',
        }),
      );
      return {};
    }
  } catch (e) {
    Toast.error(
      i18nService.t('observation_fetch_meta_error', {
        msg: (e as unknown as { message: string }).message || '',
      }),
    );
  }
}

export const useGetMetaInfo = ({
  selectedPlatform,
  selectedSpanType,
  spaceID,
}: UseGetMetaInfoParams) => {
  const { data: metaInfo, loading } = useRequest(
    () => {
      if (!selectedPlatform || !selectedSpanType) {
        return Promise.resolve(undefined);
      }
      return fetchMetaInfo({
        selectedPlatform,
        selectedSpanType,
        spaceID,
      });
    },
    {
      refreshDeps: [selectedPlatform, selectedSpanType],
    },
  );

  return {
    metaInfo,
    loading,
  };
};
