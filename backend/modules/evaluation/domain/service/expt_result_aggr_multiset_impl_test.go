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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// T2: MultiSetConfig 实验恢复 evaluator 维度聚合, 并按 (version_id, alias) 分桶。
// CreateExptAggrResult 不再跳过 evaluator 维度计算 —— 同 version 多 alias 各自独立成桶,
// field_key 用 EncodeEvaluatorInstanceKey 编码 (verID:alias)。
func TestCreateExptAggrResult_MultiSetConfig_ComputesEvaluatorGroupByAlias(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

	mockExptAggrResultRepo.EXPECT().
		GetExptAggrResultByExperimentID(gomock.Any(), gomock.Any()).
		Return([]*entity.ExptAggrResult{}, nil)
	mockExperimentRepo.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.Experiment{
			ID:                1,
			EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		}, nil).AnyTimes()

	// computeEvaluatorAggrGroup: 同 version=100, 两个 alias (judge_A / judge_B) 各一条 record
	mockExptTurnResultRepo.EXPECT().
		GetTurnEvaluatorResultRefByExptID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptTurnEvaluatorResultRef{
			{EvaluatorVersionID: 100, EvaluatorResultID: 11, Alias: "judge_A"},
			{EvaluatorVersionID: 100, EvaluatorResultID: 12, Alias: "judge_B"},
		}, nil)
	mockEvaluatorRecordService.EXPECT().
		BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.EvaluatorRecord{
			{
				ID:                  11,
				Status:              entity.EvaluatorRunStatusSuccess,
				EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.8)}},
			},
			{
				ID:                  12,
				Status:              entity.EvaluatorRunStatusSuccess,
				EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.4)}},
			},
		}, nil)

	// weighted 维度: 空成功轮次 -> 不写 WeightedScore
	mockExptTurnResultRepo.EXPECT().
		ScanTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptTurnResult{}, int64(0), nil).AnyTimes()

	// ★ 关键断言: 写入两条 EvaluatorScore 行, field_key 分别为 "100:judge_A" / "100:judge_B"
	mockExptAggrResultRepo.EXPECT().
		BatchCreateExptAggrResult(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, results []*entity.ExptAggrResult) error {
			fieldKeys := map[string]bool{}
			for _, r := range results {
				if r.FieldType == int32(entity.FieldType_EvaluatorScore) {
					fieldKeys[r.FieldKey] = true
				}
			}
			assert.True(t, fieldKeys["100:judge_A"], "应写 100:judge_A 桶")
			assert.True(t, fieldKeys["100:judge_B"], "应写 100:judge_B 桶")
			return nil
		}).AnyTimes()

	mockLocker := lockMocks.NewMockILocker(ctrl)
	mockLocker.EXPECT().Unlock(gomock.Any()).Return(true, nil).AnyTimes()
	mockMetric := metricsMocks.NewMockExptMetric(ctrl)
	mockMetric.EXPECT().EmitCalculateExptAggrResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	svc := &ExptAggrResultServiceImpl{
		exptAggrResultRepo:     mockExptAggrResultRepo,
		exptTurnResultRepo:     mockExptTurnResultRepo,
		experimentRepo:         mockExperimentRepo,
		evaluatorRecordService: mockEvaluatorRecordService,
		locker:                 mockLocker,
		metric:                 mockMetric,
	}

	err := svc.CreateExptAggrResult(context.Background(), int64(100), int64(1))
	assert.NoError(t, err)
}

// T2: MultiSetConfig 实验 createWeightedScoreAggrResult 不再跳过 ——
// 行级 weighted_score 已按 (version, alias) 算对, 这里做实验级 avg 汇总。
func TestCreateWeightedScoreAggrResult_MultiSetConfig_Computes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptTurnResultRepo.EXPECT().
		ScanTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptTurnResult{
			{WeightedScore: gptr.Of(0.6)},
		}, int64(0), nil)

	svc := &ExptAggrResultServiceImpl{
		exptTurnResultRepo: mockExptTurnResultRepo,
	}

	result, err := svc.createWeightedScoreAggrResult(context.Background(), int64(100), int64(1))
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int32(entity.FieldType_WeightedScore), result.FieldType)
}

