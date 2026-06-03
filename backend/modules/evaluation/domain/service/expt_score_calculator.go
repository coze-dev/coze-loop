// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/http"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/utils"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate mockgen -destination=mocks/expt_score_calculator.go -package=mocks . IEvaluatorScoreCalculator

// IEvaluatorScoreCalculator 计算某一行（turn）的行维度（多评估器）加权得分。
type IEvaluatorScoreCalculator interface {
	// CalculateWeightedScore 计算某一行（turn）的行维度得分。
	// 优先经 configer.GetExptTurnScoreHookConf 判定是否走 HTTP 回调：命中则组装请求并调用 HTTP 打分，
	// 调用失败或拿不到分数时返回 nil（即不写入 weighted_score）；未命中则回退本地等权/加权计算。
	CalculateWeightedScore(ctx context.Context, expt *entity.Experiment,
		version2Record map[int64]*entity.EvaluatorRecord, scoreWeights map[int64]float64) *float64
}

// NewEvaluatorScoreCalculator 构造行维度得分计算器。
func NewEvaluatorScoreCalculator(configer component.IConfiger, httpClient http.IClient) IEvaluatorScoreCalculator {
	return &evaluatorScoreCalculator{
		configer:   configer,
		httpClient: httpClient,
	}
}

type evaluatorScoreCalculator struct {
	configer   component.IConfiger
	httpClient http.IClient
}

func (c *evaluatorScoreCalculator) CalculateWeightedScore(ctx context.Context, expt *entity.Experiment,
	version2Record map[int64]*entity.EvaluatorRecord, scoreWeights map[int64]float64,
) *float64 {
	var (
		spaceID int64
		exptID  int64
	)
	if expt != nil {
		spaceID = expt.SpaceID
		exptID = expt.ID
	}

	evaluatorRefs := buildEvaluatorVersionRefs(expt, version2Record)

	var (
		hookConf *entity.ExptTurnScoreHookConf
		hit      bool
	)
	if c.configer != nil {
		hookConf, hit = c.configer.GetExptTurnScoreHookConf(ctx, spaceID, exptID, evaluatorRefs)
	}
	if !hit {
		return calculateWeightedScore(version2Record, scoreWeights)
	}

	req := buildCaseScoreRequest(expt, version2Record)
	if req == nil || len(req.EvaluatorScore) == 0 {
		return nil
	}

	score, err := c.callCaseScoreHook(ctx, hookConf, req)
	if err != nil {
		logs.CtxError(ctx, "[ExptEval] case score hook failed, expt_id=%v, err=%v", exptID, err)
		return nil
	}
	if score == nil {
		return nil
	}
	rounded := utils.RoundScoreToTwoDecimals(*score)
	return &rounded
}

// callCaseScoreHook 调用行维度得分 HTTP 回调对该行多个评估器分数做聚合，返回行维度得分。
// 命中配置（URL/Method/Timeout）经注入的 http.IClient 发起请求；
// /score/case 即使逻辑异常也返回 200，需同时检查响应体 error 字段。
func (c *evaluatorScoreCalculator) callCaseScoreHook(ctx context.Context, conf *entity.ExptTurnScoreHookConf, req *entity.CaseScoreRequest) (*float64, error) {
	if c.httpClient == nil {
		return nil, errorx.New("case score http client not injected")
	}
	if conf == nil || conf.URL == "" {
		return nil, errorx.New("case score hook conf invalid")
	}

	method := conf.Method
	if method == "" {
		method = "POST"
	}

	resp := &entity.CaseScoreResponse{}
	param := &http.RequestParam{
		RequestURI: conf.URL,
		Method:     method,
		Header:     map[string]string{"Content-Type": "application/json"},
		Body:       req,
		Response:   resp,
		Timeout:    time.Duration(conf.TimeoutMS) * time.Millisecond,
	}
	if err := c.httpClient.DoHTTPRequest(ctx, param); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errorx.New("case score hook returned error: %s", resp.Error)
	}
	return &resp.Score, nil
}

func buildEvaluatorVersionRefs(expt *entity.Experiment, version2Record map[int64]*entity.EvaluatorRecord) []*entity.ExptEvaluatorVersionRef {
	if len(version2Record) == 0 {
		return nil
	}

	versionID2EvaluatorID := make(map[int64]int64)
	if expt != nil {
		for _, ev := range expt.Evaluators {
			if ev == nil {
				continue
			}
			versionID2EvaluatorID[ev.GetEvaluatorVersionID()] = ev.GetEvaluatorID()
		}
	}

	refs := make([]*entity.ExptEvaluatorVersionRef, 0, len(version2Record))
	for versionID := range version2Record {
		refs = append(refs, &entity.ExptEvaluatorVersionRef{
			EvaluatorID:        versionID2EvaluatorID[versionID],
			EvaluatorVersionID: versionID,
		})
	}
	return refs
}

