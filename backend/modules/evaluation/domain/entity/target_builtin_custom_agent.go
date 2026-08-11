// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type CustomAgent struct {
	// 应用ID
	ID int64
	// DTO使用，不存数据库
	Name string `json:"-"`
	// DTO使用，不存数据库
	Description string `json:"-"`

	// 自定义输出结果
	CustomFieldSchemas []*CustomFieldSchema `json:"custom_field_schemas,omitempty"`

	ExecRegion          Region           // 执行区域
	ExecEnv             *string          // 执行环境
	Cluster             *string          // 执行集群
	TimeoutMs           *int64           // 超时时间，单位ms
	FirstTokenTimeoutMs *int64           // 首包超时时间，单位ms
	AgentConnection     *AgentConnection // 连接信息

	// EnableAnalysis 是否开启分析：创建评测对象时从 application.usages（含 "analysis"）反查固化，
	// 控制 item-complete MQ 是否发送（与 TCC 空间白名单 AND）。
	EnableAnalysis bool `json:"enable_analysis,omitempty"`
}

type AgentConnection struct {
	FrontierInfo    *FrontierInfo
	IP              string
	Region          string
	IDC             string
	SDKVersion      string
	ProtocolVersion string
	PSM             string
	AgentImpl       *AgentImpl
}

type FrontierInfo struct {
	AppID     int64 `json:"app_id"`
	ProductID int64 `json:"product_id"`
	UserID    int64 `json:"user_id"`
	DeviceID  int64 `json:"device_id"`
}

type AgentImpl struct {
	Language  string // go/python
	Framework string // Eino/Langchain
	Kind      string // 用户agent的具体实体类型标识
}
