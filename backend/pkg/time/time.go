// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package time

import (
	"time"
)

func UnixMilliToTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ts)
}

func MillSec2MicroSec(millSec int64) int64 {
	d := time.Duration(millSec) * time.Millisecond
	return int64(d / time.Microsecond)
}

func MicroSec2MillSec(microSec int64) int64 {
	d := time.Duration(microSec) * time.Microsecond
	return int64(d / time.Millisecond)
}

func Day2MillSec(day int) int64 {
	d := time.Duration(day) * 24 * time.Hour
	return int64(d / time.Millisecond)
}
