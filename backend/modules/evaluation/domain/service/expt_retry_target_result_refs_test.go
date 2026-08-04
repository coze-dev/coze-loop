// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
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

// runLogItemVersionMatcher 断言创建的 ExptItemResultRunLog 都携带非零 ItemVersionID,
// 且与期望的 (item_id -> version_id) 映射一致。
type runLogItemVersionMatcher struct{ want map[int64]int64 }

func (m runLogItemVersionMatcher) Matches(x any) bool {
	logs, ok := x.([]*entity.ExptItemResultRunLog)
	if !ok {
		return false
	}
	if len(logs) != len(m.want) {
		return false
	}
	for _, rl := range logs {
		if rl == nil {
			return false
		}
		want, ok := m.want[rl.ItemID]
		if !ok {
			return false
		}
		if rl.ItemVersionID != want {
			return false
		}
	}
	return true
}

func (m runLogItemVersionMatcher) String() string {
	return "[]*ExptItemResultRunLog with expected ItemVersionID per ItemID"
}

// TestExptRetryItemsExec_ResetEvalItems_ItemVersionIDPassThrough 覆盖 Single-set 单行重试:
// 新建的 expt_item_result_run_log 必须从 expt_item_result 平移 item_version_id,
// 否则 BuildExptRecordEvalCtx 兜底读到 0, ItemVersionQueries 里 ItemVersionID=nil,
// 触发下游 601100201 (item_version_id or item_version is required)。
func TestExptRetryItemsExec_ResetEvalItems_ItemVersionIDPassThrough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID          = int64(1)
		newRunID        = int64(2)
		spaceID         = int64(3)
		itemID          = int64(700)
		expectVersionID = int64(70) // 从 expt_item_result.ItemVersionID 平移
	)

	f := buildRetryItemsExecFields(ctrl)

	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.evaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: itemID, ItemVersionID: ptr.Of(expectVersionID), Turns: []*entity.Turn{{ID: 7001}}},
	}, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{800, 801}, nil).Times(1)
	// item_result 表里已经存了 item 版本 (首次执行时落下), 重试重置时需要平移到 run_log。
	f.exptItemResultRepo.EXPECT().MGetItemResults(gomock.Any(), exptID, gomock.Any(), spaceID).Return([]*entity.ExptItemResult{
		{ItemID: itemID, ItemVersionID: expectVersionID, Status: entity.ItemRunState_Fail},
	}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(nil).Times(1)

	// 核心断言: 落 run_log 时必须带上从 item_result 平移的 ItemVersionID。
	f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), runLogItemVersionMatcher{want: map[int64]int64{itemID: expectVersionID}}).Return(nil).Times(1)

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

// TestExptRetryAllExec_ExptStart_ItemVersionIDPassThrough 覆盖 Single-set 全部重试:
// 新建的 expt_item_result_run_log 必须从 List 返回的 item.ItemVersionID 平移到 run_log,
// 与单行重试对齐, 避免同类 601100201 风险 (当前用户没在 committed 版本上试全部重试, 但潜在漏洞相同)。
func TestExptRetryAllExec_ExptStart_ItemVersionIDPassThrough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID          = int64(1)
		newRunID        = int64(2)
		spaceID         = int64(3)
		itemID          = int64(500)
		expectVersionID = int64(50)
	)

	f := buildRetryAllExecFields(ctrl)

	f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
	f.evaluationSetItemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: itemID, ItemVersionID: ptr.Of(expectVersionID), Turns: []*entity.Turn{{ID: 5001}}},
	}, ptr.Of(int64(1)), ptr.Of(int64(1)), nil, nil).Times(1)

	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{600, 601}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(nil).Times(1)

	// 核心断言。
	f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), runLogItemVersionMatcher{want: map[int64]int64{itemID: expectVersionID}}).Return(nil).Times(1)

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

// ============================================================================
// resetRetryRunLogsForItems helper 直测 (被 MultiSet 全部/单行重试共用): 断言 turn map
// 精确形态 + run_log ItemVersionID 覆盖 (item.ItemVersionID 优先, itemVersionByItemID 兜底)。
// ============================================================================

