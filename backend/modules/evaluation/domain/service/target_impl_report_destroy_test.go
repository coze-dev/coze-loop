// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// TestReportInvokeRecords_DestroyUsesOperatorExecuteIDs 钉住「双沙箱 item 正常跑完后，销毁请求必须
// 带上 operator 回传的带后缀 execute id」这条端到端契约。
//
// 线上实测的故障（expt 7590114891010282754 / invoke 7590114876341516034）：item success 后
// evaluation 发出的 Destroy 请求 ExecuteIDs=["7590114876341516034"]（裸 invokeID），而实际创建的是
// ["<invokeID>-agent","<invokeID>-orch"]。agent_studio 侧 cancelExecutions 逐个 execRepo.Get(id)
// 查不到该 id 就 continue（静默跳过，affected=0）→ 不 DecrActive、不销毁 session，于是：
//   - task 的 active_count 永不归还，`WHERE active_count < N` 永久为假，后续 item 全部
//     601300702 concurrency limit reached（实测 medium 53/59 全挂在此）；
//   - 两个沙箱 session 无限存活（实测 13 小时后仍可 attach、runner /healthz 仍 200）。
//
// 本用例只断言平台侧的可控部分：ReportInvokeRecords 之后传给 Destroy 的 ExecuteIDs 必须是 ext 里
// operator 回传的那两个带后缀 id，且 TaskID 是 exptID。裸 invokeID 即回归。
func TestReportInvokeRecords_DestroyUsesOperatorExecuteIDs(t *testing.T) {
	t.Parallel()

	const (
		invokeID = int64(7590114876341516034)
		spaceID  = int64(7590103974980812802)
		versionID = int64(7590114891010282498)
		runID    = int64(7590114891010283266)
		exptID   = int64(7590114891010282754)
	)
	wantIDs := []string{"7590114876341516034-agent", "7590114876341516034-orch"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
	mockSched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
	mockRunLog := repomocks.NewMockIExptRunLogRepo(ctrl)

	svc := &EvalTargetServiceImpl{
		evalTargetRepo:          mockRepo,
		sandboxSchedulerAdapter: mockSched,
		exptRunLogRepo:          mockRunLog,
	}

	// AsyncExecute 阶段 operator 把真实 execute id 列表落在了 output ext 上（线上 record 确实有这份数据）。
	stored := &entity.EvalTargetRecord{
		ID:              invokeID,
		SpaceID:         spaceID,
		TargetVersionID: versionID,
		ExperimentRunID: runID,
		Status:          gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			Ext: map[string]string{
				consts.OutputDataExtKeySandboxExecuteIDs: `["7590114876341516034-agent","7590114876341516034-orch"]`,
			},
		},
	}
	mockRepo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(gomock.Any(), spaceID, invokeID).Return(stored, nil)
	mockRepo.EXPECT().SaveEvalTargetRecord(gomock.Any(), gomock.Any(), gomock.Nil()).Return(nil)

	// destroySandboxExecuteIfNeeded 的类型门禁：必须是 SandboxAgent 才销毁。
	mockRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), spaceID, versionID).
		Return(&entity.EvalTarget{
			EvalTargetType:    entity.EvalTargetTypeSandboxAgent,
			EvalTargetVersion: &entity.EvalTargetVersion{ID: versionID, EvalTargetType: entity.EvalTargetTypeSandboxAgent},
		}, nil)
	mockRunLog.EXPECT().Get(gomock.Any(), int64(0), runID).Return(&entity.ExptRunLog{ExptID: exptID}, nil)

	destroyed := make(chan *rpc.SandboxDestroyRequest, 1)
	mockSched.EXPECT().Destroy(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
			destroyed <- req
			return &rpc.SandboxDestroyResponse{}, nil
		}).Times(1)

	// 回调上报的 OutputData 不含 ext —— 这正是线上形态（sandbox 只回传结果字段）。
	err := svc.ReportInvokeRecords(context.Background(), &entity.ReportTargetRecordParam{
		SpaceID:    spaceID,
		RecordID:   invokeID,
		OutputData: &entity.EvalTargetOutputData{},
		Status:     entity.EvalTargetRunStatusSuccess,
	})
	assert.NoError(t, err)

	select {
	case req := <-destroyed:
		assert.Equal(t, wantIDs, req.ExecuteIDs,
			"Destroy 必须带 operator 回传的带后缀 execute id；裸 invokeID 会让 agent_studio 查不到 execution "+
				"→ 静默跳过、名额不归还、沙箱泄漏")
		assert.Equal(t, "7590114891010282754", req.TaskID)
		assert.Equal(t, rpc.SandboxDestroyTypeExecute, req.DestroyType)
		assert.Equal(t, spaceID, req.WorkspaceID)
	case <-time.After(3 * time.Second):
		t.Fatal("destroy goroutine timeout: ReportInvokeRecords 没有发出销毁请求")
	}
}
