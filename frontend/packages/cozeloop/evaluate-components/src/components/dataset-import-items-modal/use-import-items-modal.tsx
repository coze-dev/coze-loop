// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';

import { DatasetImportItemsModal } from '.';

export const useImportItemsModal = (
  datasetDetail: EvaluationSet | undefined,
  onRefresh: () => void,
) => {
  const [visible, setVisible] = useState(false);
  const onSave = () => {
    setVisible(false);
    onRefresh();
  };
  const node = visible ? (
    <DatasetImportItemsModal
      onOk={onSave}
      datasetDetail={datasetDetail}
      onCancel={() => setVisible(false)}
    />
  ) : null;
  return {
    visible,
    setVisible,
    modalNode: node,
  };
};