// runLogFullMatcher 断言创建的 ExptItemResultRunLog 的 (ItemID, ItemVersionID) 完全等于期望。
type runLogFullMatcher struct{ want map[int64]int64 }

func (m runLogFullMatcher) Matches(x any) bool {
	logs, ok := x.([]*entity.ExptItemResultRunLog)
	if !ok || len(logs) != len(m.want) {
		return false
	}
	for _, rl := range logs {
		if rl == nil {
			return false
		}
		w, ok := m.want[rl.ItemID]
		if !ok || rl.ItemVersionID != w {
			return false
		}
	}
	return true
}

func (m runLogFullMatcher) String() string {
	return "run_log slice with expected (item_id, version_id)"
}

func newRetryHelperDeps(ctrl *gomock.Controller) retryItemResetDeps {
	return retryItemResetDeps{
		evaluationSetItemService: svcmocks.NewMockEvaluationSetItemService(ctrl),
		exptItemResultRepo:       mock_repo.NewMockIExptItemResultRepo(ctrl),
		exptTurnResultRepo:       mock_repo.NewMockIExptTurnResultRepo(ctrl),
		idgenerator:              idgenmocks.NewMockIIDGenerator(ctrl),
	}
}

func TestResetRetryRunLogsForItems_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deps := newRetryHelperDeps(ctrl)
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}

	got, err := resetRetryRunLogsForItems(context.Background(), deps, event, nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestResetRetryRunLogsForItems_VersionPrecedenceAndTurnMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(11)
		newRunID = int64(22)
		spaceID  = int64(33)
	)

	deps := newRetryHelperDeps(ctrl)
	items := []*entity.EvaluationSetItem{
		{ItemID: 100, ItemVersionID: ptr.Of(int64(1000)), Turns: []*entity.Turn{{ID: 10001}}}, // 自带版本优先
		{ItemID: 200, ItemVersionID: nil, Turns: []*entity.Turn{{ID: 20001}}},                 // fallback 到 map
		{ItemID: 300, ItemVersionID: ptr.Of(int64(0)), Turns: []*entity.Turn{{ID: 30001}}},    // 0 也走 fallback
	}
	versionMap := map[int64]int64{
		200: 2000,
		// 300 缺失 → 期望 run_log ItemVersionID=0 (fallback map 也不命中即为 0, 与"无版本"语义一致)
	}

	deps.idgenerator.(*idgenmocks.MockIIDGenerator).EXPECT().GenMultiIDs(gomock.Any(), 6).Return([]int64{1, 2, 3, 4, 5, 6}, nil).Times(1)
	deps.exptItemResultRepo.(*mock_repo.MockIExptItemResultRepo).EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), map[string]any{
		"status":      int32(entity.ItemRunState_Queueing),
		"expt_run_id": newRunID,
	}).Return(nil).Times(1)

	// 核心断言 1: turn map 满足 (status, target_result_id=0, expt_run_id=NEW)。
	deps.exptTurnResultRepo.(*mock_repo.MockIExptTurnResultRepo).EXPECT().UpdateTurnResults(
		gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID},
	).Return(nil).Times(1)
	deps.exptTurnResultRepo.(*mock_repo.MockIExptTurnResultRepo).EXPECT().UpdateTurnRunLogWithItemIDs(
		gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any(),
	).Return(nil).Times(1)

	// 核心断言 2: run_log 每个 item 的 ItemVersionID 符合优先级 (item.ItemVersionID > map > 0)。
	deps.exptItemResultRepo.(*mock_repo.MockIExptItemResultRepo).EXPECT().BatchCreateNXRunLogs(
		gomock.Any(),
		runLogFullMatcher{want: map[int64]int64{100: 1000, 200: 2000, 300: 0}},
	).Return(nil).Times(1)

	got, err := resetRetryRunLogsForItems(context.Background(), deps, &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID,
	}, items, versionMap)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int64{100, 200, 300}, got)
}

