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
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExptSubmitExec_exptStartMultiSet(t *testing.T) {
	const testUserID = "u1"
	baseEvent := func() *entity.ExptScheduleEvent {
		return &entity.ExptScheduleEvent{
			ExptID:    1,
			ExptRunID: 2,
			SpaceID:   3,
			Session:   &entity.Session{UserID: testUserID},
		}
	}

	t.Run("error_nil_itemRefRepo", func(t *testing.T) {
		e := &ExptSubmitExec{} // exptItemRefRepo 为 nil
		err := e.exptStartMultiSet(context.Background(), baseEvent(), &entity.Experiment{ID: 1})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exptItemRefRepo is nil")
	})

	t.Run("error_no_eval_set_configs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		e := &ExptSubmitExec{
			exptItemRefRepo: mock_repo.NewMockIExptItemRefRepo(ctrl),
		}
		// EvalConf 为 nil → 无 eval_set_configs
		err := e.exptStartMultiSet(context.Background(), baseEvent(), &entity.Experiment{ID: 1})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no eval_set_configs")
	})

	t.Run("happy_path_single_set_single_page", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		itemRefRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
		setItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
		idgenerator := idgenmocks.NewMockIIDGenerator(ctrl)
		turnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		itemResultRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
		statsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
		exptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
		resultSvc := svcmocks.NewMockExptResultService(ctrl)
		configer := configmocks.NewMockIConfiger(ctrl)
		idemSvc := idemmocks.NewMockIdempotentService(ctrl)

		// idgen: 按请求数量返回连续 id, 避免下标越界
		idgenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, n int) ([]int64, error) {
				ids := make([]int64, n)
				for i := range ids {
					ids[i] = int64(i + 1)
				}
				return ids, nil
			}).AnyTimes()

		// 单评测集, 单页 2 item, 各 1 turn; total=2 → 一页后终止
		// item1 无版本 (老数据集语义, ItemVersionID nil → 写 0); item2 带版本 (新数据集, 写真值 555)
		setItemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return(
			[]*entity.EvaluationSetItem{
				{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
				{ItemID: 2, ItemVersionID: ptr.Of(int64(555)), Turns: []*entity.Turn{{ID: 22}}},
			}, ptr.Of(int64(2)), ptr.Of(int64(2)), nil, nil).Times(1)

		var captured []*entity.ExptItemRef
		itemRefRepo.EXPECT().BatchCreate(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, refs []*entity.ExptItemRef) error {
				captured = refs
				return nil
			}).Times(1)

		// createItemTurnResults
		turnRepo.EXPECT().BatchCreateNX(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		itemResultRepo.EXPECT().BatchCreateNX(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		itemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(1)

		// finishExptStart
		resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
		statsRepo.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
		exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		exptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{ID: 1}, nil).Times(1)
		configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1}).Times(1)
		idemSvc.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

		e := &ExptSubmitExec{
			exptItemRefRepo:          itemRefRepo,
			evaluationSetItemService: setItemSvc,
			idgenerator:              idgenerator,
			exptTurnResultRepo:       turnRepo,
			exptItemResultRepo:       itemResultRepo,
			exptStatsRepo:            statsRepo,
			exptRepo:                 exptRepo,
			resultSvc:                resultSvc,
			configer:                 configer,
			idem:                     idemSvc,
		}

		expt := &entity.Experiment{
			ID:      1,
			SpaceID: 3,
			EvalConf: &entity.EvaluationConfiguration{
				EvalSetConfigs: []*entity.EvalSetConfig{
					{
						EvalSetID:        10,
						EvalSetVersionID: 100,
						EvaluatorConfs: []*entity.ExptEvaluatorConf{
							{EvaluatorID: 7, EvaluatorVersionID: 700, Alias: "j"},
						},
						TargetConfs: []*entity.ExptTargetConf{{TargetVersionID: 999}},
					},
				},
			},
		}

		ctx := session.WithCtxUser(context.Background(), &session.User{ID: testUserID})
		err := e.exptStartMultiSet(ctx, baseEvent(), expt)
		assert.NoError(t, err)

		// 扁平化结果: 2 行 expt_item_ref
		assert.Len(t, captured, 2)
		// item1
		assert.Equal(t, int64(1), captured[0].ItemID)
		assert.Equal(t, int32(0), captured[0].OrderIdx)
		assert.Equal(t, int64(0), captured[0].ItemVersionID) // item1 无版本 → 0 (老数据集)
		assert.Equal(t, int64(10), captured[0].EvalSetID)
		assert.Equal(t, int64(100), captured[0].EvalSetVersionID)
		assert.NotNil(t, captured[0].ItemConfig)
		assert.Len(t, captured[0].ItemConfig.EvaluatorConfs, 1)
		assert.Equal(t, "j", captured[0].ItemConfig.EvaluatorConfs[0].Alias)
		assert.Equal(t, int64(700), captured[0].ItemConfig.EvaluatorConfs[0].EvaluatorVersionID)
		// item2 带版本 → 写真值 555 (新数据集)
		assert.Equal(t, int64(2), captured[1].ItemID)
		assert.Equal(t, int64(555), captured[1].ItemVersionID)
		assert.NotNil(t, captured[0].ItemConfig.EvalTargetConf)
		assert.Equal(t, int64(999), captured[0].ItemConfig.EvalTargetConf.TargetVersionID)
		// item2: OrderIdx 连续递增
		assert.Equal(t, int64(2), captured[1].ItemID)
		assert.Equal(t, int32(1), captured[1].OrderIdx)
	})
}

