// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type A2AAgent struct {
	// 应用ID
	ID int64
	// DTO使用，不存数据库
	Name string `json:"-"`
	// DTO使用，不存数据库
	Description string `json:"-"`

	ServerName string
	URL        string

	ExecRegion Region  // 执行区域
	ExecEnv    *string // 执行环境

	// EnableAnalysis 是否开启分析：创建评测对象时从 application.usages（含 "analysis"）反查固化，
	// 控制 item-complete MQ 是否发送（与 TCC 空间白名单 AND）。
	EnableAnalysis bool `json:"enable_analysis,omitempty"`
}