func TestResetRetryRunLogsForItems_UpdateItemsResultError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deps := newRetryHelperDeps(ctrl)
	items := []*entity.EvaluationSetItem{{ItemID: 1, Turns: []*entity.Turn{{ID: 10}}}}
	deps.idgenerator.(*idgenmocks.MockIIDGenerator).EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil).Times(1)
	deps.exptItemResultRepo.(*mock_repo.MockIExptItemResultRepo).EXPECT().UpdateItemsResult(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(errors.New("boom")).Times(1)

	got, err := resetRetryRunLogsForItems(context.Background(), deps, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, items, nil)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestResetRetryRunLogsForItems_UpdateTurnResultsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deps := newRetryHelperDeps(ctrl)
	items := []*entity.EvaluationSetItem{{ItemID: 1, Turns: []*entity.Turn{{ID: 10}}}}
	deps.idgenerator.(*idgenmocks.MockIIDGenerator).EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil).Times(1)
	deps.exptItemResultRepo.(*mock_repo.MockIExptItemResultRepo).EXPECT().UpdateItemsResult(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil).Times(1)
	deps.exptTurnResultRepo.(*mock_repo.MockIExptTurnResultRepo).EXPECT().UpdateTurnResults(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(errors.New("boom")).Times(1)

	got, err := resetRetryRunLogsForItems(context.Background(), deps, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, items, nil)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestResetRetryRunLogsForItems_BatchCreateRunLogsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deps := newRetryHelperDeps(ctrl)
	items := []*entity.EvaluationSetItem{{ItemID: 1, Turns: []*entity.Turn{{ID: 10}}}}
	deps.idgenerator.(*idgenmocks.MockIIDGenerator).EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil).Times(1)
	deps.exptItemResultRepo.(*mock_repo.MockIExptItemResultRepo).EXPECT().UpdateItemsResult(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil).Times(1)
	deps.exptTurnResultRepo.(*mock_repo.MockIExptTurnResultRepo).EXPECT().UpdateTurnResults(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil).Times(1)
	deps.exptTurnResultRepo.(*mock_repo.MockIExptTurnResultRepo).EXPECT().UpdateTurnRunLogWithItemIDs(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil).Times(1)
	deps.exptItemResultRepo.(*mock_repo.MockIExptItemResultRepo).EXPECT().BatchCreateNXRunLogs(
		gomock.Any(), gomock.Any(),
	).Return(errors.New("boom")).Times(1)

	got, err := resetRetryRunLogsForItems(context.Background(), deps, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, items, nil)
	assert.Error(t, err)
	assert.Nil(t, got)
}

// ============================================================================
// ExptRetryItemsExec.resetEvalItems (Single-set) 覆盖率补丁: 边界/失败/统计分支。
// ============================================================================

// newRetryItemsExecFromFields 组装 ExptRetryItemsExec, 供本文件多个 test 复用。
func newRetryItemsExecFromFields(f *exptRetryItemsExecFields) *ExptRetryItemsExec {
	return &ExptRetryItemsExec{
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
}

// TestExptRetryItemsExec_ResetEvalItems_NilIrsAndMissingVersion 覆盖:
//   - MGetItemResults 返回含 nil 元素 → 跳过不 panic;
//   - 某个 item 在 item_result 里查不到 → run_log ItemVersionID=0 (map 未命中)。
func TestExptRetryItemsExec_ResetEvalItems_NilIrsAndMissingVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
		itemA    = int64(1001) // MGetItemResults 命中, 版本 = 10001
		itemB    = int64(1002) // MGetItemResults 未命中 → 版本兜底为 0
	)

	f := buildRetryItemsExecFields(ctrl)

	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.evaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: itemA, Turns: []*entity.Turn{{ID: 90001}}},
		{ItemID: itemB, Turns: []*entity.Turn{{ID: 90002}}},
	}, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 4).Return([]int64{7001, 7002, 7003, 7004}, nil).Times(1)
	// MGetItemResults: 一个 nil, 一个只覆盖 itemA。
	f.exptItemResultRepo.EXPECT().MGetItemResults(gomock.Any(), exptID, gomock.Any(), spaceID).Return([]*entity.ExptItemResult{
		nil,
		{ItemID: itemA, ItemVersionID: 10001, Status: entity.ItemRunState_Fail},
	}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(nil).Times(1)

	f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(
		gomock.Any(),
		runLogFullMatcher{want: map[int64]int64{itemA: 10001, itemB: 0}},
	).Return(nil).Times(1)

	f.exptStatsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	exec := newRetryItemsExecFromFields(f)

	err := exec.resetEvalItems(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt(), []int64{itemA, itemB})
	assert.NoError(t, err)
}