// newExptStartMultiSetTestExec 构造一个全套 mock 的 ExptSubmitExec, 把与 set/page 数无关的
// 下游调用(idgen、createItemTurnResults、finishExptStart)统一设为 AnyTimes, 让各 case 只需聚焦
// ListEvaluationSetItems 的分页/分集编排与 BatchCreate 的扁平化结果断言。
func newExptStartMultiSetTestExec(ctrl *gomock.Controller) (*ExptSubmitExec, *svcmocks.MockEvaluationSetItemService, *mock_repo.MockIExptItemRefRepo) {
	itemRefRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
	setItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	idgenerator := idgenmocks.NewMockIIDGenerator(ctrl)
	turnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
	itemResultRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
	statsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
	exptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	idemSvc := idemmocks.NewMockIdempotentService(ctrl)

	idgenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, n int) ([]int64, error) {
			ids := make([]int64, n)
			for i := range ids {
				ids[i] = int64(i + 1)
			}
			return ids, nil
		}).AnyTimes()

	// createItemTurnResults: 每页一次, 次数随分页变化
	turnRepo.EXPECT().BatchCreateNX(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	itemResultRepo.EXPECT().BatchCreateNX(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	itemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// finishExptStart: 末尾一次性
	resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	statsRepo.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	exptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{ID: 1}, nil).AnyTimes()
	configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1}).AnyTimes()
	idemSvc.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	exec := &ExptSubmitExec{
		exptItemRefRepo:          itemRefRepo,
		evaluationSetItemService: setItemSvc,
		idgenerator:              idgenerator,
		exptTurnResultRepo:       turnRepo,
		exptItemResultRepo:       itemResultRepo,
		exptStatsRepo:            statsRepo,
		exptRepo:                 exptRepo,
		resultSvc:                resultSvc,
		configer:                 configer,
		idem:                     idemSvc,
	}
	return exec, setItemSvc, itemRefRepo
}

