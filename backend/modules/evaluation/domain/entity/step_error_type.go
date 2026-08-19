// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// ClassifyStepErrorType 把一个阶段的结果归入错误分类看板类别。
//
// 放在 domain 而不是 metric 实现里，是因为它有**两个**消费者：metric 的 error_type tag 和 MQ
// 明细的 error_type 列。分类规则在两处各写一份，早晚会漂成两个答案，那时「在线看板说是工程错误、
// Hive 说不是」这类问题无从下手。
//
//	success == true            → "-"                成功不带错误分类
//	success == false, code=0   → "unknown"          失败但上报侧没给错误码
//	success == false, 命中配置  → "non_engineering"  用户 / 题目侧问题，不计入 SLA
//	success == false, 其它      → "engineering"      平台工程问题，计入 SLA
//
// ⚠️ **默认值刻意是 engineering，与前人相反**（spec D4）。前人的 ClassifyErrorType 硬编码 9 个
// coze-loop 侧 601 开头的错误码，命中即 engineering，**否则 non_engineering**。而 runtime 的
// pkg/errno 是另一套码（600 开头），直接上报一个都命中不了——所有运行时故障会被归成「用户侧
// 问题、不计入 SLA」，与本需求（及时发现故障）直接冲突。
//
// 默认值必须指向「会被发现」的那一侧：漏配算成非工程错误 → 故障被静默豁免（最坏）；漏配算成
// 工程错误 → 误报，但会有人发现并去补配置。这也让配置被整值覆盖清空时的失败方向是安全的。
//
// cfg 为 nil（配置读取失败 / 键不存在）时 IsNonSLACode 恒 false，于是全部落到 engineering——
// 「读不到配置」与「码不在表里」共用这一个安全出口，不在读配置处再写一份兜底。
func ClassifyStepErrorType(success bool, code int32, cfg *ExptSandboxStepMetricConf) string {
	if success {
		return "-"
	}
	if code == 0 {
		return "unknown"
	}
	if cfg.IsNonSLACode(code) {
		return "non_engineering"
	}
	return "engineering"
}

// ExptSandboxStepMetricConf 评测链路阶段埋点的可运维配置（配置键 expt_sandbox_step_metric_cfg）。
//
// 配置值是个对象而不是直接一个码列表，当前只有 non_sla_code 一个字段：后续新增设置项时
// 直接加字段，不用再开一个配置键。
type ExptSandboxStepMetricConf struct {
	// NonSLACode 不计入 SLA 的错误码。表达方向是 error_type → 码列表，而非码 → 类型：
	// 运维配置时的心智是「哪些码算非工程错误」，反向表要为每个码写一行且新增码时容易漏。
	NonSLACode []int32 `json:"non_sla_code" mapstructure:"non_sla_code"`
}

// IsNonSLACode 判断错误码是否被配成非 SLA。接收者为 nil 或列表为空时返回 false。
func (c *ExptSandboxStepMetricConf) IsNonSLACode(code int32) bool {
	if c == nil {
		return false
	}
	for _, v := range c.NonSLACode {
		if v == code {
			return true
		}
	}
	return false
}
