// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// SchedulerQueueScanParam 中心调度候选实验的跨空间扫描参数。
//
// 排序固定为 priority_level DESC, created_at ASC, id ASC：
//   - priority 越大越优先；
//   - 同优先级按实验创建时间先到先得（Retry 沿用原 created_at，不额外维护入队时间）；
//   - 再相同以 id 兜底，保证顺序确定 —— 否则翻页会重复或漏掉实验。
type SchedulerQueueScanParam struct {
	// DispatchMode 目标调度模式，通常为 enforce。
	DispatchMode string
	// Statuses 候选状态，通常为 Pending + Processing。用单条 status IN (...) 查询而非按状态分别扫描，
	// 避免双流归并游标与未消费 lookahead。
	Statuses []int32
	// Cursor keyset 游标，nil 表示从队头开始。
	Cursor *SchedulerQueueCursor
	// Limit 单页条数。
	Limit int32
}

// SchedulerQueueCursor keyset 游标，与排序键一一对应。
//
// 用 keyset 而非 offset 分页：候选集合在扫描过程中会被并发写入（新实验提交、实验进终态），
// offset 分页在这种场景下会漏掉或重复元素。
type SchedulerQueueCursor struct {
	PriorityLevel int32
	CreatedAtUnix int64
	ExptID        int64
}