// 单评测集翻页: total=3, page0 返回 2 + nextPageToken, page1 返回 1 + nil。
// 验证 pageToken 正确透传、跨页 OrderIdx 连续递增。
func TestExptSubmitExec_exptStartMultiSet_Paging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exec, setItemSvc, itemRefRepo := newExptStartMultiSetTestExec(ctrl)

	var captured []*entity.ExptItemRef
	itemRefRepo.EXPECT().BatchCreate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, refs []*entity.ExptItemRef) error {
			captured = append(captured, refs...)
			return nil
		}).AnyTimes()

	setItemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
			if p.PageToken == nil {
				return []*entity.EvaluationSetItem{
					{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
					{ItemID: 2, Turns: []*entity.Turn{{ID: 22}}},
				}, ptr.Of(int64(3)), nil, ptr.Of("p2"), nil
			}
			assert.Equal(t, "p2", *p.PageToken) // 第二页应带上一页返回的 token
			return []*entity.EvaluationSetItem{
				{ItemID: 3, Turns: []*entity.Turn{{ID: 33}}},
			}, ptr.Of(int64(3)), nil, nil, nil
		}).Times(2)

	expt := &entity.Experiment{
		ID:      1,
		SpaceID: 3,
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{EvalSetID: 10, EvalSetVersionID: 100},
			},
		},
	}
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u1"})
	err := exec.exptStartMultiSet(ctx, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u1"}}, expt)
	assert.NoError(t, err)

	assert.Len(t, captured, 3) // 跨两页累计 3 行
	assert.Equal(t, int32(0), captured[0].OrderIdx)
	assert.Equal(t, int32(1), captured[1].OrderIdx)
	assert.Equal(t, int32(2), captured[2].OrderIdx) // OrderIdx 跨页连续
	assert.Equal(t, int64(3), captured[2].ItemID)
}

// 多评测集(各单页): set10 与 set20 各 2 item。
// 验证扁平化后 EvalSetID 归属正确、OrderIdx 跨 set 连续。
func TestExptSubmitExec_exptStartMultiSet_MultipleSets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exec, setItemSvc, itemRefRepo := newExptStartMultiSetTestExec(ctrl)

	var captured []*entity.ExptItemRef
	itemRefRepo.EXPECT().BatchCreate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, refs []*entity.ExptItemRef) error {
			captured = append(captured, refs...)
			return nil
		}).AnyTimes()

	setItemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
			switch p.EvaluationSetID {
			case 10:
				return []*entity.EvaluationSetItem{
					{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
					{ItemID: 2, Turns: []*entity.Turn{{ID: 22}}},
				}, ptr.Of(int64(2)), nil, nil, nil
			case 20:
				return []*entity.EvaluationSetItem{
					{ItemID: 3, Turns: []*entity.Turn{{ID: 33}}},
					{ItemID: 4, Turns: []*entity.Turn{{ID: 44}}},
				}, ptr.Of(int64(2)), nil, nil, nil
			}
			return nil, ptr.Of(int64(0)), nil, nil, nil
		}).Times(2)

	expt := &entity.Experiment{
		ID:      1,
		SpaceID: 3,
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{EvalSetID: 10, EvalSetVersionID: 100},
				{EvalSetID: 20, EvalSetVersionID: 200},
			},
		},
	}
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u1"})
	err := exec.exptStartMultiSet(ctx, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u1"}}, expt)
	assert.NoError(t, err)

	assert.Len(t, captured, 4)
	// EvalSetID 归属
	assert.Equal(t, int64(10), captured[0].EvalSetID)
	assert.Equal(t, int64(10), captured[1].EvalSetID)
	assert.Equal(t, int64(20), captured[2].EvalSetID)
	assert.Equal(t, int64(20), captured[3].EvalSetID)
	// OrderIdx 跨 set 连续
	for i := range captured {
		assert.Equal(t, int32(i), captured[i].OrderIdx)
	}
}

