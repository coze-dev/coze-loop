// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { IconCozFileAudio } from '@coze-arch/coze-design/illustrations';

import { type DatasetItemProps } from '../type';

export const AudioDatasetItem: React.FC<DatasetItemProps> = ({
  fieldContent,
  expand,
  onChange,
}) => {
  const { audio } = fieldContent || {};
  return (
    <IconCozFileAudio
      onClick={() => {
        window.open(audio?.url, '_blank');
      }}
      className="h-[36px] w-[36px] cursor-pointer"
    />
  );
};
