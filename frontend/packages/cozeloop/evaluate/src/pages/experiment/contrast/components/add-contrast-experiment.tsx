// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { ExptStatus, type Experiment } from '@cozeloop/api-schema/evaluation';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';

import { ExperimentSelectModal } from '@/components/experiment';

export default function AddContrastExperiment({
  currentExperiments = [],
  onClick,
  onOk,
}: {
  currentExperiments: Experiment[];
  onClick?: () => void;
  onOk?: (ids: Int64[]) => void;
}) {
  const [visible, setVisible] = useState<boolean>(false);
  return (
    <>
      <Button
        disabled={!currentExperiments?.[0]}
        icon={<IconCozPlus />}
        onClick={() => {
          onClick?.();
          setVisible(true);
        }}
      >
        {I18n.t('add_comparison_experiment')}
      </Button>
      {visible ? (
        <ExperimentSelectModal
          contrastExperiments={currentExperiments}
          disabledFilterFields={['eval_set', 'status']}
          defaultFilter={{
            status: [
              ExptStatus.Success,
              ExptStatus.Failed,
              ExptStatus.Terminated,
            ],
            eval_set: [currentExperiments?.[0]?.eval_set?.id ?? ''].filter(
              Boolean,
            ),
          }}
          onReportCompare={status => {
            sendEvent(EVENT_NAMES.cozeloop_experiment_compare_count, {
              from: 'comparative_expt',
              status: status ?? 'success',
            });
          }}
          onOk={keys => {
            onOk?.(keys);
            setVisible(false);
          }}
          onClose={() => setVisible(false)}
        />
      ) : null}
    </>
  );
}