// item_id 点选 item_filter: 走 BatchGetEvaluationSetItems (不走 List), 只落命中的 item。
func TestExptSubmitExec_exptStartMultiSet_ItemIDFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exec, setItemSvc, itemRefRepo := newExptStartMultiSetTestExec(ctrl)

	var captured []*entity.ExptItemRef
	itemRefRepo.EXPECT().BatchCreate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, refs []*entity.ExptItemRef) error {
			captured = append(captured, refs...)
			return nil
		}).AnyTimes()

	// 点选只选 item_id=2,7 → 必须走 BatchGet 且 query 只带这两个 id; List 不应被调用
	var batchQueried []int64
	setItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
			assert.Empty(t, p.ItemIDs)
			for _, q := range p.ItemVersionQueries {
				batchQueried = append(batchQueried, q.ItemID)
			}
			return []*entity.EvaluationSetItem{
				{ItemID: 2, Turns: []*entity.Turn{{ID: 22}}},
				{ItemID: 7, Turns: []*entity.Turn{{ID: 77}}},
			}, nil
		}).Times(1)
	// List 绝不应被调用 (点选路径)
	setItemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Times(0)

	expt := &entity.Experiment{
		ID:      1,
		SpaceID: 3,
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{
					EvalSetID:        10,
					EvalSetVersionID: 100,
					ItemFilter: &entity.ExptItemFilter{
						FilterFields: []*entity.ExptItemFilterField{
							{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"2", "7"}},
						},
					},
				},
			},
		},
	}
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u1"})
	err := exec.exptStartMultiSet(ctx, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u1"}}, expt)
	assert.NoError(t, err)

	assert.ElementsMatch(t, []int64{2, 7}, batchQueried)
	assert.Len(t, captured, 2)
	assert.ElementsMatch(t, []int64{2, 7}, []int64{captured[0].ItemID, captured[1].ItemID})
}

// item_id not_in (排除): List 全集拉回后内存过滤掉 exclude。
func TestExptSubmitExec_exptStartMultiSet_ItemIDExclude(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exec, setItemSvc, itemRefRepo := newExptStartMultiSetTestExec(ctrl)

	var captured []*entity.ExptItemRef
	itemRefRepo.EXPECT().BatchCreate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, refs []*entity.ExptItemRef) error {
			captured = append(captured, refs...)
			return nil
		}).AnyTimes()

	// 排除 item_id=2 → List 拉回 1,2,3, 落库应只剩 1,3
	setItemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return(
		[]*entity.EvaluationSetItem{
			{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}},
			{ItemID: 2, Turns: []*entity.Turn{{ID: 22}}},
			{ItemID: 3, Turns: []*entity.Turn{{ID: 33}}},
		}, ptr.Of(int64(3)), nil, nil, nil).Times(1)
	setItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Times(0)

	expt := &entity.Experiment{
		ID:      1,
		SpaceID: 3,
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{
					EvalSetID:        10,
					EvalSetVersionID: 100,
					ItemFilter: &entity.ExptItemFilter{
						FilterFields: []*entity.ExptItemFilterField{
							{FieldName: "item_id", FieldType: "long", QueryType: "not_in", Values: []string{"2"}},
						},
					},
				},
			},
		},
	}
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u1"})
	err := exec.exptStartMultiSet(ctx, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u1"}}, expt)
	assert.NoError(t, err)

	assert.Len(t, captured, 2)
	assert.ElementsMatch(t, []int64{1, 3}, []int64{captured[0].ItemID, captured[1].ItemID})
}

