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
	// WriteFile 向某个 Running execution 的 session 写入文件（批量）。
	// 用于双沙箱 bring-up：session 建好后把补全 endpoint 的 case-file 写进 Orchestrator 沙箱。
	WriteFile(ctx context.Context, req *SandboxWriteFileRequest) (*SandboxWriteFileResponse, error)
	// RunCommand 在某个 Running execution 的 session 内执行命令。
	// 用于双沙箱 bring-up：在 Orchestrator 沙箱内启动 `fornax-eval orchestrator`。
	RunCommand(ctx context.Context, req *SandboxRunCommandRequest) (*SandboxRunCommandResponse, error)
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
	SandboxExecuteStatusFinished  SandboxExecuteStatus = 13
)

// SandboxDestroyType 销毁类型。
type SandboxDestroyType int32

const (
	SandboxDestroyTypeTask    SandboxDestroyType = 0
	SandboxDestroyTypeExecute SandboxDestroyType = 1
)

// SandboxTenant 沙箱租户，与 stone.cozeloop.agent_studio 的 sandbox_scheduler.Tenant 枚举保持数值一致。
// 决定容器 start_cmd / env 注入 / case-file 等能力开关，Init 时下发，一个 task 只能配置一次。
type SandboxTenant int32

const (
	// SandboxTenantDefault = FornaxTraeEval，兼容单沙箱链路的默认租户。
	SandboxTenantDefault SandboxTenant = 0
	// SandboxTenantGeneralAgent 通用 agent 场景。
	SandboxTenantGeneralAgent SandboxTenant = 1
	// SandboxTenantLabelingAnalysis 标注 / 分析场景。
	SandboxTenantLabelingAnalysis SandboxTenant = 2
	// SandboxTenantFornaxTraeEvalDualSandbox = FornaxTraeEvalDoubleSandbox，双沙箱模式。
	SandboxTenantFornaxTraeEvalDualSandbox SandboxTenant = 3
	// SandboxTenantFornaxEvalGeneral = FornaxEvalGeneral，通用评测租户（能力同 GeneralAgent 的最小集：
	// 不注入平台 env、不写 case-file、无默认 start_cmd，全部由调用方显式下发）。
	// 双沙箱链路现用此租户：编排完全由 operator 侧完成，调度侧只需提供裸沙箱。
	SandboxTenantFornaxEvalGeneral SandboxTenant = 4
	// SandboxTenantFornaxEvalGeneralGUI = FornaxEvalGeneralGUI，Mac VM GUI 评测专用租户（能力同 GeneralAgent 最小集）。
	// mac_vm_plus_sandbox 链路的两个 task（mac_vm + sandbox）共用此租户，靠 ResourceType 区分落到哪种 backend。
	SandboxTenantFornaxEvalGeneralGUI SandboxTenant = 5
)

// SandboxResourceType 标识 task 落到哪种计算资源，与 stone.cozeloop.agent_studio 的
// sandbox_scheduler.ResourceType 枚举保持数值一致。与 SandboxTenant 正交：Tenant 管能力门控/配额，
// ResourceType 管路由到哪种 backend。task 级属性，Init 时下发，同一 task 只能配置一次；
// per-op（Run/RunCommand/WriteFile/Destroy）不携带，调度侧从 task 反查。
type SandboxResourceType int32

const (
	// SandboxResourceTypeSandbox = sandbox(0)，默认，Linux 沙箱；未显式设置即此值，保持原行为不变。
	SandboxResourceTypeSandbox SandboxResourceType = 0
	// SandboxResourceTypeMacVM = mac_vm(1)，从 warm pool 租借的 Mac VM。
	SandboxResourceTypeMacVM SandboxResourceType = 1
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
	// ResourceType 该 task 落到的计算资源类型（调度侧回显）；缺省为 sandbox。
	ResourceType SandboxResourceType
}

// ---------- Requests / Responses ----------

