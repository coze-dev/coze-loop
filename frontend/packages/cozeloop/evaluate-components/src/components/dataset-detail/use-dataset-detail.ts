// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useParams } from 'react-router-dom';
import { useState } from 'react';

import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

export const useFetchDatasetDetail = () => {
  const { id } = useParams();
  const { spaceID } = useSpace();
  const [loading, setLoading] = useState(true);
  const { data, run } = useRequest(
    async () => {
      const res = await StoneEvaluationApi.GetEvaluationSet({
        workspace_id: spaceID,
        evaluation_set_id: id as string,
      });
      return res.evaluation_set;
    },
    {
      onFinally: () => {
        setLoading(false);
      },
    },
  );
  return {
    datasetDetail: data,
    refreshDataset: run,
    loading,
  };
};