func TestExtractItemIDFilter(t *testing.T) {
	t.Run("nil filter", func(t *testing.T) {
		inc, exc, hasTag, err := extractItemIDFilter(nil)
		assert.NoError(t, err)
		assert.Empty(t, inc)
		assert.Empty(t, exc)
		assert.False(t, hasTag)
	})

	t.Run("item_id in/eq -> include", func(t *testing.T) {
		f := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"1", "2"}},
			{FieldName: "item_id", FieldType: "long", QueryType: "eq", Values: []string{"3"}},
		}}
		inc, exc, hasTag, err := extractItemIDFilter(f)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2, 3}, inc)
		assert.Empty(t, exc)
		assert.False(t, hasTag)
	})

	t.Run("item_id not_in/not_eq -> exclude", func(t *testing.T) {
		f := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "item_id", FieldType: "long", QueryType: "not_in", Values: []string{"5"}},
			{FieldName: "item_id", FieldType: "long", QueryType: "not_eq", Values: []string{"6"}},
		}}
		inc, exc, _, err := extractItemIDFilter(f)
		assert.NoError(t, err)
		assert.Empty(t, inc)
		assert.ElementsMatch(t, []int64{5, 6}, exc)
	})

	t.Run("tag field -> hasTagFilter, item_id 不受影响", func(t *testing.T) {
		f := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "env", FieldType: "tag", QueryType: "in", Values: []string{"prod"}},
			{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"9"}},
		}}
		inc, _, hasTag, err := extractItemIDFilter(f)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{9}, inc)
		assert.True(t, hasTag)
	})

	t.Run("非法 item_id 值 -> err", func(t *testing.T) {
		f := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"abc"}},
		}}
		_, _, _, err := extractItemIDFilter(f)
		assert.Error(t, err)
	})
}

func TestExtractNormalColumnFilter(t *testing.T) {
	t.Run("nil / 无普通列 -> nil", func(t *testing.T) {
		assert.Nil(t, extractNormalColumnFilter(nil))
		// 只有 item_id + tag, 无普通列
		f := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"1"}},
			{FieldName: "lang", FieldType: "tag", QueryType: "in", Values: []string{"zh"}},
		}}
		assert.Nil(t, extractNormalColumnFilter(f))
	})

	t.Run("混合 -> 只抽普通列, 跳过 item_id/tag", func(t *testing.T) {
		f := &entity.ExptItemFilter{
			QueryAndOr: "and",
			FilterFields: []*entity.ExptItemFilterField{
				{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"1"}},
				{FieldName: "lang", FieldType: "tag", QueryType: "in", Values: []string{"zh"}},
				{FieldName: "category", FieldType: "string", QueryType: "match", Values: []string{"math"}},
				{FieldName: "difficulty", FieldType: "integer", QueryType: "not_eq", Values: []string{"5"}},
			},
		}
		out := extractNormalColumnFilter(f)
		assert.NotNil(t, out)
		assert.Len(t, out.FilterFields, 2)
		assert.Equal(t, "and", ptr.From(out.QueryAndOr))
		assert.Equal(t, "category", out.FilterFields[0].FieldName)
		assert.Equal(t, "match", ptr.From(out.FilterFields[0].QueryType))
		assert.Equal(t, "difficulty", out.FilterFields[1].FieldName)
	})
}

func TestExtractTagFilter(t *testing.T) {
	t.Run("nil / 无 tag -> nil", func(t *testing.T) {
		assert.Nil(t, extractTagFilter(nil))
		f := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "category", FieldType: "string", QueryType: "match", Values: []string{"math"}},
		}}
		assert.Nil(t, extractTagFilter(f))
	})

	t.Run("抽 tag values 扁平收集, relation 默认 or", func(t *testing.T) {
		f := &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{
			{FieldName: "lang", FieldType: "tag", QueryType: "in", Values: []string{"zh", "en"}},
			{FieldName: "level", FieldType: "tag", QueryType: "in", Values: []string{"hard"}},
		}}
		out := extractTagFilter(f)
		assert.NotNil(t, out)
		assert.ElementsMatch(t, []string{"zh", "en", "hard"}, out.TagNames)
		assert.Equal(t, entity.TagFilterRelationOr, out.Relation)
	})

	t.Run("query_and_or=and -> relation And", func(t *testing.T) {
		f := &entity.ExptItemFilter{
			QueryAndOr: "and",
			FilterFields: []*entity.ExptItemFilterField{
				{FieldName: "lang", FieldType: "tag", QueryType: "in", Values: []string{"zh"}},
			},
		}
		out := extractTagFilter(f)
		assert.NotNil(t, out)
		assert.Equal(t, entity.TagFilterRelationAnd, out.Relation)
	})
}
