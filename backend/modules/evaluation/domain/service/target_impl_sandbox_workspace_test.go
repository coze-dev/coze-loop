// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	errno "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	errorx "github.com/coze-dev/coze-loop/backend/pkg/errorx"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

func TestSandboxRecordSchedulerWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name string
		ext  map[string]string
		want int64
	}{
		{"shared experiment", map[string]string{consts.OutputDataExtKeySandboxWorkspaceID: "2"}, 2},
		{"debug", map[string]string{consts.OutputDataExtKeySandboxWorkspaceID: "1"}, 1},
		{"legacy", nil, 1},
		{"invalid metadata", map[string]string{consts.OutputDataExtKeySandboxWorkspaceID: "invalid"}, 1},
	} {
		for _, operation := range []string{"callback", "terminate", "zombie", "patrol"} {
			t.Run(tc.name+"/"+operation, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				repo := repomocks.NewMockIEvalTargetRepo(ctrl)
				sched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
				runLog := repomocks.NewMockIExptRunLogRepo(ctrl)
				svc := &EvalTargetServiceImpl{evalTargetRepo: repo, sandboxSchedulerAdapter: sched, exptRunLogRepo: runLog}
				ext := make(map[string]string)
				for k, v := range tc.ext {
					ext[k] = v
				}
				record := &entity.EvalTargetRecord{
					ID: 10, SpaceID: 1, TargetVersionID: 20, ExperimentRunID: 30,
					Status:               gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
					EvalTargetOutputData: &entity.EvalTargetOutputData{Ext: ext},
				}
				version := &entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{ID: 20}}
				if operation == "callback" {
					repo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(gomock.Any(), int64(1), int64(10)).Return(record, nil)
					repo.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(1), int64(20)).Return(version, nil)
				} else {
					repo.EXPECT().ListEvalTargetRecordByIDsAndSpaceID(gomock.Any(), int64(1), []int64{10}).Return([]*entity.EvalTargetRecord{record}, nil)
					repo.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), int64(1), []int64{20}).Return([]*entity.EvalTarget{version}, nil)
				}
				if operation == "patrol" {
					sched.EXPECT().Get(gomock.Any(), &rpc.SandboxGetRequest{ExecuteID: "10", WorkspaceID: tc.want}).Return(&rpc.SandboxGetResponse{ExecuteInfo: &rpc.SandboxExecuteInfo{Status: rpc.SandboxExecuteStatusFailed}}, nil)
					terminated, _ := svc.CheckSandboxTerminated(context.Background(), 1, []int64{10})
					assert.Equal(t, []int64{10}, terminated)
					return
				}
				runLog.EXPECT().Get(gomock.Any(), int64(0), int64(30)).Return(&entity.ExptRunLog{ExptID: 40}, nil)
				repo.EXPECT().SaveEvalTargetRecord(gomock.Any(), gomock.Any(), gomock.Nil()).DoAndReturn(func(_ context.Context, r *entity.EvalTargetRecord, _ *bool) error {
					assert.Equal(t, int64(1), r.SpaceID)
					assert.Equal(t, tc.ext[consts.OutputDataExtKeySandboxWorkspaceID], r.EvalTargetOutputData.Ext[consts.OutputDataExtKeySandboxWorkspaceID])
					return nil
				})
				destroyed := make(chan *rpc.SandboxDestroyRequest, 1)
				sched.EXPECT().Destroy(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
					destroyed <- req
					return &rpc.SandboxDestroyResponse{}, nil
				})
				if operation == "callback" {
					require.NoError(t, svc.ReportInvokeRecords(context.Background(), &entity.ReportTargetRecordParam{
						SpaceID: 1, RecordID: 10, Status: entity.EvalTargetRunStatusSuccess, EnableExtractTrajectory: gptr.Of(false),
						OutputData: &entity.EvalTargetOutputData{Ext: map[string]string{consts.OutputDataExtKeySandboxWorkspaceID: "999"}},
					}))
				} else {
					require.NoError(t, svc.TerminateAsyncRecordsAndDestroySandbox(context.Background(), 1, []int64{10}, 1, "stopped", operation == "zombie", operation == "terminate"))
				}
				select {
				case req := <-destroyed:
					assert.Equal(t, tc.want, req.WorkspaceID)
					assert.Equal(t, int64(1), req.SourceWorkspaceID)
					assert.Equal(t, "40", req.TaskID)
					assert.Equal(t, operation == "zombie", req.ZombieTimeout)
				case <-time.After(3 * time.Second):
					t.Fatal("sandbox was not destroyed")
				}
			})
		}
	}
}

func TestAsyncExecuteTarget_CreateRecordFail_ExperimentWorkspace(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, target, input, got, mu, done := createRecordFailFixture(t, ctrl, entity.EvalTargetTypeSandboxAgent, nil)
	store := svc.evalTargetRepo.(*repomocks.MockIEvalTargetRepo)
	store.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(2), target.EvalTargetVersion.ID).Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))
	store.EXPECT().GetEvalTarget(gomock.Any(), target.ID).Return(target, nil)

	_, _, err := svc.asyncExecuteTarget(context.Background(), target.SpaceID, target, &entity.ExecuteTargetCtx{
		ExperimentID: gptr.Of(int64(40)), ExptSpaceID: 2,
	}, input)
	require.Error(t, err)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("sandbox was not destroyed")
	}
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, *got, 1)
	assert.Equal(t, int64(2), (*got)[0].WorkspaceID)
	assert.Equal(t, target.SpaceID, (*got)[0].SourceWorkspaceID)
}
