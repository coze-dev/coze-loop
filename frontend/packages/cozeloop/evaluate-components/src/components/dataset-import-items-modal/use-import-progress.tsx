// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, useState } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace, useDataImportApi } from '@cozeloop/biz-hooks-adapter';
import { JobStatus } from '@cozeloop/api-schema/data';
import { Button, Loading, Modal } from '@coze-arch/coze-design';

import { ImportResultInfo } from '../import-result-info';

export const useDatasetImportProgress = (onImportSuccess: () => void) => {
  const [visible, setVisible] = useState(false);
  const { getDatasetIOJobApi } = useDataImportApi();
  const { spaceID } = useSpace();
  const { data, runAsync, cancel, error } = useRequest(
    async (jobID: string) => {
      const res = await getDatasetIOJobApi({
        job_id: jobID,
        workspace_id: spaceID,
      });
      return res.job;
    },
    {
      pollingErrorRetryCount: 0,
      pollingInterval: 4000,
      pollingWhenHidden: false,
      manual: true,
      onError: () => {
        cancel();
      },
    },
  );

  const { isFinish } = useMemo(() => {
    const isDataFinish =
      [JobStatus.Completed, JobStatus.Failed, JobStatus.Cancelled].includes(
        data?.status || JobStatus.Undefined,
      ) || error !== undefined;
    if (isDataFinish) {
      cancel();
    }
    return { isFinish: isDataFinish };
  }, [data, error]);
  const startProgressTask = async (taskID: string) => {
    setVisible(true);
    await runAsync(taskID);
    return '';
  };
  const { progress, errors } = data || {};
  const node = (
    <Modal
      visible={visible}
      centered={true}
      closable={false}
      keepDOM={false}
      width={420}
      title={
        <div className="flex items-center">
          <span>
            {isFinish ? I18n.t('execution_result') : I18n.t('status_running')}
          </span>
        </div>
      }
      footer={
        isFinish && (
          <Button
            onClick={() => {
              setVisible(false);
              onImportSuccess();
            }}
          >
            {I18n.t('known')}
          </Button>
        )
      }
    >
      {isFinish ? (
        <div className="py-2">
          <ImportResultInfo progress={progress} errors={errors} />
        </div>
      ) : (
        <div className="p-6 flex justify-center items-center">
          <Loading size="large" loading={true} color="blue" />
        </div>
      )}
    </Modal>
  );

  return {
    node,
    startProgressTask,
  };
};
