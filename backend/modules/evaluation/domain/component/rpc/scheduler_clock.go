// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

// ISchedulerClock 让本进程所属调度域的周期性 tick 发生。
//
// 与 IExptScheduleAdapter 的分工：后者是「向外部平台注册一个 job」，它的 BizKey /
// CallbackMethod / CallbackPayload 都以「有另一个系统稍后回调我」为前提；本接口只承诺
// 「tick 会按周期发生」，不预设由谁触发。因此进程内 ticker（无 BizKey、无回调、直接函数调用）
// 与外部调度平台可以是同一个抽象下的两种实现，而不必让前者伪造后者的字段。
//
// 实现方负责决定周期来源、并发互斥与故障恢复；调用方只做 Start / Stop。
//
//go:generate mockgen -destination=mocks/scheduler_clock.go -package=mocks . ISchedulerClock
type ISchedulerClock interface {
	// Name 返回实现标识（如 "bytescheduler"、"ticker"），用于日志与可观测。
	// 排障时第一个要确认的就是「这个 pod 到底在用哪种时钟」。
	Name() string

	// Start 启动时钟，非阻塞。
	//
	// 外部触发型实现在此完成注册；进程内型实现在此启动后台循环。
	// 必须幂等：重复调用不产生第二个时钟。
	//
	// 返回 error 表示时钟未能启动（调用方应告警但通常不应阻止服务启动 ——
	// 缺少时钟只影响调度推进，不影响已在途的执行）。
	Start(ctx context.Context) error

	// Stop 停止时钟，用于优雅退出。已停止或从未启动时返回 nil。
	//
	// 注意：对外部触发型实现，Stop 不必等同于「注销远端 job」——
	// 是否保留远端 job 由实现自行决定（常驻 job 在无候选时空跑的代价通常可忽略，
	// 而反复注销/重建会引入 job 定义漂移的风险）。
	Stop(ctx context.Context) error
}
