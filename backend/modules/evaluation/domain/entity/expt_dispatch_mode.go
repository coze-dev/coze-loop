// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

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
	switch {
	case priority < MinExptPriorityLevel:
		return DefaultExptPriorityLevel
	case priority > MaxExptPriorityLevel:
		return MaxExptPriorityLevel
	default:
		return priority
	}
}
