// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

// StepEventTags 评测链路阶段事件的 metric tag 集合，**只有低基数维度**。
//
// ⚠️ 这个 struct 的字段清单本身就是一条约束：invoke_id / experiment_id / item_id / item_key /
// log_id / dataset_id / dataset_version / model_name 一个都不在这里，且不应该被加进来。
// metric 的 series 数是各 tag 取值的笛卡尔积，把 invoke 级标识放进 tag 意味着每跑一次实验就
// 新增一整套永不复用的 series。这些明细维度只走 MQ（Hive 宽表不怕多列），不走 metric。
//
// 前人的 SandboxAgentStepTags 正是把 invoke_id / item_id / item_key 放进了 tag，
// 且只对 error_code 写了基数风险注释——本 struct 刻意不复制那个缺陷。
type StepEventTags struct {
	StepName  string
	AgentType string
	Round     int32
}

//go:generate mockgen -destination=mocks/step_event.go -package=mocks . StepEventMetrics
type StepEventMetrics interface {
	// EmitStepStarted 阶段开始事件，只有 counter。
	// 开始时刻成败尚未发生，success / error_type / error_code 三个 tag 走占位符。
	EmitStepStarted(tags StepEventTags)
	// EmitStepFinished 阶段结束事件，counter + duration timer。
	//
	// success 由上报侧给出（runtime 侧按 trial_status 推导），服务端不做推导——那条规则属于
	// runtime 的领域语义。durationMS 也取上报侧的值：服务端只知道收到事件的时刻，算不出阶段
	// 何时开始，用接收时刻现算等于把网络抖动和排队时间算进阶段耗时。
	EmitStepFinished(tags StepEventTags, success bool, errorCode int32, durationMS int64)
}
