// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import "time"

// Usec2Msec 微秒转毫秒
func Usec2Msec(usec int64) int64 {
	d := time.Duration(usec) * time.Microsecond
	return int64(d / time.Millisecond)
}
