// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central_schedule

import "sort"

// SortRuns 按优先级降序、等待时长升序排序。
// waiting 以 run 的 created_at 为准（越早越先出）。
func SortRuns(runs []*RunView) {
	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].Priority != runs[j].Priority {
			return runs[i].Priority > runs[j].Priority
		}
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.Before(runs[j].CreatedAt)
		}
		if runs[i].ExptID != runs[j].ExptID {
			return runs[i].ExptID < runs[j].ExptID
		}
		return runs[i].RunID < runs[j].RunID
	})
}
