// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { Guard, GuardPoint } from '@cozeloop/guard';
import { ExptStatus } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Button, Toast } from '@coze-arch/coze-design';

import styles from './kill-button.module.less';

export default function KillButton({
  status,
  spaceID,
  expt_id = '',
  onRefresh,
}: {
  status?: ExptStatus;
  spaceID?: string;
  expt_id?: string;
  onRefresh: (() => void) | undefined;
}) {
  const [loading, setLoading] = useState(false);

  // 只在 Pending 或 Processing 状态时展示终止按钮
  if (status === ExptStatus.Pending || status === ExptStatus.Processing) {
    const onClick = async () => {
      setLoading(true);
      Toast.info({
        content: I18n.t('terminating'),
        icon: <></>,
        className: styles.toast,
      });
      try {
        await StoneEvaluationApi.KillExperiment({
          workspace_id: spaceID,
          expt_id,
        });
      } catch (error) {
        console.error('Kill experiment failed:', error);
      }
      // 给足够延时确保状态更新
      setTimeout(() => {
        onRefresh?.();
        setLoading(false);
      }, 1200);
    };

    return (
      <Guard point={GuardPoint['eval.experiments.kill']}>
        <Button
          color="primary"
          loading={loading}
          disabled={loading}
          onClick={() => onClick?.()}
        >
          {I18n.t('terminate')}
        </Button>
      </Guard>
    );
  }

  return null;
}