// TestExptRetryItemsExec_ResetEvalItems_StatsPerStatus 覆盖 stats 统计所有状态分支的转移。
// 4 个 item 对应 Processing / Success / Fail / Terminal 各 1 个, 都应转成 PendingItemCnt++, 原计数--。
func TestExptRetryItemsExec_ResetEvalItems_StatsPerStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	f := buildRetryItemsExecFields(ctrl)

	got := &entity.ExptStats{
		ProcessingItemCnt: 1,
		SuccessItemCnt:    1,
		FailItemCnt:       1,
		TerminatedItemCnt: 1,
	}
	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(got, nil).Times(1)
	f.evaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
		{ItemID: 2, Turns: []*entity.Turn{{ID: 21}}},
		{ItemID: 3, Turns: []*entity.Turn{{ID: 31}}},
		{ItemID: 4, Turns: []*entity.Turn{{ID: 41}}},
	}, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 8).Return([]int64{1, 2, 3, 4, 5, 6, 7, 8}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().MGetItemResults(gomock.Any(), exptID, gomock.Any(), spaceID).Return([]*entity.ExptItemResult{
		{ItemID: 1, Status: entity.ItemRunState_Processing},
		{ItemID: 2, Status: entity.ItemRunState_Success},
		{ItemID: 3, Status: entity.ItemRunState_Fail},
		{ItemID: 4, Status: entity.ItemRunState_Terminal},
	}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	// Save 时验证四个计数字段都清零, PendingItemCnt=4。
	f.exptStatsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s *entity.ExptStats) error {
		assert.EqualValues(t, 4, s.PendingItemCnt)
		assert.EqualValues(t, 0, s.ProcessingItemCnt)
		assert.EqualValues(t, 0, s.SuccessItemCnt)
		assert.EqualValues(t, 0, s.FailItemCnt)
		assert.EqualValues(t, 0, s.TerminatedItemCnt)
		return nil
	}).Times(1)

	exec := newRetryItemsExecFromFields(f)
	err := exec.resetEvalItems(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt(), []int64{1, 2, 3, 4})
	assert.NoError(t, err)
}

// TestExptRetryItemsExec_ResetEvalItems_MGetItemResultsError 覆盖 MGetItemResults 报错短路:
// 不应再触发 UpdateItemsResult / UpdateTurnResults / BatchCreateNXRunLogs。
func TestExptRetryItemsExec_ResetEvalItems_MGetItemResultsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	f := buildRetryItemsExecFields(ctrl)
	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.evaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
	}, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().MGetItemResults(gomock.Any(), exptID, gomock.Any(), spaceID).Return(nil, errors.New("boom")).Times(1)

	exec := newRetryItemsExecFromFields(f)
	err := exec.resetEvalItems(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt(), []int64{1})
	assert.Error(t, err)
}

// TestExptRetryItemsExec_ResetEvalItems_UpdateTurnResultsError 覆盖 UpdateTurnResults 报错短路:
// 不应再触发 clearExptTurnRunLog / BatchCreateNXRunLogs / Save。
func TestExptRetryItemsExec_ResetEvalItems_UpdateTurnResultsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	f := buildRetryItemsExecFields(ctrl)
	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.evaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
	}, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().MGetItemResults(gomock.Any(), exptID, gomock.Any(), spaceID).Return([]*entity.ExptItemResult{
		{ItemID: 1, ItemVersionID: 100, Status: entity.ItemRunState_Fail},
	}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(errors.New("boom")).Times(1)

	exec := newRetryItemsExecFromFields(f)
	err := exec.resetEvalItems(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt(), []int64{1})
	assert.Error(t, err)
}