// buildCaseScoreRequest 基于实验实体与本行评估器记录组装 /score/case 请求。
// 评估器名称取自实验实体中的评估器实体（expt.Evaluators）。
func buildCaseScoreRequest(expt *entity.Experiment, version2Record map[int64]*entity.EvaluatorRecord) *entity.CaseScoreRequest {
	if len(version2Record) == 0 {
		return nil
	}

	type evaluatorMeta struct {
		name        string
		evaluatorID int64
	}
	versionID2Meta := make(map[int64]evaluatorMeta)
	if expt != nil {
		for _, ev := range expt.Evaluators {
			if ev == nil {
				continue
			}
			versionID2Meta[ev.GetEvaluatorVersionID()] = evaluatorMeta{
				name:        ev.Name,
				evaluatorID: ev.GetEvaluatorID(),
			}
		}
	}

	req := &entity.CaseScoreRequest{EvaluatorScore: make([]*entity.CaseScoreItem, 0, len(version2Record))}
	if expt != nil {
		req.ExptID = expt.ID
	}

	for versionID, record := range version2Record {
		score := effectiveEvaluatorScore(record)
		if score == nil {
			continue
		}
		meta := versionID2Meta[versionID]
		req.EvaluatorScore = append(req.EvaluatorScore, &entity.CaseScoreItem{
			EvaluatorName:      meta.name,
			EvaluatorID:        meta.evaluatorID,
			EvaluatorVersionID: versionID,
			Score:              *score,
		})
	}

	return req
}

// effectiveEvaluatorScore 取评估器记录的有效分数：优先修正分，其次原始分。
func effectiveEvaluatorScore(record *entity.EvaluatorRecord) *float64 {
	if record == nil || record.EvaluatorOutputData == nil || record.EvaluatorOutputData.EvaluatorResult == nil {
		return nil
	}
	result := record.EvaluatorOutputData.EvaluatorResult
	if result.Correction != nil && result.Correction.Score != nil {
		return result.Correction.Score
	}
	return result.Score
}

// calculateWeightedScore 计算加权分数
func calculateWeightedScore(
	evaluatorRecords map[int64]*entity.EvaluatorRecord,
	weights map[int64]float64,
) *float64 {
	if len(evaluatorRecords) == 0 {
		return nil
	}

	// 如果未配置权重（weights 为空），则按所有评估器权重相同计算加权分（即简单平均）
	if len(weights) == 0 {
		var (
			sumScore float64
			cnt      int
		)
		for _, record := range evaluatorRecords {
			if record == nil {
				continue
			}
			// 获取评估器分数（优先使用修正分数）
			var score *float64
			if record.EvaluatorOutputData != nil && record.EvaluatorOutputData.EvaluatorResult != nil {
				if record.EvaluatorOutputData.EvaluatorResult.Correction != nil &&
					record.EvaluatorOutputData.EvaluatorResult.Correction.Score != nil {
					score = record.EvaluatorOutputData.EvaluatorResult.Correction.Score
				} else if record.EvaluatorOutputData.EvaluatorResult.Score != nil {
					score = record.EvaluatorOutputData.EvaluatorResult.Score
				}
			}
			if score == nil {
				continue
			}
			sumScore += *score
			cnt++
		}
		if cnt == 0 {
			return nil
		}
		avg := sumScore / float64(cnt)
		roundedAvg := utils.RoundScoreToTwoDecimals(avg)
		return &roundedAvg
	}

	var totalWeightedScore float64
	var totalWeight float64
	hasValidScore := false

	for evaluatorVersionID, record := range evaluatorRecords {
		if record == nil {
			continue
		}

		// 获取评估器分数（优先使用修正分数）
		var score *float64
		if record.EvaluatorOutputData != nil && record.EvaluatorOutputData.EvaluatorResult != nil {
			if record.EvaluatorOutputData.EvaluatorResult.Correction != nil &&
				record.EvaluatorOutputData.EvaluatorResult.Correction.Score != nil {
				score = record.EvaluatorOutputData.EvaluatorResult.Correction.Score
			} else if record.EvaluatorOutputData.EvaluatorResult.Score != nil {
				score = record.EvaluatorOutputData.EvaluatorResult.Score
			}
		}

		// 如果没有有效分数，跳过
		if score == nil {
			continue
		}

		// 获取权重（0 合法：不参与分子/分母，等价于乘 0）
		weight, ok := weights[evaluatorVersionID]
		if !ok || weight <= 0 {
			continue
		}

		// 累加加权分数
		totalWeightedScore += *score * weight
		totalWeight += weight
		hasValidScore = true
	}

	// 如果没有有效分数或权重总和为0，返回nil
	if !hasValidScore || totalWeight <= 0 {
		return nil
	}

	// 计算加权平均分数
	weightedScore := totalWeightedScore / totalWeight
	roundedScore := utils.RoundScoreToTwoDecimals(weightedScore)
	return &roundedScore
}
