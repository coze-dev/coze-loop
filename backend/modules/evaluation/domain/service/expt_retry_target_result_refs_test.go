// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// 重试重置回归: 三条 Single-set 重试路径以及批量失败重试, 都必须在 expt_turn_result 上
// 同时清 target_result_id=0 并把 expt_run_id 同步到新 run。否则 batch_get 兜底按
// tr.ExptRunID 分组去 turn_run_log 查不到新 run 的 target_result_id, 异步 target 阶段
// eval_target_record 无法出现在返回结果里 (残留旧记录或空)。
//
// MultiSet 分支走 resetRetryRunLogsForItems helper, 现有 MultiSet 集成测试
// (TestExptRetryAllExec_ExptStart_MultiSetConfig_ShouldUseItemRefs 等) 已对 map 内容做精确
// 断言, 本文件专门覆盖三条 Single-set 内联路径 + 批量失败重试。

// turnResetMapMatcher 断言重试传给 UpdateTurnResults 的 map 满足恢复不变量: 状态回到
// Queueing、清零 target_result_id、并把 expt_run_id 同步到新 run。
type turnResetMapMatcher struct{ wantRunID int64 }

func (m turnResetMapMatcher) Matches(x any) bool {
	got, ok := x.(map[string]any)
	if !ok {
		return false
	}
	if got["status"] != int32(entity.TurnRunState_Queueing) {
		return false
	}
	if got["target_result_id"] != int64(0) {
		return false
	}
	if got["expt_run_id"] != m.wantRunID {
		return false
	}
	return true
}

func (m turnResetMapMatcher) String() string {
	return "map{status:Queueing, target_result_id:0, expt_run_id:<new>}"
}

// TestExptFailRetryExec_ExptStart_ResetsTurnResultTargetAndRunID 覆盖批量失败重试:
// UpdateTurnResults 必须包含 target_result_id=0 + expt_run_id=NEW_run_id。
func TestExptFailRetryExec_ExptStart_ResetsTurnResultTargetAndRunID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	manager := svcmocks.NewMockIExptManager(ctrl)
	exptItemResultRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
	exptTurnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
	exptStatsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	exptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)

	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
	// 首页扫出两条待重试 turn, 第二页为空退出扫描循环。
	exptTurnResultRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), gomock.Any(), gomock.Any(), spaceID).Return([]*entity.ExptTurnResult{
		{ItemID: 100, TurnID: 1000, ItemVersionID: 10, Status: int32(entity.TurnRunState_Fail)},
		{ItemID: 200, TurnID: 2000, ItemVersionID: 20, Status: int32(entity.TurnRunState_Terminal)},
	}, int64(0), nil).Times(1)
	exptTurnResultRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), gomock.Any(), gomock.Any(), spaceID).Return([]*entity.ExptTurnResult{}, int64(0), nil).Times(1)

	idgen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{9001, 9002}, nil).Times(1)
	exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), map[string]any{
		"status":      int32(entity.ItemRunState_Queueing),
		"expt_run_id": newRunID,
	}).Return(nil).Times(1)

	// 核心断言: 重试重置 expt_turn_result 必须清 target_result_id 且同步到新 run_id。
	exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(nil).Times(1)

	exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{FailItemCnt: 1, TerminatedItemCnt: 1}, nil).Times(1)
	exptStatsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1}).Times(1)
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	exec := &ExptFailRetryExec{
		manager:            manager,
		exptItemResultRepo: exptItemResultRepo,
		exptTurnResultRepo: exptTurnResultRepo,
		exptStatsRepo:      exptStatsRepo,
		idgenerator:        idgen,
		exptRepo:           exptRepo,
		idem:               idem,
		configer:           configer,
		publisher:          publisher,
	}

	err := exec.ExptStart(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID:    exptID,
		ExptRunID: newRunID,
		SpaceID:   spaceID,
		Session:   &entity.Session{UserID: "u"},
	}, buildMockExpt())
	assert.NoError(t, err)
}

