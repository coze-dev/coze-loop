// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useMemo } from 'react';

import { usePagination } from 'ahooks';
import { DEFAULT_PAGE_SIZE } from '@cozeloop/evaluate-components';
import { TableWithPagination, getStoragePageSize } from '@cozeloop/components';
import {
  type ListWebhookDeliveryRequest,
  type WebhookDelivery,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Tag, EmptyState } from '@coze-arch/coze-design';

interface Props {
  spaceID: string;
  experimentID: string;
}

const STATUS_COLORS: Record<string, 'green' | 'red' | 'yellow' | 'grey'> = {
  success: 'green',
  failed: 'red',
  retrying: 'yellow',
  pending: 'grey',
};

function formatMs(ms?: string): string {
  if (!ms || ms === '0') {
    return '-';
  }
  const n = Number(ms);
  if (!Number.isFinite(n) || n <= 0) {
    return '-';
  }
  return new Date(n).toLocaleString();
}

export default function NotificationLog({ spaceID, experimentID }: Props) {
  const service = usePagination(
    async ({
      current,
      pageSize,
    }: {
      current: number;
      pageSize: number;
    }): Promise<{ total: number; list: WebhookDelivery[] }> => {
      if (!spaceID || !experimentID) {
        return { total: 0, list: [] };
      }
      const params: ListWebhookDeliveryRequest = {
        workspace_id: spaceID,
        experiment_id: experimentID,
        page_number: current,
        page_size: pageSize,
      };
      const res = await StoneEvaluationApi.ListWebhookDelivery(params);
      return {
        total: Number(res.total) || 0,
        list: (res.deliveries ?? []) as WebhookDelivery[],
      };
    },
    {
      defaultPageSize:
        getStoragePageSize('notification_log_page_size') ?? DEFAULT_PAGE_SIZE,
      manual: true,
      refreshDeps: [spaceID, experimentID],
    },
  );

  useEffect(() => {
    if (spaceID && experimentID) {
      service.run({ current: 1, pageSize: service.pagination?.pageSize });
    }
  }, [spaceID, experimentID, service.run]);

  const columns = useMemo(
    () => [
      {
        title: '投递状态',
        dataIndex: 'status',
        width: 100,
        render: (v: string) => (
          <Tag color={STATUS_COLORS[v] ?? 'grey'}>{v || '-'}</Tag>
        ),
      },
      { title: '触发事件', dataIndex: 'event_type', width: 120 },
      { title: '通道', dataIndex: 'channel_type', width: 140 },
      {
        title: 'URL',
        dataIndex: 'webhook_url',
        ellipsis: { showTitle: true },
      },
      {
        title: '首次投递',
        dataIndex: 'first_sent_at_ms',
        width: 170,
        render: (v: string) => formatMs(v),
      },
      {
        title: '末次投递',
        dataIndex: 'last_sent_at_ms',
        width: 170,
        render: (v: string) => formatMs(v),
      },
      {
        title: '次数',
        width: 80,
        render: (_: unknown, r: WebhookDelivery) =>
          `${r.attempt_count ?? 0}/${r.max_attempts ?? 0}`,
      },
      { title: 'HTTP', dataIndex: 'response_code', width: 90 },
      {
        title: '错误信息',
        dataIndex: 'error_message',
        ellipsis: { showTitle: true },
      },
    ],
    [],
  );

  return (
    <TableWithPagination<WebhookDelivery>
      heightFull
      service={service}
      pageSizeStorageKey="notification_log_page_size"
      showTableWhenEmpty={false}
      tableProps={{
        rowKey: 'delivery_id',
        columns,
        sticky: { top: 0 },
        loading: service.loading,
      }}
      empty={<EmptyState size="full_screen" title="暂无通知记录" />}
    />
  );
}