// ============================================================================
// ExptFailRetryExec.ExptStart 边界补测。
// ============================================================================

// TestExptFailRetryExec_ExptStart_IdemExistsShortCircuits 覆盖 idem 已存在时早退:
// 不应再触发 ScanTurnResults / UpdateTurnResults / Save。
func TestExptFailRetryExec_ExptStart_IdemExistsShortCircuits(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil).Times(1)

	exec := &ExptFailRetryExec{
		manager:            svcmocks.NewMockIExptManager(ctrl),
		exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
		exptTurnResultRepo: mock_repo.NewMockIExptTurnResultRepo(ctrl),
		exptStatsRepo:      mock_repo.NewMockIExptStatsRepo(ctrl),
		idgenerator:        idgenmocks.NewMockIIDGenerator(ctrl),
		exptRepo:           mock_repo.NewMockIExperimentRepo(ctrl),
		idem:               idem,
		configer:           configmocks.NewMockIConfiger(ctrl),
		publisher:          eventmocks.NewMockExptEventPublisher(ctrl),
	}

	err := exec.ExptStart(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt())
	assert.NoError(t, err)
}

// TestExptFailRetryExec_ExptStart_UpdateTurnResultsError 覆盖 UpdateTurnResults 报错传播:
// turn map 必须仍满足契约, 且报错后不再触发下游 clear/BatchCreate/Get/Save/Update。
func TestExptFailRetryExec_ExptStart_UpdateTurnResultsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
	exptTurnResultRepo.EXPECT().ScanTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{
		{ItemID: 1, TurnID: 10, Status: int32(entity.TurnRunState_Fail)},
	}, int64(0), nil).Times(1)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{1}, nil).Times(1)
	exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	// 核心断言: turn map 契约仍满足, 错误后不再触发下游。
	exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), turnResetMapMatcher{wantRunID: 2}).Return(errors.New("boom")).Times(1)

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
		ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt())
	assert.Error(t, err)
}

// ============================================================================
// ExptRetryAllExec.ExptStart Single-set 分支补测: 多 item 混合版本 + 错误路径。
// ============================================================================

// TestExptRetryAllExec_ExptStart_MultiItemMixedVersion 覆盖 List 返回多 item 且版本存在/为 nil 混合场景:
// 有 ItemVersionID 的 item 落 run_log 时精确保留, nil / 0 的 item 落 run_log ItemVersionID=0。
func TestExptRetryAllExec_ExptStart_MultiItemMixedVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	f := buildRetryAllExecFields(ctrl)
	f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
	f.evaluationSetItemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: 10, ItemVersionID: ptr.Of(int64(1010)), Turns: []*entity.Turn{{ID: 100}}},
		{ItemID: 20, ItemVersionID: nil, Turns: []*entity.Turn{{ID: 200}}},              // nil → 0
		{ItemID: 30, ItemVersionID: ptr.Of(int64(0)), Turns: []*entity.Turn{{ID: 300}}}, // 0 也 0
	}, ptr.Of(int64(3)), ptr.Of(int64(3)), nil, nil).Times(1)

	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 6).Return([]int64{1, 2, 3, 4, 5, 6}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), runLogFullMatcher{want: map[int64]int64{10: 1010, 20: 0, 30: 0}}).Return(nil).Times(1)
	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.exptStatsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1}).Times(1)
	f.idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	exec := &ExptRetryAllExec{
		manager: f.manager, exptItemResultRepo: f.exptItemResultRepo, exptStatsRepo: f.exptStatsRepo,
		exptTurnResultRepo: f.exptTurnResultRepo, idgenerator: f.idgenerator, evaluationSetItemService: f.evaluationSetItemService,
		exptRepo: f.exptRepo, idem: f.idem, configer: f.configer, publisher: f.publisher,
		evaluatorRecordService: f.evaluatorRecordService, templateManager: f.templateManager,
	}
	err := exec.ExptStart(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt())
	assert.NoError(t, err)
}