// SandboxInitRequest 初始化任务请求。
type SandboxInitRequest struct {
	TaskID      string
	Concurrency int32
	Metadata    map[string]string
	WorkspaceID int64
	// Tenant 沙箱租户；未显式设置（值为 SandboxTenantDefault）时沿用调度侧默认租户 FornaxTraeEval。
	// 双沙箱模式的评测对象必须传 SandboxTenantFornaxEvalGeneral。
	Tenant SandboxTenant
	// ResourceType 计算资源类型；未显式设置（值为 SandboxResourceTypeSandbox）时落 Linux 沙箱，保持原行为。
	// mac_vm + sandbox 双资源实验中，两个 task 用同一租户、靠本字段区分落到哪种 backend。
	ResourceType SandboxResourceType
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
	// StartCmd 覆盖沙箱容器启动后执行的默认命令；仅在租户开启 start_cmd 能力时生效。
	StartCmd string
	// Env 会与租户策略注入的环境变量合并；同名 key 以调用方为准（含 FORNAX_* 前缀）。
	Env map[string]string
	// Sync 为 true 时同步等待 session 创建完成，返回 SessionID；默认异步返回。
	Sync bool
	// Files 在 session 创建后、StartCmd 执行前写入容器内的文件。
	Files []*SandboxFileWrite
}

// SandboxFileWrite 单个待写入沙箱容器的文件。
type SandboxFileWrite struct {
	// Path 容器内绝对路径。
	Path string
	// Content UTF-8 文本；IsBase64=true 时按 base64 解码后写入。
	Content string
	// IsBase64 表示 Content 是 base64 编码的二进制内容。
	IsBase64 bool
}

// SandboxRunResponse 提交一次执行响应。
type SandboxRunResponse struct {
	ExecuteID string
	// SessionID 仅在 SandboxRunRequest.Sync=true 时返回；异步模式下 session 未创建，值为空。
	SessionID string
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
	// ZombieTimeout 标记本次销毁是否由 SandboxAgent zombie 超时触发；
	// 具体收尾命令的拼接由调用侧适配器实现，backend 不感知命令内容。
	ZombieTimeout bool
}

// SandboxDestroyResponse 销毁响应。
type SandboxDestroyResponse struct {
	AffectedCount int32
}

// SandboxWriteFileRequest 向某个 Running execution 的 session 写文件（批量）。
type SandboxWriteFileRequest struct {
	ExecuteID   string
	WorkspaceID int64
	Files       []*SandboxFileWrite
}

// SandboxWriteFileResponse 写文件响应。
type SandboxWriteFileResponse struct {
	// TotalBytesWritten 所有文件写入字节数之和。
	TotalBytesWritten int64
	// Results 每个文件的写入结果，与入参顺序一致。
	Results []*SandboxFileWriteResult
}

// SandboxFileWriteResult 单个文件的写入结果。
type SandboxFileWriteResult struct {
	Path         string
	BytesWritten int64
}

// SandboxRunCommandRequest 在某个 Running execution 的 session 内执行命令。
type SandboxRunCommandRequest struct {
	ExecuteID   string
	WorkspaceID int64
	// Command 命令行，经 bash -lc 执行。
	Command string
	// Cwd 工作目录，默认 "/"。
	Cwd string
	// TimeoutMS 执行超时（毫秒），服务端会封顶。
	TimeoutMS int64
	// Async 为 true 时后台执行、立即返回（不回 stdout/stderr）；默认同步。
	Async bool
	// Env 注入被执行命令的环境变量（不上命令行）。mac_vm 场景下 runner 拉起靠它注入
	// FORNAX_ORCHESTRATOR_WS_LISTEN / FORNAX_RUNNER_SESSION_ID 等及凭据；调度侧经 guest-exec env 数组下发。
	Env map[string]string
}

// SandboxRunCommandResponse 执行命令响应。
type SandboxRunCommandResponse struct {
	// Stdout 标准输出（async 模式下为空）。
	Stdout string
	// Stderr 标准错误（async 模式下为空）。
	Stderr string
	// ExitCode 进程退出码（async 模式下 0/未设）。
	ExitCode int32
}
