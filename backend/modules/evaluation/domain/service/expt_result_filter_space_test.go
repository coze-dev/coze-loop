// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// TestExptResultServiceImpl_ListTurnResult_CarriesEvalSetSpaceID
// 钉死 ListTurnResult 把实验冻结的 EvalSetSpaceID 装进 accelerator 这一步。
//
// 为什么单列一条：这是跨空间共享链条上「唯一把来源空间从实验行搬进筛选条件」的地方，
// 少了它，下游 repo → DAO 再怎么正确也只能拿实验空间去查评测集快照表 → dis 恒 0 行 →
// 两阶段查询第一阶段早退 → 所有评测集列筛选静默返回 0 条。
// 该字段是纯赋值，编译器和其它用例都盖不住，只能在这里断言。
func TestExptResultServiceImpl_ListTurnResult_CarriesEvalSetSpaceID(t *testing.T) {
	const (
		consumerSpaceID = int64(7590110994886651906) // 实验(消费方)空间
		srcSpaceID      = int64(7590106916835897090) // 评测集来源空间
		exptID          = int64(1)
	)

	// newSvc 组装一套最小依赖：只让 accelerator 分支跑到 QueryItemIDStates 为止。
	// captured 收下真正传给 repo 的 accelerator。
	newSvc := func(t *testing.T, ctrl *gomock.Controller, captured **entity.ExptTurnResultFilterAccelerator) ExptResultServiceImpl {
		t.Helper()
		filterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
		filterRepo.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, acc *entity.ExptTurnResultFilterAccelerator) (map[int64]entity.ItemRunState, int64, error) {
				*captured = acc
				// 返回空集合让 ListTurnResult 就地收敛，避免牵扯后续排序/组装依赖
				return map[int64]entity.ItemRunState{}, 0, nil
			}).Times(1)

		evalSetSvc := svcMocks.NewMockIEvaluationSetService(ctrl)
		evalSetSvc.EXPECT().QueryItemSnapshotMappings(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *rpc.QueryItemSnapshotMappingRequest) ([]*entity.ItemSnapshotFieldMapping, string, error) {
				// mapping 也必须按来源空间查（与 accelerator 同口径），顺手一起钉
				assert.Equal(t, srcSpaceID, req.SpaceID, "mapping RPC 也必须用评测集来源空间")
				return []*entity.ItemSnapshotFieldMapping{
					{FieldKey: "my_schema_field", MappingKey: "string_map", MappingSubKey: "string_key_0"},
				}, "2026-08-18", nil
			}).AnyTimes()

		metric := metricsMocks.NewMockExptMetric(ctrl)
		metric.EXPECT().EmitExptTurnResultFilterQueryLatency(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		return ExptResultServiceImpl{
			exptTurnResultFilterRepo: filterRepo,
			evaluationSetService:     evalSetSvc,
			Metric:                   metric,
		}
	}

	now := time.Now()
	buildParam := func() *entity.MGetExperimentResultParam {
		return &entity.MGetExperimentResultParam{
			SpaceID:        consumerSpaceID,
			ExptIDs:        []int64{exptID},
			BaseExptID:     gptr.Of(exptID),
			UseAccelerator: true,
			Page:           entity.NewPage(1, 20),
			FilterAccelerators: map[int64]*entity.ExptTurnResultFilterAccelerator{
				exptID: {
					ItemSnapshotCond: &entity.ItemSnapshotFilter{
						StringMapFilters: []*entity.FieldFilter{{Key: "my_schema_field", Op: "=", Values: []any{"v"}}},
					},
					KeywordSearch: &entity.KeywordFilter{ItemSnapshotFilter: &entity.ItemSnapshotFilter{}},
				},
			},
		}
	}

	t.Run("跨空间共享: accelerator 带上评测集来源空间，实验空间不被覆盖", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var captured *entity.ExptTurnResultFilterAccelerator
		svc := newSvc(t, ctrl, &captured)

		expt := &entity.Experiment{
			ID: exptID, SpaceID: consumerSpaceID, ExptType: entity.ExptType_Offline,
			EvalSetID: 2, EvalSetVersionID: 3, EvalSetSpaceID: srcSpaceID, StartAt: &now,
		}
		_, _, _, err := svc.ListTurnResult(context.Background(), buildParam(), expt)
		require.NoError(t, err)

		require.NotNil(t, captured)
		assert.Equal(t, srcSpaceID, captured.EvalSetSpaceID, "EvalSetSpaceID 必须来自实验冻结的评测集来源空间")
		assert.Equal(t, consumerSpaceID, captured.SpaceID, "SpaceID 仍必须是实验空间（用于查 etrf 实验结果表）")
		// sync_ck_date 是快照表分区列，必须随 mapping 一起被带出来
		assert.Equal(t, "2026-08-18", captured.EvalSetSyncCkDate)
	})

	t.Run("同空间: EvalSetSpaceID 保持 0（下游据此回落实验空间）", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var captured *entity.ExptTurnResultFilterAccelerator

		// 同空间场景 mapping 必须用实验空间查，这里单独装一套断言
		filterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
		filterRepo.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, acc *entity.ExptTurnResultFilterAccelerator) (map[int64]entity.ItemRunState, int64, error) {
				captured = acc
				return map[int64]entity.ItemRunState{}, 0, nil
			}).Times(1)
		evalSetSvc := svcMocks.NewMockIEvaluationSetService(ctrl)
		evalSetSvc.EXPECT().QueryItemSnapshotMappings(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *rpc.QueryItemSnapshotMappingRequest) ([]*entity.ItemSnapshotFieldMapping, string, error) {
				assert.Equal(t, consumerSpaceID, req.SpaceID, "非共享时 mapping 必须用实验空间")
				return []*entity.ItemSnapshotFieldMapping{
					{FieldKey: "my_schema_field", MappingKey: "string_map", MappingSubKey: "string_key_0"},
				}, "2026-08-18", nil
			}).Times(1)
		metric := metricsMocks.NewMockExptMetric(ctrl)
		metric.EXPECT().EmitExptTurnResultFilterQueryLatency(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		svc := ExptResultServiceImpl{
			exptTurnResultFilterRepo: filterRepo,
			evaluationSetService:     evalSetSvc,
			Metric:                   metric,
		}

		expt := &entity.Experiment{
			ID: exptID, SpaceID: consumerSpaceID, ExptType: entity.ExptType_Offline,
			EvalSetID: 2, EvalSetVersionID: 3, EvalSetSpaceID: 0, StartAt: &now,
		}
		_, _, _, err := svc.ListTurnResult(context.Background(), buildParam(), expt)
		require.NoError(t, err)

		require.NotNil(t, captured)
		assert.Zero(t, captured.EvalSetSpaceID, "同空间必须保持 0，不能被回填成实验空间（那会让下游误判为跨空间）")
		assert.Equal(t, consumerSpaceID, captured.SpaceID)
	})
}
