// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package time_range

import (
	"context"
)

//go:generate mockgen -destination=mocks/time_range_mock.go -package=mocks . ITimeRangeProvider
type ITimeRangeProvider interface {
	GetTimeRange(ctx context.Context, workSpaceID, logID, traceID string, delayTime int64) (*int64, *int64)
}
