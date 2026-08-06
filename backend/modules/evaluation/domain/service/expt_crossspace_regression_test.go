// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// TestBackfillRefEvalSetSourceSpace_MixedSpaceMultiSet 钉住调度侧(重试链路)的混合空间多集回归。
// expt_item_ref 表未存来源空间, 重试时靠本函数从 EvalSetConfigs 反推。
// 关键: SourceSpaceID==0 是"该集在调用方空间"的合法冻结值, 必须入 map;
// 若只收 >0, 同空间集会命中不到 map → 回退被 configs[0] 兜底回填的 expt.EvalSetSpaceID
// (=主集来源空间) → 用主集来源空间读同空间评测集 → 601103001, 重试永远救不回来。
// (entity 侧由 TestExptItemEvalCtx_SourceSpaceIDs_MixedSpaceMultiSet 钉住, 这里补调度侧。)
func TestBackfillRefEvalSetSourceSpace_MixedSpaceMultiSet(t *testing.T) {
	const (
		srcSpaceB = int64(9001) // 主集(configs[0]) 来源空间, 已被兜底回填进顶层冻结列
		crossSet  = int64(10)   // 跨空间集
		sameSet   = int64(20)   // 同空间集 (SourceSpaceID=0)
	)

	expt := &entity.Experiment{
		EvalSetSpaceID:    srcSpaceB, // 顶层冻结列 = 主集来源空间
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{EvalSetID: crossSet, SourceSpaceID: srcSpaceB},
				{EvalSetID: sameSet}, // SourceSpaceID = 0, 合法: 该集在调用方空间
			},
		},
	}
	refs := []*entity.ExptItemRef{
		{ItemID: 1, EvalSetID: crossSet},
		{ItemID: 2, EvalSetID: sameSet},
		nil, // 不得 panic
	}

	backfillRefEvalSetSourceSpace(refs, expt)

	assert.Equal(t, srcSpaceB, refs[0].EvalSetSourceSpaceID, "跨空间集应取其来源空间")
	assert.Zero(t, refs[1].EvalSetSourceSpaceID, "同空间集须保持 0, 不得回退到主集来源空间")
}

// TestBackfillRefEvalSetSourceSpace_SingleSetFallback 单集/老实验(EvalSetConfigs 未覆盖该集):
// 回退实验级冻结列, 行为与改动前一致。
func TestBackfillRefEvalSetSourceSpace_SingleSetFallback(t *testing.T) {
	const srcSpaceB = int64(9001)

	expt := &entity.Experiment{EvalSetSpaceID: srcSpaceB} // EvalConf 为 nil
	refs := []*entity.ExptItemRef{{ItemID: 1, EvalSetID: 10}}

	backfillRefEvalSetSourceSpace(refs, expt)
	assert.Equal(t, srcSpaceB, refs[0].EvalSetSourceSpaceID)

	// expt 为 nil 时直接返回, 不得 panic / 不改 refs
	refs2 := []*entity.ExptItemRef{{ItemID: 1, EvalSetID: 10, EvalSetSourceSpaceID: 123}}
	backfillRefEvalSetSourceSpace(refs2, nil)
	assert.Equal(t, int64(123), refs2[0].EvalSetSourceSpaceID)
}

// TestCompleteItemRunOnUnretriableErr 钉住"失败且不重试"的兜底落库。
// 目标场景是 BuildExptRecordEvalCtx 等前置阶段失败(此时 PreEval 未执行、无 turn run log):
// 必须同时落 item run log(Fail) 与 turn run log(Fail) —— 只落前者会让 item 变 Fail+Logged
// 后被判 complete, 而 RecordItemRunLogs 因 turn run log 缺失报 "null turn log result",
// 重试 5min 后整实验被判 Failed(单 item 失败放大成整实验失败, 比原先卡 Processing 更糟)。
func TestCompleteItemRunOnUnretriableErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo}

	event := &entity.ExptItemEvalEvent{
		SpaceID:       7,
		ExptID:        1001,
		ExptRunID:     2002,
		EvalSetItemID: 3003,
	}
	evalErr := errors.New("resource not found, get dataset_version 123")

	itemRepo.EXPECT().
		UpdateItemRunLog(gomock.Any(), int64(1001), int64(2002), []int64{3003}, gomock.Any(), int64(7)).
		DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any, _ int64) error {
			// 字段须与 CompleteItemRun 失败分支一致
			assert.Equal(t, int32(entity.ItemRunState_Fail), ufields["status"])
			assert.Equal(t, int32(entity.ExptItemResultStateLogged), ufields["result_state"])
			assert.NotEmpty(t, ufields["err_msg"])
			return nil
		})
	// 必须同步补建/更新 turn run log 为 Fail (与僵尸清理路径同款)
	turnRepo.EXPECT().
		CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), int64(7), int64(1001), int64(2002), []int64{3003}, entity.TurnRunState_Fail).
		Return(nil)

	svc.completeItemRunOnUnretriableErr(context.Background(), event, evalErr)
}

// TestCompleteItemRunOnUnretriableErr_NoopAndTolerant 不该落库的输入不得触发写库;
// 写库失败只告警、不 panic (僵尸清理仍是最后防线)。
func TestCompleteItemRunOnUnretriableErr_NoopAndTolerant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl) // 以下三种输入均不应被调用
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo}

	event := &entity.ExptItemEvalEvent{SpaceID: 7, ExptID: 1, ExptRunID: 2, EvalSetItemID: 3}
	svc.completeItemRunOnUnretriableErr(context.Background(), event, nil) // evalErr 为 nil
	svc.completeItemRunOnUnretriableErr(context.Background(), nil, errors.New("x"))

	// repo 为 nil 时不得 panic
	(&ExptItemEventEvalServiceImpl{}).completeItemRunOnUnretriableErr(context.Background(), event, errors.New("x"))

	// 两次写库都失败时只告警, 不 panic
	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()
	itemRepo2 := repoMocks.NewMockIExptItemResultRepo(ctrl2)
	turnRepo2 := repoMocks.NewMockIExptTurnResultRepo(ctrl2)
	itemRepo2.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("db down"))
	turnRepo2.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("db down"))
	svc2 := &ExptItemEventEvalServiceImpl{exptItemResultRepo: itemRepo2, exptTurnResultRepo: turnRepo2}
	svc2.completeItemRunOnUnretriableErr(context.Background(), event, errors.New("boom"))
}
