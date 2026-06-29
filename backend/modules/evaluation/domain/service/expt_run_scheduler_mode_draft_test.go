// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// 草稿哨兵判定 + 读侧/落库版本解析。
func TestDraftEvalSetSentinel_Resolvers(t *testing.T) {
	t.Run("draft: version_id == set_id (提交占位哨兵)", func(t *testing.T) {
		assert.True(t, isDraftEvalSet(7656754417005232130, 7656754417005232130))
		assert.Nil(t, resolveSetReadVersionID(7656754417005232130, 7656754417005232130))
		assert.Equal(t, int64(0), resolveSetRefVersionID(7656754417005232130, 7656754417005232130))
	})

	t.Run("draft: version_id == 0 (显式不锁版本)", func(t *testing.T) {
		assert.True(t, isDraftEvalSet(10, 0))
		assert.Nil(t, resolveSetReadVersionID(10, 0))
		assert.Equal(t, int64(0), resolveSetRefVersionID(10, 0))
	})

	t.Run("committed: 真实 version_id (≠ set_id 且 ≠ 0) → ByVersion 冻结不变", func(t *testing.T) {
		assert.False(t, isDraftEvalSet(7655663479541465089, 7656751138586230785))
		got := resolveSetReadVersionID(7655663479541465089, 7656751138586230785)
		assert.NotNil(t, got)
		assert.Equal(t, int64(7656751138586230785), *got)
		assert.Equal(t, int64(7656751138586230785), resolveSetRefVersionID(7655663479541465089, 7656751138586230785))
	})
}

// 扫描层: 草稿集 List 拉取必须传 VersionID=nil (走 live), 且 expt_item_ref.EvalSetVersionID 落 0。
func TestExptSubmitExec_exptStartMultiSet_DraftSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exec, setItemSvc, itemRefRepo := newExptStartMultiSetTestExec(ctrl)

	var captured []*entity.ExptItemRef
	itemRefRepo.EXPECT().BatchCreate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, refs []*entity.ExptItemRef) error {
			captured = append(captured, refs...)
			return nil
		}).AnyTimes()

	// 草稿集 set_id=draftSet, version_id 提交侧用 set_id 当占位哨兵。
	// List 拉取时必须 VersionID==nil (live 读当前草稿)。
	const draftSet = int64(7656754417005232130)
	var sawVersionID *int64
	var sawVersionIDSet bool
	setItemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
			sawVersionID = p.VersionID
			sawVersionIDSet = true
			return []*entity.EvaluationSetItem{
				{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
			}, ptr.Of(int64(1)), nil, nil, nil
		}).Times(1)

	expt := &entity.Experiment{
		ID:      1,
		SpaceID: 3,
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{EvalSetID: draftSet, EvalSetVersionID: draftSet}, // 草稿哨兵
			},
		},
	}
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u1"})
	err := exec.exptStartMultiSet(ctx, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u1"}}, expt)
	assert.NoError(t, err)

	assert.True(t, sawVersionIDSet, "ListEvaluationSetItems 应被调用")
	assert.Nil(t, sawVersionID, "草稿集读侧 VersionID 必须为 nil → 走 live BatchGet/List")
	assert.Len(t, captured, 1)
	assert.Equal(t, draftSet, captured[0].EvalSetID)
	assert.Equal(t, int64(0), captured[0].EvalSetVersionID, "草稿集 expt_item_ref.eval_set_version_id 落 0")
}

// 扫描层: committed 集维持 ByVersion (VersionID 非 nil) + ref 落真实 version_id 不变 (回归守卫)。
func TestExptSubmitExec_exptStartMultiSet_CommittedSet_Unchanged(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exec, setItemSvc, itemRefRepo := newExptStartMultiSetTestExec(ctrl)

	var captured []*entity.ExptItemRef
	itemRefRepo.EXPECT().BatchCreate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, refs []*entity.ExptItemRef) error {
			captured = append(captured, refs...)
			return nil
		}).AnyTimes()

	const committedSet = int64(7655663479541465089)
	const committedVer = int64(7656751138586230785)
	var sawVersionID *int64
	setItemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
			sawVersionID = p.VersionID
			return []*entity.EvaluationSetItem{
				{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
			}, ptr.Of(int64(1)), nil, nil, nil
		}).Times(1)

	expt := &entity.Experiment{
		ID:      1,
		SpaceID: 3,
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{EvalSetID: committedSet, EvalSetVersionID: committedVer},
			},
		},
	}
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u1"})
	err := exec.exptStartMultiSet(ctx, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u1"}}, expt)
	assert.NoError(t, err)

	assert.NotNil(t, sawVersionID, "committed 集读侧 VersionID 必须非 nil → 走 ByVersion 冻结")
	assert.Equal(t, committedVer, *sawVersionID)
	assert.Len(t, captured, 1)
	assert.Equal(t, committedVer, captured[0].EvalSetVersionID, "committed 集 ref 落真实 version_id 不变")
}
