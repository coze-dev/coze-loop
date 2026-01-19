// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { PlatformType, type span } from '@cozeloop/api-schema/observation';
import { Typography } from '@coze-arch/coze-design';

import { JumpButton } from '@/shared/components/jump-button';
import { getNodeConfig } from '@/features/trace-detail/utils/span';
import { SpanType } from '@/features/trace-detail/types/params';
import { useTraceDetailContext } from '@/features/trace-detail/hooks/use-trace-detail-context';
import { CustomIconWrapper } from '@/features/trace-detail/constants/span';

import styles from './index.module.less';
export const SpanDetailHeader: React.FC<{
  span: span.OutputSpan;
}> = ({ span }) => {
  const { type, span_type } = span;
  const nodeConfig = getNodeConfig({
    spanTypeEnum: type ?? SpanType.Unknown,
    spanType: span_type,
  });
  const { spanDetailHeaderSlot, platformType } = useTraceDetailContext();

  return (
    <div className={styles['detail-header']}>
      <div className={styles['detail-title']}>
        <span className={styles['icon-wrapper']}>
          {nodeConfig.icon ? (
            nodeConfig.icon({ className: '!w-[16px] !h-[16px]', size: 'large' })
          ) : (
            <CustomIconWrapper color={nodeConfig.color} size="large">
              {nodeConfig.character}
            </CustomIconWrapper>
          )}
        </span>
        <Typography.Text
          ellipsis={{ rows: 1 }}
          className="text-[16px] !font-semibold"
        >
          {span.span_name}
        </Typography.Text>
      </div>
      <div className="flex items-center gap-x-2">
        {spanDetailHeaderSlot?.(span, platformType ?? PlatformType.Cozeloop)}
        <JumpButton span={span} />
      </div>
    </div>
  );
};
