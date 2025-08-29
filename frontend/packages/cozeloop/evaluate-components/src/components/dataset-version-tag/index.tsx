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
      修改未提交
    </Tag>
  ) : (
    <Tag color="primary" className="font-normal">
      草稿版本
    </Tag>
  );
};