// T2: MultiSetConfig 实验 UpdateExptAggrResult 入口不再防御性早返回 ——
// 继续走 GetExptAggrResult 等后续逻辑 (此处实验未完成 + NotFound -> 静默返回 nil)。
func TestUpdateExptAggrResult_MultiSetConfig_Proceeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExperimentRepo.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.Experiment{
			ID:                1,
			EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
			Status:            entity.ExptStatus_Processing,
		}, nil)

	mockExptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)
	// ★ 不再早返回: 后续逻辑会查 GetExptAggrResult; 返回 NotFound + 实验未完成 -> 静默返回 nil (无 MQ 重试)
	mockExptAggrResultRepo.EXPECT().
		GetExptAggrResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))

	mockMetric := metricsMocks.NewMockExptMetric(ctrl)
	mockMetric.EXPECT().EmitCalculateExptAggrResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	svc := &ExptAggrResultServiceImpl{
		experimentRepo:     mockExperimentRepo,
		exptAggrResultRepo: mockExptAggrResultRepo,
		metric:             mockMetric,
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
// filter 不命中产生的 Skipped record 不参与聚合分数计算。
// alias 为空时 instanceKey 退化为裸 versionID 字符串 "100"。
func TestComputeEvaluatorAggrGroup_FiltersSkippedRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

	// 同一 evaluator (versionID=100, alias 空) 有两条 record: 1 条 Success 0.8 分, 1 条 Skipped
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
	assert.Contains(t, group, "100")

	// 验证只 Success 的 0.8 被纳入聚合, Skipped 的 0.2 被丢弃
	aggrResult := group["100"].Result()
	var average float64
	for _, r := range aggrResult.AggregatorResults {
		if r.AggregatorType == entity.Average {
			average = r.GetScore()
			break
		}
	}
	assert.InDelta(t, 0.8, average, 1e-9)
}

// computeEvaluatorAggrGroup 按 (version_id, alias) 分桶 —
// 同 version 多 alias 不再撞 key, 各自独立成桶。
func TestComputeEvaluatorAggrGroup_BucketsByAlias(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

	mockExptTurnResultRepo.EXPECT().
		GetTurnEvaluatorResultRefByExptID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptTurnEvaluatorResultRef{
			{EvaluatorVersionID: 100, EvaluatorResultID: 11, Alias: "judge_A"},
			{EvaluatorVersionID: 100, EvaluatorResultID: 12, Alias: "judge_B"},
		}, nil)

	mockEvaluatorRecordService.EXPECT().
		BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.EvaluatorRecord{
			{
				ID:                  11,
				Status:              entity.EvaluatorRunStatusSuccess,
				EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.8)}},
			},
			{
				ID:                  12,
				Status:              entity.EvaluatorRunStatusSuccess,
				EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.4)}},
			},
		}, nil)

	svc := &ExptAggrResultServiceImpl{
		exptTurnResultRepo:     mockExptTurnResultRepo,
		evaluatorRecordService: mockEvaluatorRecordService,
	}

	group, err := svc.computeEvaluatorAggrGroup(context.Background(), int64(100), int64(1))
	assert.NoError(t, err)
	assert.Contains(t, group, "100:judge_A")
	assert.Contains(t, group, "100:judge_B")

	avgOf := func(g *AggregatorGroup) float64 {
		for _, r := range g.Result().AggregatorResults {
			if r.AggregatorType == entity.Average {
				return r.GetScore()
			}
		}
		return 0
	}
	assert.InDelta(t, 0.8, avgOf(group["100:judge_A"]), 1e-9)
	assert.InDelta(t, 0.4, avgOf(group["100:judge_B"]), 1e-9)
}
