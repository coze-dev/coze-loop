// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// ClassifyStepErrorType 把一个阶段的结果归入错误分类看板类别。
//
// 放在 domain 而不是 metric 实现里，是因为它有**两个**消费者：metric 的 error_type tag 和 MQ
// 明细的 error_type 列。分类规则在两处各写一份，早晚会漂成两个答案，那时「在线看板说是工程错误、
// Hive 说不是」这类问题无从下手。
//
//	success == true          → "-"                成功不带错误分类
//	success == false, code=0 → "unknown"          失败但上报侧没给错误码
//	success == false, 命中表  → "non_engineering"  用户 / 题目侧问题，不计入 SLA
//	success == false, 其它    → "engineering"      平台工程问题，计入 SLA
//
// ⚠️ **默认值刻意是 engineering，与前人相反**（spec D4）。前人的 ClassifyErrorType 硬编码 9 个
// coze-loop 侧 601 开头的错误码，命中即 engineering，**否则 non_engineering**。而 runtime 的
// pkg/errno 是另一套码（600 开头），直接上报一个都命中不了——所有运行时故障会被归成「用户侧
// 问题、不计入 SLA」，与本需求（及时发现故障）直接冲突。
//
// 默认值必须指向「会被发现」的那一侧：漏配算成非工程错误 → 故障被静默豁免（最坏）；漏配算成
// 工程错误 → 误报，但会有人发现并去补配置。
//
// nonEngineeringStepErrorCodes 当前**故意为空**：runtime errno 的「用户侧 / 平台侧」切分还没定，
// 而在定下来之前，把所有带码的失败算成 engineering 正是上面那条默认值规则要的结果。这张表是
// spec D4 的 TCC 化改造（另一张 ticket）要接进来的位置——届时它变成从 IConfiger 读取，本函数
// 的分类逻辑不变。
var nonEngineeringStepErrorCodes = map[int32]struct{}{}

func ClassifyStepErrorType(success bool, code int32) string {
	if success {
		return "-"
	}
	if code == 0 {
		return "unknown"
	}
	if _, ok := nonEngineeringStepErrorCodes[code]; ok {
		return "non_engineering"
	}
	return "engineering"
}
