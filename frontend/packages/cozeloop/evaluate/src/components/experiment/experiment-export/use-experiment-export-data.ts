import { useEffect } from 'react';

import { usePagination } from 'ahooks';
import { DEFAULT_PAGE_SIZE } from '@cozeloop/evaluate-components';
import { getStoragePageSize } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type ListExptResultExportRecordRequest,
  type ExptResultExportRecord,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

export function useExperimentExportData(exptId: string) {
  const { spaceID } = useSpace();

  const service = usePagination(
    async ({
      current,
      pageSize,
    }: {
      current: number;
      pageSize: number;
    }): Promise<{
      total: number;
      list: ExptResultExportRecord[];
    }> => {
      if (!exptId || !spaceID) {
        return {
          total: 0,
          list: [],
        };
      }
      // 这里需要补充 workspace_id 和 expt_id，假设可以从 spaceID 获取 workspace_id
      // expt_id 需要根据实际情况传递，这里暂时写成空字符串
      const params: ListExptResultExportRecordRequest = {
        workspace_id: spaceID.toString(),
        expt_id: exptId,
        page_number: current,
        page_size: pageSize,
      };
      const res = await StoneEvaluationApi.ListExptResultExportRecord(params);

      return {
        total: Number(res.total) || 0,
        list: res.expt_result_export_records as unknown as ExptResultExportRecord[],
      };
    },
    {
      defaultPageSize:
        getStoragePageSize('export_table_page_size') ?? DEFAULT_PAGE_SIZE,
      manual: true,
      refreshDeps: [exptId, spaceID],
    },
  );

  // 只有在 exptId 和 spaceID 都有效时才执行请求
  useEffect(() => {
    if (exptId && spaceID) {
      service.run({
        current: 1,
        pageSize: service.pagination?.pageSize,
      });
    }
  }, [exptId, spaceID, service.run]);

  return {
    service,
  };
}
