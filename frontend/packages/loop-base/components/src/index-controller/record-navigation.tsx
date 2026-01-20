// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  IconCozArrowLeft,
  IconCozArrowRight,
} from '@coze-arch/coze-design/icons';
import { Space, Button, Tooltip } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { type IndexControllerStore } from './use-item-index-controller';

/** 控制上一个下一个记录的控制器组件 */
export function IndexControllerView({
  indexControllerStore,
  className,
}: {
  indexControllerStore: IndexControllerStore;
  className?: string;
}) {
  const I18n = useI18n();
  const {
    hasPrevious,
    hasNext,
    currentIndex,
    total,
    loading,
    goToPrevious,
    goToNext,
  } = indexControllerStore ?? {};
  return (
    <Space className={className} spacing={4}>
      <Tooltip content={I18n.t('prev_item')} theme="dark">
        <div className="flex items-center">
          <Button
            icon={<IconCozArrowLeft />}
            size="mini"
            color="primary"
            style={{ height: 20 }}
            disabled={!hasPrevious || loading}
            onClick={goToPrevious}
          />
        </div>
      </Tooltip>
      <span className="text-sm text-gray-500 min-w-[64px] text-center">
        {currentIndex + 1} / {total}
      </span>
      <Tooltip content={I18n.t('next_one')} theme="dark">
        <div className="flex items-center">
          <Button
            icon={<IconCozArrowRight />}
            size="mini"
            color="primary"
            style={{ height: 20 }}
            disabled={!hasNext || loading}
            onClick={goToNext}
          />
        </div>
      </Tooltip>
    </Space>
  );
}
