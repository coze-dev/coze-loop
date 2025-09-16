// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Version, VersionItem, VersionList } from '@cozeloop/components';
import {
  type EvaluatorVersion,
  type Evaluator,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozCross } from '@coze-arch/coze-design/icons';
import { IconButton, Spin } from '@coze-arch/coze-design';

function evaluatorVersionToVersion(
  evaluatorVersion: EvaluatorVersion,
): Version {
  const version = evaluatorVersion;
  const versionItem: Version = {
    id: version.id || '',
    version: version.version,
    description: version.description,
    submitTime: version.base_info?.created_at,
    submitter: version.base_info?.created_by,
    isDraft: false,
  };
  return versionItem;
}

export function VersionListPane({
  evaluator,
  selectedVersion,
  onSelectVersion,
  onClose,
  refreshFlag,
}: {
  evaluator: Evaluator;
  selectedVersion: EvaluatorVersion | undefined;
  onSelectVersion: (version: EvaluatorVersion | undefined) => void;
  onClose: () => void;
  refreshFlag: never[];
}) {
  const [versions, setVersions] = useState<Version[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const service = useRequest(
    async ({ pageNumber }: { pageNumber: number }) =>
      StoneEvaluationApi.ListEvaluatorVersions({
        workspace_id: evaluator.workspace_id || '',
        evaluator_id: evaluator.evaluator_id,
        page_size: 20,
        page_number: pageNumber,
      }),
    {
      manual: true,
    },
  );
  const handleLoadMore = async (pageNumber: number) => {
    const res = await service.runAsync({ pageNumber });
    setTotal(Number(res?.total) || 0);
    setPage(pageNumber + 1);
    const newVersions =
      res.evaluator_versions?.map(evaluatorVersionToVersion) ?? [];
    if (newVersions.length > 0) {
      setVersions(oldVersions => [...oldVersions, ...newVersions]);
    }
  };

  useEffect(() => {
    setVersions([]);
    handleLoadMore(1);
  }, [refreshFlag]);

  return (
    <div className="flex-shrink-0 w-[340px] h-full overflow-hidden flex flex-col border-0 border-l border-solid coz-stroke-primary">
      <div className="flex-shrink-0 h-12 px-6 flex flex-row items-center justify-between coz-mg-secondary border-0 border-b border-solid coz-stroke-primary">
        <div className="text-sm font-medium coz-fg-plus">
          {I18n.t('version_record')}
        </div>
        <IconButton
          className="flex-shrink-0"
          color="secondary"
          size="small"
          icon={<IconCozCross className="w-4 h-4 coz-fg-primary" />}
          onClick={onClose}
        />
      </div>
      <div className="flex-1 overflow-y-auto p-6 gap-3 styled-scrollbar pr-[18px]">
        {versions.length === 0 && service.loading ? (
          <div className="h-full flex items-center justify-center">
            <Spin spinning={true} />
          </div>
        ) : (
          <>
            <VersionItem
              key={'isDraft'}
              className="pb-3"
              version={{
                id: 'isDraft',
                isDraft: true,
                submitTime: evaluator?.base_info?.updated_at,
              }}
              active={!selectedVersion}
              onClick={() => onSelectVersion(undefined)}
            />
            <VersionList
              activeVersionId={selectedVersion?.id ?? 'draft'}
              onActiveChange={(_, version) => {
                if (selectedVersion && selectedVersion.id === version.id) {
                  return;
                }
                onSelectVersion(version.isDraft ? undefined : version);
              }}
              enableLoadMore={true}
              noMore={total <= versions?.length}
              loadMoreLoading={service.loading}
              versions={versions}
              onLoadMore={() => {
                handleLoadMore(page);
              }}
            />
          </>
        )}
      </div>
    </div>
  );
}
