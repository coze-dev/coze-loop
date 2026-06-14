// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// MultiSetConfig 实验类型在 CreateExptAggrResult 里跳过 evaluator 维度计算 —
// 不调用 GetTurnEvaluatorResultRefByExptID 也不调用 BatchGetEvaluatorRecord;
// target 维度照常 (这里 mock 返回空 turn_results, 等价于没数据)。
func TestCreateExptAggrResult_MultiSetConfig_SkipsEvaluatorGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)

	mockExptAggrResultRepo.EXPECT().
		GetExptAggrResultByExperimentID(gomock.Any(), gomock.Any()).
		Return([]*entity.ExptAggrResult{}, nil)
	mockExperimentRepo.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.Experiment{
			ID:                1,
			EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		}, nil).AnyTimes()
	// target 维度: 空 turn_results, 但 buildAggrResult 仍会为 4 个 target 指标构造 aggr_result(score=0)
	mockExptTurnResultRepo.EXPECT().
		ScanTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptTurnResult{}, int64(0), nil).AnyTimes()

	// ★ 关键断言: 写入的 aggr_results 只含 Target 类, 不含 EvaluatorScore 或 WeightedScore
	mockExptAggrResultRepo.EXPECT().
		BatchCreateExptAggrResult(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, results []*entity.ExptAggrResult) error {
			for _, r := range results {
				assert.NotEqual(t, int32(entity.FieldType_EvaluatorScore), r.FieldType,
					"MultiSetConfig 实验不应写 EvaluatorScore 行")
				assert.NotEqual(t, int32(entity.FieldType_WeightedScore), r.FieldType,
					"MultiSetConfig 实验不应写 WeightedScore 行")
			}
			return nil
		}).AnyTimes()
	// 锁解锁
	mockLocker := lockMocks.NewMockILocker(ctrl)
	mockLocker.EXPECT().Unlock(gomock.Any()).Return(true, nil).AnyTimes()
	mockMetric := metricsMocks.NewMockExptMetric(ctrl)
	mockMetric.EXPECT().EmitCalculateExptAggrResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// ★ 不 EXPECT GetTurnEvaluatorResultRefByExptID / BatchGetEvaluatorRecord —
	// gomock 严格模式下,任一次未预期的调用都会让测试失败
	svc := &ExptAggrResultServiceImpl{
		exptAggrResultRepo: mockExptAggrResultRepo,
		exptTurnResultRepo: mockExptTurnResultRepo,
		experimentRepo:     mockExperimentRepo,
		locker:             mockLocker,
		metric:             mockMetric,
	}

	err := svc.CreateExptAggrResult(context.Background(), int64(100), int64(1))
	assert.NoError(t, err)
}

// MultiSetConfig 实验类型 createWeightedScoreAggrResult 直接返回 nil —
// 不调用 ScanTurnResults
func TestCreateWeightedScoreAggrResult_MultiSetConfig_ReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExperimentRepo.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.Experiment{
			ID:                1,
			EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		}, nil)

	// ★ 不 EXPECT exptTurnResultRepo —— 任一次调用 (ScanTurnResults) 会让测试失败
	svc := &ExptAggrResultServiceImpl{
		experimentRepo: mockExperimentRepo,
	}

	result, err := svc.createWeightedScoreAggrResult(context.Background(), int64(100), int64(1))
	assert.NoError(t, err)
	assert.Nil(t, result)
}

// MultiSetConfig 实验类型 UpdateExptAggrResult 入口直接返回 nil —
// 不调用 GetExptAggrResult / UpdateAndGetLatestVersion 等任何后续逻辑
func TestUpdateExptAggrResult_MultiSetConfig_ReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExperimentRepo.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.Experiment{
			ID:                1,
			EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		}, nil)

	mockMetric := metricsMocks.NewMockExptMetric(ctrl)
	mockMetric.EXPECT().EmitCalculateExptAggrResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	// ★ 不 EXPECT exptAggrResultRepo —— 任一次调用会让测试失败
	svc := &ExptAggrResultServiceImpl{
		experimentRepo: mockExperimentRepo,
		metric:         mockMetric,
	}

	err := svc.UpdateExptAggrResult(context.Background(), &entity.UpdateExptAggrResultParam{
		SpaceID:      100,
		ExperimentID: 1,
		FieldType:    entity.FieldType_EvaluatorScore,
		FieldKey:     "1",
	})
	assert.NoError(t, err)
}

// computeEvaluatorAggrGroup 过滤 Status=Skipped 的占位 record —
// filter 不命中产生的 Skipped record 不参与聚合分数计算
func TestComputeEvaluatorAggrGroup_FiltersSkippedRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

	// 同一 evaluator (versionID=100) 有两条 record: 1 条 Success 0.8 分, 1 条 Skipped
	mockExptTurnResultRepo.EXPECT().
		GetTurnEvaluatorResultRefByExptID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptTurnEvaluatorResultRef{
			{EvaluatorVersionID: 100, EvaluatorResultID: 11},
			{EvaluatorVersionID: 100, EvaluatorResultID: 12},
		}, nil)

	mockEvaluatorRecordService.EXPECT().
		BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.EvaluatorRecord{
			{
				ID:     11,
				Status: entity.EvaluatorRunStatusSuccess,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.8)},
				},
			},
			{
				ID:     12,
				Status: entity.EvaluatorRunStatusSkipped, // ★ 应被过滤
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.2)},
				},
			},
		}, nil)

	svc := &ExptAggrResultServiceImpl{
		exptTurnResultRepo:     mockExptTurnResultRepo,
		evaluatorRecordService: mockEvaluatorRecordService,
	}

	group, err := svc.computeEvaluatorAggrGroup(context.Background(), int64(100), int64(1))
	assert.NoError(t, err)
	assert.Contains(t, group, int64(100))

	// 验证只 Success 的 0.8 被纳入聚合, Skipped 的 0.2 被丢弃
	aggrResult := group[100].Result()
	var average float64
	for _, r := range aggrResult.AggregatorResults {
		if r.AggregatorType == entity.Average {
			average = r.GetScore()
			break
		}
	}
	assert.InDelta(t, 0.8, average, 1e-9)
}
