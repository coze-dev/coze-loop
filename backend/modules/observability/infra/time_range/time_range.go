// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package time_range

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/time_range"
)

type TimeRangeProvider struct{}

func NewTimeRangeProvider() time_range.ITimeRangeProvider {
	return &TimeRangeProvider{}
}

func (p *TimeRangeProvider) GetTimeRange(ctx context.Context, workSpaceID, logID, traceID string) (*int64, *int64) {
	return nil, nil
}
