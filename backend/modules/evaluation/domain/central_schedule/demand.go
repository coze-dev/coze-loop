// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central_schedule

// Available 计算本次可补的 item 数。
//
// 口径（用户已确认）：目标并发 − 已派发未确认 − 已预占。
// 已确认在途（processing）也必须扣减，否则会超补；因此这里把它作为
// 独立项显式扣掉，等价于“目标并发 − 所有已派发 − 已预占”。
func Available(target, inFlightConfirmed, dispatchedUnconfirmed, reserved int64) int64 {
	available := target - inFlightConfirmed - dispatchedUnconfirmed - reserved
	if available < 0 {
		return 0
	}
	return available
}

// AggregateDimensions 汇总一批 item 的维度需求。
func AggregateDimensions(items []*ItemView) []Dimension {
	if len(items) == 0 {
		return nil
	}
	var all []Dimension
	for _, item := range items {
		all = append(all, item.Dimensions...)
	}
	return Dimensions(all)
}

// MinBatchBy 按维度余量裁剪 batch，保证候选 batch 不会因某个维度超量。
// 若任一维度余量为 0，返回空 batch（整批不放）。
func MinBatchBy(items []*ItemView, remaining map[string]int64) []*ItemView {
	if len(items) == 0 {
		return nil
	}
	out := make([]*ItemView, 0, len(items))
	used := make(map[string]int64, len(remaining))
	for _, item := range items {
		for _, d := range item.Dimensions {
			if d.Amount <= 0 {
				continue
			}
			// 未显式配置上限的维度视为不限；由 Ledger 的 limit=-1 兜底。
			if lim, ok := remaining[d.Key]; ok && used[d.Key]+d.Amount > lim {
				return out
			}
		}
		for _, d := range item.Dimensions {
			if d.Amount > 0 {
				used[d.Key] += d.Amount
			}
		}
		out = append(out, item)
	}
	return out
}
