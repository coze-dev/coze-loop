// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "strings"

// ExptDispatchMode 实验的 item 派发模式，对应 experiment.scheduler_mode 列。
//
// 该列是唯一权威源：创建实验时按灰度白名单一次性写入并冻结，Run / Retry / MQ consumer 一律回查此列裁决，
// 灰度配置热变更只影响后续新建实验，不得让存量实验在中心调度与旧 daemon 之间切换 —— 否则同一 run 会
// 出现两个派发驱动，既绕过全局优先级也绕过额度账本。
//
// 命名注意：本类型与 ExptSchedulerMode（实验跑法调度器 interface，见 expt_run.go）是完全不同的概念。
// 前者回答「谁来派发这个实验的 item」，后者回答「这个实验按什么跑法执行」。
type ExptDispatchMode = string

const (
	// ExptDispatchModeLegacy 旧链路：每实验一条 MQ 自循环，按配置并发自主补 item，受空间运行实验数闸约束。
	// 历史数据与非灰度空间的新实验都是该值（DB 列默认值）。
	ExptDispatchModeLegacy ExptDispatchMode = "legacy"
	// ExptDispatchModeEnforce 中心调度：由中心调度器按全局优先级 + 资源额度决定派发，跳过空间运行实验数闸。
	// 旧 per-experiment tick 对该模式实验只做初始化与生命周期维护，不得启动新 item。
	ExptDispatchModeEnforce ExptDispatchMode = "enforce"
)

const (
	// DefaultExptPriorityLevel 未申报优先级时的缺省值，与 DB 列默认值一致。
	DefaultExptPriorityLevel int32 = 1
	// MinExptPriorityLevel / MaxExptPriorityLevel 优先级合法区间（闭区间），数值越大越优先。
	MinExptPriorityLevel int32 = 1
	MaxExptPriorityLevel int32 = 99
)

// IsValidExptDispatchMode 判断落库前的模式取值是否合法。空串视为非法：
// 写入路径必须显式给出 legacy 或 enforce，避免把空值写进 NOT NULL 列后再靠 DB 默认值兜底。
func IsValidExptDispatchMode(mode ExptDispatchMode) bool {
	switch mode {
	case ExptDispatchModeLegacy, ExptDispatchModeEnforce:
		return true
	default:
		return false
	}
}

// IsCentralDispatch 判断该实验是否由中心调度器派发 item。
// 读取侧统一走本函数，不要散落 == "enforce" 的字面比较。
func IsCentralDispatch(mode ExptDispatchMode) bool {
	return mode == ExptDispatchModeEnforce
}

// NormalizeExptDispatchMode 把历史/异常取值收敛为 legacy。
// 用于读路径：老数据该列虽有 NOT NULL DEFAULT 'legacy'，但 PO2DO 之外的构造路径可能留空，
// 而「未知模式」按 legacy 处理是安全侧 —— 最坏是走旧链路，不会让实验绕过额度账本执行。
func NormalizeExptDispatchMode(mode ExptDispatchMode) ExptDispatchMode {
	if IsValidExptDispatchMode(mode) {
		return mode
	}
	return ExptDispatchModeLegacy
}

// NormalizeExptPriorityLevel 把越界/未设置的优先级收敛到合法区间。
// 0 值（未申报）收敛为缺省 1；越界值按边界截断而非报错，因为读路径不应因历史脏数据中断调度。
// 写入路径的合法性校验在 application 层做，会显式返回参数错误。
func NormalizeExptPriorityLevel(priority int32) int32 {
	return NormalizeExptPriorityLevelWithDefault(priority, 0)
}

// NormalizeExptPriorityLevelWithDefault 同上，但允许调用方指定"未申报时用哪个缺省值"。
//
// 存在意义：缺省优先级是可运维配置的（commercial 的
// `central_expt_scheduler_space_config.default_priority`），而这个收敛函数在 OSS entity 层、
// 拿不到那份配置。让调用方把值传进来，避免在 entity 层反向依赖配置中心。
//
// defaultPriority 的三种取值都收敛到安全行为：
//
//	0 或越界   —— 视为"没有意见"，回落到 DefaultExptPriorityLevel（=1），
//	              这同时覆盖了"TCC 里没配这个字段"与 noop policy 两种情况
//	1-99      —— 采纳
//
// 为什么越界的 defaultPriority 也回落到 1 而不是截断到 99：它来自人工维护的配置，
// 配成 999 更可能是笔误而非"想要最高优先级"，而截断到 99 会让一次笔误静默变成
// "该空间所有实验都最高优"，那是最难发现的一类事故。
func NormalizeExptPriorityLevelWithDefault(priority, defaultPriority int32) int32 {
	fallback := DefaultExptPriorityLevel
	if defaultPriority >= MinExptPriorityLevel && defaultPriority <= MaxExptPriorityLevel {
		fallback = defaultPriority
	}

	switch {
	case priority < MinExptPriorityLevel:
		return fallback
	case priority > MaxExptPriorityLevel:
		return MaxExptPriorityLevel
	default:
		return priority
	}
}

// ExptTriggerTypeEvalx 是 EvalX 平台发起实验时携带的 trigger_type，与 IDL 常量
// `const ExptTriggerType Evalx = "evalx"` 一致。
//
// 单独在 entity 层再声明一次而不 import kitex_gen：domain 层不依赖生成代码是本仓库的
// 分层约束；两处值必须一致，由 TestExptTriggerTypeEvalxMatchesIDL 守住。
const ExptTriggerTypeEvalx = "evalx"

// ShouldEnforceByTrigger 判断某 trigger 来源的新实验是否应写 enforce。
//
// 本期规则：只有 EvalX 发起的实验进入中心调度。EvalX 是内部平台，会按约定携带
// priority 与 expected_quota_consumption；其它入口（控制台手动、OpenAPI、定时任务）
// 一律 legacy，行为与引入中心调度前完全一致。
//
// 为什么按 trigger 而不是按空间白名单：空间白名单会把同一空间里"人手点的实验"也拽进
// enforce，而那些实验不会申报资源消耗向量 —— 没有向量就无法预占，调度器只能跳过，
// 表现为"实验建好了但一个 item 都不跑"。按来源区分则天然对齐"谁申报、谁被管控"。
//
// 大小写与空白容忍：trigger_type 是跨系统传递的字符串字段，上游拼装方式不受本仓库控制。
func ShouldEnforceByTrigger(triggerType string) bool {
	return strings.EqualFold(strings.TrimSpace(triggerType), ExptTriggerTypeEvalx)
}
