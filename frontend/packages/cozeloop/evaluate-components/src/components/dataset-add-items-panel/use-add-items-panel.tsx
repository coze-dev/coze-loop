// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';

import { DatasetAddItemsPanel } from '.';

export const useAddItemsPanel = (
  datasetDetail: EvaluationSet | undefined,
  onRefresh: () => void,
) => {
  const [visible, setVisible] = useState(false);
  const onOK = () => {
    setVisible(false);
    onRefresh();
  };

  const node = visible ? (
    <DatasetAddItemsPanel
      onOK={onOK}
      datasetDetail={datasetDetail}
      onCancel={() => setVisible(false)}
    />
  ) : null;
  return {
    visible,
    setVisible,
    panelNode: node,
  };
};
