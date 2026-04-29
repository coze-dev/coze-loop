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
	Cluster    *string // 执行集群
}