// TestExptRetryAllExec_ExptStart_ResetsTurnResultTargetAndRunID 覆盖 Single-set 全部重试
// (非 MultiSetConfig 分支): 内联的 UpdateTurnResults 必须写全 target_result_id + expt_run_id。
func TestExptRetryAllExec_ExptStart_ResetsTurnResultTargetAndRunID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	f := buildRetryAllExecFields(ctrl)

	f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
	// 单页返回 1 个 item / 1 个 turn, 结束循环 (nextPageToken=nil)。
	f.evaluationSetItemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: 500, ItemVersionID: ptr.Of(int64(50)), Turns: []*entity.Turn{{ID: 5001}}},
	}, ptr.Of(int64(1)), ptr.Of(int64(1)), nil, nil).Times(1)

	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{600, 601}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), map[string]any{
		"status":      int32(entity.ItemRunState_Queueing),
		"expt_run_id": newRunID,
	}).Return(nil).Times(1)

	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(nil).Times(1)

	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.exptStatsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1}).Times(1)
	f.idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	exec := &ExptRetryAllExec{
		manager:                  f.manager,
		exptItemResultRepo:       f.exptItemResultRepo,
		exptStatsRepo:            f.exptStatsRepo,
		exptTurnResultRepo:       f.exptTurnResultRepo,
		idgenerator:              f.idgenerator,
		evaluationSetItemService: f.evaluationSetItemService,
		exptRepo:                 f.exptRepo,
		idem:                     f.idem,
		configer:                 f.configer,
		publisher:                f.publisher,
		evaluatorRecordService:   f.evaluatorRecordService,
		templateManager:          f.templateManager,
	}

	err := exec.ExptStart(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID:    exptID,
		ExptRunID: newRunID,
		SpaceID:   spaceID,
		Session:   &entity.Session{UserID: "u"},
	}, buildMockExpt())
	assert.NoError(t, err)
}

// TestExptRetryItemsExec_ResetEvalItems_ResetsTurnResultTargetAndRunID 覆盖 Single-set
// 单行重试 (非 MultiSetConfig 分支): UpdateTurnResults 必须写全 target_result_id + expt_run_id。
func TestExptRetryItemsExec_ResetEvalItems_ResetsTurnResultTargetAndRunID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
		itemID   = int64(700)
	)

	f := buildRetryItemsExecFields(ctrl)

	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.evaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: itemID, ItemVersionID: ptr.Of(int64(70)), Turns: []*entity.Turn{{ID: 7001}}},
	}, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{800, 801}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().MGetItemResults(gomock.Any(), exptID, gomock.Any(), spaceID).Return([]*entity.ExptItemResult{{ItemID: itemID, Status: entity.ItemRunState_Fail}}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), map[string]any{
		"status":      int32(entity.ItemRunState_Queueing),
		"expt_run_id": newRunID,
	}).Return(nil).Times(1)

	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(nil).Times(1)

	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptStatsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	exec := &ExptRetryItemsExec{
		manager:                  f.manager,
		exptItemResultRepo:       f.exptItemResultRepo,
		exptStatsRepo:            f.exptStatsRepo,
		exptTurnResultRepo:       f.exptTurnResultRepo,
		idgenerator:              f.idgenerator,
		evaluationSetItemService: f.evaluationSetItemService,
		exptRepo:                 f.exptRepo,
		idem:                     f.idem,
		configer:                 f.configer,
		publisher:                f.publisher,
		evaluatorRecordService:   f.evaluatorRecordService,
		templateManager:          f.templateManager,
		exptRunLogRepo:           f.exptRunLogRepo,
	}

	err := exec.resetEvalItems(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID:    exptID,
		ExptRunID: newRunID,
		SpaceID:   spaceID,
		Session:   &entity.Session{UserID: "u"},
	}, buildMockExpt(), []int64{itemID})
	assert.NoError(t, err)
}
