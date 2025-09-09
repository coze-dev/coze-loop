// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { type Version } from '@cozeloop/components';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { Tag } from '@coze-arch/coze-design';

import { DRAFT_VERSION } from '../dataset-detail/table/use-dataset-item-list';

export interface DatasetVersionTagProps {
  currentVersion?: Version;
  datasetDetail?: EvaluationSet;
}

export const DatasetVersionTag = ({
  currentVersion,
  datasetDetail,
}: DatasetVersionTagProps) => {
  if (currentVersion?.id && currentVersion?.id !== DRAFT_VERSION) {
    return (
      <Tag color="primary" className="font-normal">
        {currentVersion.version}
      </Tag>
    );
  }
  return datasetDetail?.change_uncommitted ? (
    <Tag color="yellow" className="font-normal">
      {I18n.t('changes_not_submitted')}
    </Tag>
  ) : (
    <Tag color="primary" className="font-normal">
      {I18n.t('draft_version')}
    </Tag>
  );
};
