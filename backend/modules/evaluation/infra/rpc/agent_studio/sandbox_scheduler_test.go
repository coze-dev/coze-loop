// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package agent_studio

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
)

func TestSandboxSchedulerAdapter_AllMethodsReturnNotImplemented(t *testing.T) {
	t.Parallel()

	a := NewSandboxSchedulerAdapter()
	assert.NotNil(t, a)

	ctx := context.Background()

	initResp, err := a.Init(ctx, &rpc.SandboxInitRequest{})
	assert.Nil(t, initResp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implement")

	runResp, err := a.Run(ctx, &rpc.SandboxRunRequest{})
	assert.Nil(t, runResp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implement")

	getResp, err := a.Get(ctx, &rpc.SandboxGetRequest{})
	assert.Nil(t, getResp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implement")

	taskResp, err := a.GetTaskInfo(ctx, &rpc.SandboxGetTaskInfoRequest{})
	assert.Nil(t, taskResp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implement")

	destroyResp, err := a.Destroy(ctx, &rpc.SandboxDestroyRequest{})
	assert.Nil(t, destroyResp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implement")
}
