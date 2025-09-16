// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect } from 'react';

import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type PlatformType,
  type SpanListType,
} from '@cozeloop/api-schema/observation';

import { useTraceStore } from '../stores/trace';
import { useGetMetaInfo } from './use-get-meta-info';

export const useFetchMetaInfo = () => {
  const { selectedPlatform, selectedSpanType, setFieldMetas } = useTraceStore();
  const { spaceID } = useSpace();
  const { metaInfo, loading } = useGetMetaInfo({
    selectedPlatform: selectedPlatform as PlatformType,
    selectedSpanType: selectedSpanType as SpanListType,
    spaceID,
  });

  useEffect(() => {
    if (!loading) {
      setFieldMetas(metaInfo);
    }
  }, [loading, metaInfo, setFieldMetas]);
};
