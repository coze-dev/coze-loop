// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func newEvaluatorRecord(versionID int64, score *float64, correction *float64) *entity.EvaluatorRecord {
	result := &entity.EvaluatorResult{Score: score}
	if correction != nil {
		result.Correction = &entity.Correction{Score: correction}
	}
	return &entity.EvaluatorRecord{
		EvaluatorVersionID: versionID,
		EvaluatorOutputData: &entity.EvaluatorOutputData{
			EvaluatorResult: result,
		},
	}
}

func TestEffectiveEvaluatorScore(t *testing.T) {
	assert.Nil(t, effectiveEvaluatorScore(nil))
	assert.Nil(t, effectiveEvaluatorScore(&entity.EvaluatorRecord{}))

	// 仅原始分
	assert.Equal(t, 0.8, *effectiveEvaluatorScore(newEvaluatorRecord(101, gptr.Of(0.8), nil)))
	// 修正分优先
	assert.Equal(t, 0.5, *effectiveEvaluatorScore(newEvaluatorRecord(101, gptr.Of(0.8), gptr.Of(0.5))))
}

func TestBuildCaseScoreRequest(t *testing.T) {
	t.Run("空记录返回 nil", func(t *testing.T) {
		assert.Nil(t, buildCaseScoreRequest(&entity.Experiment{}, nil))
	})

	t.Run("从实验实体取评估器名称与ID", func(t *testing.T) {
		expt := &entity.Experiment{
			ID: 9,
			Evaluators: []*entity.Evaluator{
				{
					ID:            1001,
					Name:          "cozeclaw-qa-factuality",
					EvaluatorType: entity.EvaluatorTypePrompt,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						ID:          101,
						EvaluatorID: 1001,
					},
				},
			},
		}
		version2Record := map[int64]*entity.EvaluatorRecord{
			101: newEvaluatorRecord(101, gptr.Of(0.9), nil),
		}

		req := buildCaseScoreRequest(expt, version2Record)
		assert.NotNil(t, req)
		assert.Equal(t, int64(9), req.ExptID)
		assert.Len(t, req.EvaluatorScore, 1)
		item := req.EvaluatorScore[0]
		assert.Equal(t, "cozeclaw-qa-factuality", item.EvaluatorName)
		assert.Equal(t, int64(1001), item.EvaluatorID)
		assert.Equal(t, int64(101), item.EvaluatorVersionID)
		assert.Equal(t, 0.9, item.Score)
	})

	t.Run("无有效分数的评估器被跳过", func(t *testing.T) {
		version2Record := map[int64]*entity.EvaluatorRecord{
			101: {EvaluatorVersionID: 101}, // 无 output
		}
		req := buildCaseScoreRequest(&entity.Experiment{ID: 1}, version2Record)
		assert.NotNil(t, req)
		assert.Empty(t, req.EvaluatorScore)
	})
}

func TestExptResultServiceImpl_computeTurnWeightedScore(t *testing.T) {
	ctx := context.Background()
	expt := &entity.Experiment{ID: 1, SpaceID: 100}
	version2Record := map[int64]*entity.EvaluatorRecord{
		101: newEvaluatorRecord(101, gptr.Of(0.6), nil),
		102: newEvaluatorRecord(102, gptr.Of(0.8), nil),
	}

	t.Run("未命中回退本地等权计算", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockConfiger := componentmocks.NewMockIConfiger(ctrl)
		mockConfiger.EXPECT().GetExptTurnScoreHookConf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, false)
		svc := ExptResultServiceImpl{configer: mockConfiger}
		got := svc.computeTurnWeightedScore(ctx, expt, version2Record, nil)
		assert.NotNil(t, got)
		assert.Equal(t, 0.7, *got) // (0.6+0.8)/2
	})

	t.Run("configer 为 nil 时回退本地计算", func(t *testing.T) {
		svc := ExptResultServiceImpl{}
		got := svc.computeTurnWeightedScore(ctx, expt, version2Record, nil)
		assert.NotNil(t, got)
		assert.Equal(t, 0.7, *got)
	})

	t.Run("命中但 HTTP 回调未实现返回 nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockConfiger := componentmocks.NewMockIConfiger(ctrl)
		mockConfiger.EXPECT().GetExptTurnScoreHookConf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&entity.ExptTurnScoreHookConf{URL: "http://x", Method: "POST", TimeoutMS: 1000}, true)
		svc := ExptResultServiceImpl{configer: mockConfiger}
		got := svc.computeTurnWeightedScore(ctx, expt, version2Record, nil)
		assert.Nil(t, got)
	})

	t.Run("命中但无有效评估器分数返回 nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockConfiger := componentmocks.NewMockIConfiger(ctrl)
		mockConfiger.EXPECT().GetExptTurnScoreHookConf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&entity.ExptTurnScoreHookConf{URL: "http://x"}, true)
		svc := ExptResultServiceImpl{configer: mockConfiger}
		emptyRecords := map[int64]*entity.EvaluatorRecord{101: {EvaluatorVersionID: 101}}
		got := svc.computeTurnWeightedScore(ctx, expt, emptyRecords, nil)
		assert.Nil(t, got)
	})
}
