// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"time"

	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

const (
	HoursPerDay = 24
)

type DateValidator struct {
	Start        int64 // ms
	End          int64 // ms
	EarliestDays int64
}

func (d *DateValidator) CorrectDate() (int64, int64, error) {
	if d.Start <= 0 || d.End <= 0 {
		return 0, 0, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time or end time is invalid"))
	} else if d.Start > d.End {
		return 0, 0, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time cannot be greater than end time"))
	}
	now := time.Now()
	earliestTime := StartTimeOfDay(now.UnixMilli()) -
		int64(time.Duration(d.EarliestDays*HoursPerDay)*time.Hour/time.Millisecond)
	latestTime := EndTimeOfDay(now.UnixMilli())
	if d.End > latestTime && d.Start > latestTime {
		return 0, 0, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start&end time both exceed today"))
	} else if d.End < earliestTime && d.Start < earliestTime {
		return 0, 0, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start&end time both exceed max days ago"))
	}
	newStartTime := d.Start
	newEndTime := d.End
	if d.End >= earliestTime && d.Start < earliestTime {
		newStartTime = earliestTime
	}
	if d.End > latestTime && d.Start <= latestTime {
		newEndTime = latestTime
	}
	return newStartTime, newEndTime, nil
}

func StartTimeOfDay(from int64) int64 {
	year, month, day := time.UnixMilli(from).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local).UnixMilli()
}

func EndTimeOfDay(from int64) int64 {
	year, month, day := time.UnixMilli(from).Date()
	return time.Date(year, month, day, 23, 59, 59, 999999999, time.Local).UnixMilli()
}
