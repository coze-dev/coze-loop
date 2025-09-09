// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Button, List } from '@coze-arch/coze-design';

import VersionItem from './version-item';
import { type Integer64, type Version } from './version-descriptions';

export interface VersionListProps {
  versions: Version[] | undefined;
  activeVersionId?: Integer64;
  loadMoreLoading?: boolean;
  enableLoadMore?: boolean;
  noMore?: boolean;
  onActiveChange?: (versionId: Integer64, version: Version) => void;
  onLoadMore?: () => void;
}

export default function VersionList({
  versions = [],
  loadMoreLoading = false,
  enableLoadMore = false,
  noMore = false,
  activeVersionId,
  onActiveChange,
  onLoadMore,
}: VersionListProps) {
  const loadMore =
    !enableLoadMore || noMore ? null : (
      <div className="flex justify-center">
        <Button
          loading={loadMoreLoading}
          color="primary"
          onClick={() => onLoadMore?.()}
        >
          {I18n.t('load_more')}
        </Button>
      </div>
    );

  return (
    <>
      <List
        dataSource={versions}
        loadMore={loadMore}
        renderItem={version => {
          const active =
            activeVersionId === undefined || activeVersionId === version?.id;
          return (
            <VersionItem
              key={version.id}
              className="pb-3"
              version={version}
              active={active}
              onClick={() => onActiveChange?.(version?.id, version)}
            />
          );
        }}
      />
    </>
  );
}