// TestExptRetryAllExec_ExptStart_UpdateTurnResultsError 覆盖 UpdateTurnResults 报错短路。
func TestExptRetryAllExec_ExptStart_UpdateTurnResultsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	f := buildRetryAllExecFields(ctrl)
	f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
	f.evaluationSetItemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: 10, ItemVersionID: ptr.Of(int64(1010)), Turns: []*entity.Turn{{ID: 100}}},
	}, ptr.Of(int64(1)), ptr.Of(int64(1)), nil, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(errors.New("boom")).Times(1)

	exec := &ExptRetryAllExec{
		manager: f.manager, exptItemResultRepo: f.exptItemResultRepo, exptStatsRepo: f.exptStatsRepo,
		exptTurnResultRepo: f.exptTurnResultRepo, idgenerator: f.idgenerator, evaluationSetItemService: f.evaluationSetItemService,
		exptRepo: f.exptRepo, idem: f.idem, configer: f.configer, publisher: f.publisher,
		evaluatorRecordService: f.evaluatorRecordService, templateManager: f.templateManager,
	}
	err := exec.ExptStart(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt())
	assert.Error(t, err)
}

// TestExptRetryItemsExec_ResetEvalItems_ClearRunLogRefsError 覆盖 clearExptTurnRunLog 报错短路。
func TestExptRetryItemsExec_ResetEvalItems_ClearRunLogRefsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	f := buildRetryItemsExecFields(ctrl)
	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.evaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
	}, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().MGetItemResults(gomock.Any(), exptID, gomock.Any(), spaceID).Return([]*entity.ExptItemResult{
		{ItemID: 1, ItemVersionID: 100, Status: entity.ItemRunState_Fail},
	}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(errors.New("boom")).Times(1)

	exec := newRetryItemsExecFromFields(f)
	err := exec.resetEvalItems(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt(), []int64{1})
	assert.Error(t, err)
}

// TestExptRetryItemsExec_ResetEvalItems_BatchCreateRunLogsError 覆盖 BatchCreateNXRunLogs 报错短路。
func TestExptRetryItemsExec_ResetEvalItems_BatchCreateRunLogsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	f := buildRetryItemsExecFields(ctrl)
	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.evaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
	}, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().MGetItemResults(gomock.Any(), exptID, gomock.Any(), spaceID).Return([]*entity.ExptItemResult{
		{ItemID: 1, ItemVersionID: 100, Status: entity.ItemRunState_Fail},
	}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(errors.New("boom")).Times(1)

	exec := newRetryItemsExecFromFields(f)
	err := exec.resetEvalItems(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt(), []int64{1})
	assert.Error(t, err)
}

// TestExptRetryItemsExec_ResetEvalItems_StatsSaveError 覆盖 Stats.Save 报错。
func TestExptRetryItemsExec_ResetEvalItems_StatsSaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
	)

	f := buildRetryItemsExecFields(ctrl)
	f.exptStatsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{}, nil).Times(1)
	f.evaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
		{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
	}, nil).Times(1)
	f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().MGetItemResults(gomock.Any(), exptID, gomock.Any(), spaceID).Return([]*entity.ExptItemResult{
		{ItemID: 1, ItemVersionID: 100, Status: entity.ItemRunState_Fail},
	}, nil).Times(1)
	f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, turnResetMapMatcher{wantRunID: newRunID}).Return(nil).Times(1)
	f.exptTurnResultRepo.EXPECT().UpdateTurnRunLogWithItemIDs(gomock.Any(), spaceID, exptID, newRunID, gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	f.exptStatsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("boom")).Times(1)

	exec := newRetryItemsExecFromFields(f)
	err := exec.resetEvalItems(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, Session: &entity.Session{UserID: "u"},
	}, buildMockExpt(), []int64{1})
	assert.Error(t, err)
}
