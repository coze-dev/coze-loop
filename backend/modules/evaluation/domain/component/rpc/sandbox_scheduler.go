// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
)

// ISandboxSchedulerAdapter 沙箱调度 RPC 适配器。
//
// 对应 idl: cozeloop-idl-commercial/saas/coze/loop/agent_studio/coze.loop.agent_studio.sandbox_scheduler.thrift
// 目标 PSM: stone.cozeloop.agent_studio
//
//go:generate mockgen -destination=./mocks/sandbox_scheduler.go -package=mocks . ISandboxSchedulerAdapter
type ISandboxSchedulerAdapter interface {
	// Init 初始化一个沙箱任务。
	Init(ctx context.Context, req *SandboxInitRequest) (*SandboxInitResponse, error)
	// Run 提交一次执行。
	Run(ctx context.Context, req *SandboxRunRequest) (*SandboxRunResponse, error)
	// Get 查询单次执行状态。
	Get(ctx context.Context, req *SandboxGetRequest) (*SandboxGetResponse, error)
	// GetTaskInfo 查询任务整体状态。
	GetTaskInfo(ctx context.Context, req *SandboxGetTaskInfoRequest) (*SandboxGetTaskInfoResponse, error)
	// Destroy 销毁任务或指定执行。
	Destroy(ctx context.Context, req *SandboxDestroyRequest) (*SandboxDestroyResponse, error)
}

// ---------- 枚举 ----------

// SandboxExecuteStatus 执行状态。
type SandboxExecuteStatus int32

const (
	SandboxExecuteStatusPending   SandboxExecuteStatus = 0
	SandboxExecuteStatusCreating  SandboxExecuteStatus = 1
	SandboxExecuteStatusRunning   SandboxExecuteStatus = 2
	SandboxExecuteStatusSucceeded SandboxExecuteStatus = 10
	SandboxExecuteStatusFailed    SandboxExecuteStatus = 11
	SandboxExecuteStatusCanceled  SandboxExecuteStatus = 12
)

// SandboxDestroyType 销毁类型。
type SandboxDestroyType int32

const (
	SandboxDestroyTypeCancel    SandboxDestroyType = 0
	SandboxDestroyTypeCompleted SandboxDestroyType = 1
	SandboxDestroyTypeFailed    SandboxDestroyType = 2
)

// ---------- Domain ----------

// SandboxExecuteError 单次执行错误信息。
type SandboxExecuteError struct {
	Code    string
	Message string
}

// SandboxExecuteInfo 单次执行详情。
type SandboxExecuteInfo struct {
	ExecuteID     string
	TaskID        string
	Status        SandboxExecuteStatus
	SessionID     string
	EnqueueTimeMS int64
	StartTimeMS   int64
	EndTimeMS     int64
	Error         *SandboxExecuteError
	Param         map[string]string
	QueuePosition int32
}

// SandboxTaskInfo 任务整体状态。
type SandboxTaskInfo struct {
	TaskID         string
	Concurrency    int32
	ActiveCount    int32
	PendingCount   int32
	TotalCount     int32
	CompletedCount int32
}

// ---------- Requests / Responses ----------

// SandboxInitRequest 初始化任务请求。
type SandboxInitRequest struct {
	TaskID      string
	Concurrency int32
	Metadata    map[string]string
	WorkspaceID int64
}

// SandboxInitResponse 初始化任务响应。
type SandboxInitResponse struct {
	TaskInfo *SandboxTaskInfo
}

// SandboxRunRequest 提交一次执行请求。
type SandboxRunRequest struct {
	ExecuteID   string
	TaskID      string
	Param       map[string]string
	WorkspaceID int64
}

// SandboxRunResponse 提交一次执行响应。
type SandboxRunResponse struct {
	ExecuteID string
}

// SandboxGetRequest 查询执行请求。
type SandboxGetRequest struct {
	ExecuteID   string
	WorkspaceID int64
}

// SandboxGetResponse 查询执行响应。
type SandboxGetResponse struct {
	ExecuteInfo *SandboxExecuteInfo
}

// SandboxGetTaskInfoRequest 查询任务请求。
type SandboxGetTaskInfoRequest struct {
	TaskID      string
	WorkspaceID int64
}

// SandboxGetTaskInfoResponse 查询任务响应。
type SandboxGetTaskInfoResponse struct {
	TaskInfo *SandboxTaskInfo
}

// SandboxDestroyRequest 销毁任务/执行请求。
type SandboxDestroyRequest struct {
	TaskID      string
	DestroyType SandboxDestroyType
	ExecuteIDs  []string
	WorkspaceID int64
}

// SandboxDestroyResponse 销毁响应。
type SandboxDestroyResponse struct {
	AffectedCount int32
}
