// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestWindowsSandboxDefaultOperatorRejects(t *testing.T) {
	svc, idgen, _, _ := newSandboxAgentSvc(t)
	idgen.EXPECT().GenID(gomock.Any()).Return(int64(42), nil).AnyTimes()
	id, _, _, err := svc.AsyncExecute(context.Background(), 7, &entity.ExecuteEvalTargetParam{EvalTarget: &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{SandboxAgent: &entity.SandboxAgent{SandboxCountMode: "windows_plus_sandbox"}}}})
	require.Error(t, err)
	require.Zero(t, id)
}

func TestWindowsSandboxDestroyGroups(t *testing.T) {
	for _, zombie := range []bool{false, true} {
		t.Run(map[bool]string{false: "completion or cancellation", true: "timeout"}[zombie], func(t *testing.T) {
			requests := make(chan *rpc.SandboxDestroyRequest, 2)
			sched := rpcmocks.NewMockISandboxSchedulerAdapter(gomock.NewController(t))
			sched.EXPECT().Destroy(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(ctx context.Context, req *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
				requests <- req
				return &rpc.SandboxDestroyResponse{}, nil
			})
			svc := &EvalTargetServiceImpl{sandboxSchedulerAdapter: sched}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			svc.destroySandboxExecute(ctx, "42", 7, []string{"invoke-orch", "invoke-windows"}, zombie)
			got := map[string][]string{}
			for i := 0; i < 2; i++ {
				select {
				case req := <-requests:
					got[req.TaskID] = req.ExecuteIDs
					require.Equal(t, zombie, req.ZombieTimeout)
				case <-time.After(time.Second):
					t.Fatalf("missing destroy group: %v", got)
				}
			}
			require.Equal(t, map[string][]string{"42": {"invoke-orch"}, "42-windows": {"invoke-windows"}}, got)
		})
	}
	require.Equal(t, "windows", sideForExecuteID("42-windows", 42))
}
