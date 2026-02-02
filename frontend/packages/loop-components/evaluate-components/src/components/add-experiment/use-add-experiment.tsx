// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, Guard } from '@cozeloop/guard';
import { type Version } from '@cozeloop/components';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { IconCozPlusFill } from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';

import { ExperimentModal } from './experiment-modal';
export const useAddExperiment = ({
  datasetDetail,
  currentVersion,
  isDraftVersion,
}: {
  datasetDetail?: EvaluationSet;
  currentVersion?: Version;
  isDraftVersion?: boolean;
}) => {
  const [visible, setVisible] = useState(false);
  const navigateModule = useNavigateModule();
  const onOk = (versionID: string) => {
    navigateModule(
      `evaluation/experiments/create?evaluation_set_id=${datasetDetail?.id}&version_id=${versionID}`,
    );
  };
  const ExperimentButton = (
    <Guard point={GuardPoint['eval.dataset.create_experiment']} realtime>
      <Button
        color="primary"
        icon={<IconCozPlusFill />}
        onClick={() => {
          setVisible(true);
        }}
        disabled={!datasetDetail?.latest_version}
      >
        {I18n.t('new_experiment')}
      </Button>
    </Guard>
  );

  const ExperimentModalNode = visible ? (
    <ExperimentModal
      isDraftVersion={isDraftVersion}
      currentVersion={currentVersion}
      datasetDetail={datasetDetail}
      onOk={onOk}
      onCancel={() => {
        setVisible(false);
      }}
    />
  ) : null;

  return {
    ExperimentButton,
    ExperimentModalNode,
  };
};
