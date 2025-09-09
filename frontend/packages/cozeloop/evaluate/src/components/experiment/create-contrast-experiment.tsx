// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { ExptStatus, type Experiment } from '@cozeloop/api-schema/evaluation';
import { IconCozCompare } from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';

import ExperimentSelectModal from './experiment-select-modal';

export default function CreateContrastExperiment({
  baseExperiment,
  disabled,
  onClick,
  onReportCompare,
}: {
  baseExperiment?: Experiment;
  disabled?: boolean;
  onClick?: () => void;
  onReportCompare?: (status: string) => void;
}) {
  const [visible, setVisible] = useState<boolean>(false);
  const navigate = useNavigateModule();

  const contrastExperiments = useMemo(() => {
    if (!baseExperiment) {
      return [];
    }
    return [baseExperiment];
  }, [baseExperiment]);

  return (
    <>
      <Button
        icon={<IconCozCompare />}
        disabled={!baseExperiment || disabled}
        onClick={() => {
          onClick?.();
          setVisible(true);
        }}
      >
        {I18n.t('experiment_comparison')}
      </Button>

      {visible ? (
        <ExperimentSelectModal
          contrastExperiments={contrastExperiments}
          disabledFilterFields={['eval_set', 'status']}
          onReportCompare={status => {
            onReportCompare?.(status);
          }}
          defaultFilter={{
            status: [
              ExptStatus.Success,
              ExptStatus.Failed,
              ExptStatus.Terminated,
            ],
            eval_set: [contrastExperiments?.[0]?.eval_set?.id ?? ''].filter(
              Boolean,
            ),
          }}
          onOk={ids => {
            navigate(
              `evaluation/experiments/contrast?experiment_ids=${ids.join(',')}`,
            );
          }}
          onClose={() => setVisible(false)}
        />
      ) : null}
    </>
  );
}
