// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package agent_studio

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// SandboxSchedulerAdapter 沙箱调度 RPC 适配器在开源 backend 中的占位实现。
//
// 真实调用 stone.cozeloop.agent_studio 的逻辑需要公司内部 kitex 生成代码，放在 commercial 仓库下覆盖。
type SandboxSchedulerAdapter struct{}

func NewSandboxSchedulerAdapter() *SandboxSchedulerAdapter {
	return &SandboxSchedulerAdapter{}
}

func (a *SandboxSchedulerAdapter) Init(ctx context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
	return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("SandboxScheduler.Init not implement"))
}

func (a *SandboxSchedulerAdapter) Run(ctx context.Context, req *rpc.SandboxRunRequest) (*rpc.SandboxRunResponse, error) {
	return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("SandboxScheduler.Run not implement"))
}

func (a *SandboxSchedulerAdapter) Get(ctx context.Context, req *rpc.SandboxGetRequest) (*rpc.SandboxGetResponse, error) {
	return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("SandboxScheduler.Get not implement"))
}

func (a *SandboxSchedulerAdapter) GetTaskInfo(ctx context.Context, req *rpc.SandboxGetTaskInfoRequest) (*rpc.SandboxGetTaskInfoResponse, error) {
	return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("SandboxScheduler.GetTaskInfo not implement"))
}

func (a *SandboxSchedulerAdapter) Destroy(ctx context.Context, req *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
	return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("SandboxScheduler.Destroy not implement"))
}
