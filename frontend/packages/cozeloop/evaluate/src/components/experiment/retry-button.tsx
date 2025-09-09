// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { Guard, GuardPoint } from '@cozeloop/guard';
import { ExptRetryMode, ExptStatus } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Button, Toast } from '@coze-arch/coze-design';

import styles from './retry-button.module.less';

export default function RetryButton({
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
  // 失败时展示重试
  if (status === ExptStatus.Failed) {
    const onClick = async () => {
      setLoading(true);
      Toast.info({
        content: I18n.t('retrying'),
        icon: <></>,
        className: styles.toast,
      });
      await StoneEvaluationApi.RetryExperiment({
        workspace_id: spaceID,
        expt_id,
        retry_mode: ExptRetryMode.RetryAll,
      });
      // TODO: 增加重试的时候语音PM没回复, 先给足够延时
      setTimeout(() => {
        onRefresh?.();
        setLoading(false);
      }, 1200);
    };

    return (
      <Guard point={GuardPoint['eval.experiments.retry']}>
        <Button
          color="primary"
          loading={loading}
          disabled={loading}
          onClick={() => onClick?.()}
        >
          {I18n.t('retry')}
        </Button>
      </Guard>
    );
  }

  return null;
}
