import { useEffect } from 'react';

import {
  type FieldMeta,
  type PlatformType,
  type SpanListType,
} from '@cozeloop/api-schema/observation';

import { useTraceStore } from '@/features/trace-list/stores/trace';

import { useGetMetaInfo } from './use-get-meta-info';

interface UseFetchMetaInfoProps {
  selectedPlatform: PlatformType;
  selectedSpanType: SpanListType;
}

export const useFetchMetaInfo = ({
  selectedPlatform,
  selectedSpanType,
}: UseFetchMetaInfoProps) => {
  const { setFieldMetas, customParams } = useTraceStore();
  const { metaInfo, loading } = useGetMetaInfo({
    selectedPlatform: selectedPlatform as PlatformType,
    selectedSpanType: selectedSpanType as SpanListType,
    spaceID: customParams?.spaceID ?? '',
  });

  useEffect(() => {
    if (!loading) {
      setFieldMetas(metaInfo as unknown as Record<string, FieldMeta>);
    }
  }, [loading, metaInfo, setFieldMetas]);
};
