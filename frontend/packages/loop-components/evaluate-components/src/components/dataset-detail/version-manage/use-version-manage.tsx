// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useMemo, useState } from 'react';

import { useRequest } from 'ahooks';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Version, VersionSwitchPanel } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Button } from '@coze-arch/coze-design';

interface ReqParams {
  page?: number;
  pageSize?: number;
}

const PAGE_SIZE = 20;

export const useVersionManage = ({
  datasetDetail,
  currentVersion,
  setCurrentVersion,
}: {
  currentVersion: Version | undefined;
  setCurrentVersion: (version: Version) => void;
  datasetDetail?: EvaluationSet;
}) => {
  const { spaceID } = useSpace();
  const [visible, setVisible] = useState(false);
  const [versions, setVersions] = useState<Version[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const { loading, runAsync: loadMoreData } = useRequest(
    async (params: ReqParams) => {
      const res = await StoneEvaluationApi.ListEvaluationSetVersions({
        evaluation_set_id: datasetDetail?.id as string,
        workspace_id: spaceID,
        page_number: params.page,
        page_size: params.pageSize,
      });
      setTotal(Number(res?.total) || 0);
      return res?.versions?.map(
        version =>
          ({
            id: version.id as string,
            version: version?.version,
            description: version?.description,
            submitTime: version?.base_info?.created_at,
            submitter: version?.base_info?.created_by,
          }) satisfies Version,
      );
    },
    {
      manual: true,
    },
  );
  useEffect(() => {
    if (visible) {
      setPage(1);
      loadMoreData({ page: 1, pageSize: PAGE_SIZE })
        .then(newVersions => {
          setVersions(newVersions || []);
        })
        .catch(console.error);
    }
  }, [visible, datasetDetail?.latest_version]);

  const draftVersion = useMemo(
    () => ({
      id: 'draft',
      version: '0.0.0',
      description: I18n.t('current_draft'),
      submitTime: datasetDetail?.base_info?.updated_at,
      draftSubmitText: I18n.t('update_time'),
      isDraft: true,
    }),
    [datasetDetail?.base_info?.updated_at],
  );

  const versionList = useMemo(
    () => [draftVersion, ...versions],
    [draftVersion, versions],
  );
  const VersionPanel = (
    <VersionSwitchPanel
      visible={visible}
      onClose={() => setVisible(false)}
      activeVersionId={currentVersion?.id}
      onActiveChange={(_, version) => {
        setVisible(false);
        setCurrentVersion(version);
      }}
      enableLoadMore={true}
      noMore={total <= versions?.length}
      loadMoreLoading={loading}
      versions={versionList}
      onLoadMore={async () => {
        const newVersions = await loadMoreData({
          page: page + 1,
          pageSize: PAGE_SIZE,
        });
        setPage(page + 1);
        setVersions(old => [...old, ...(newVersions || [])]);
      }}
    />
  );

  const VersionChangeButton = (
    <Button
      color="primary"
      onClick={() => {
        setVisible(true);
        sendEvent(EVENT_NAMES.cozeloop_dataset_version);
      }}
    >
      {I18n.t('version_record')}
    </Button>
  );

  return {
    VersionPanel,
    VersionChangeButton,
  };
};
