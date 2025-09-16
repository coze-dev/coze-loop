// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type span } from '@cozeloop/api-schema/observation';

import { type TraceDetailContext } from '@/trace-detail';

import { TraceFeedBack } from './trace-detail-table';

export const tabs: TraceDetailContext['extraSpanDetailTabs'] = [
  {
    label: 'Feedback',
    tabKey: 'feedback',
    render: (span: span.OutputSpan, platformType: string | number) => (
      <TraceFeedBack
        span={span}
        platformType={platformType}
        annotationRefreshKey={0}
      />
    ),
    visible: true,
  },
];
