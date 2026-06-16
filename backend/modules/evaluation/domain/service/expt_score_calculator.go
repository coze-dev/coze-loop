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
	//
	// version2Record / scoreWeights 的 key 为评估器实例 key (entity.EncodeEvaluatorInstanceKey(versionID, alias)):
	// alias 为空时退化为裸 versionID 字符串, 与旧行为 byte 级一致。
	CalculateWeightedScore(ctx context.Context, expt *entity.Experiment,
		version2Record map[string]*entity.EvaluatorRecord, scoreWeights map[string]float64) *float64
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
	version2Record map[string]*entity.EvaluatorRecord, scoreWeights map[string]float64,
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

func buildEvaluatorVersionRefs(expt *entity.Experiment, version2Record map[string]*entity.EvaluatorRecord) []*entity.ExptEvaluatorVersionRef {
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
	for instanceKey, record := range version2Record {
		if record == nil {
			continue
		}
		// 从实例 key 解析回 versionID (兼容 verID 与 verID:alias); 解析失败回退 record 字段
		versionID, _, err := entity.ParseEvaluatorScoreFieldKey(instanceKey)
		if err != nil {
			versionID = record.EvaluatorVersionID
		}
		refs = append(refs, &entity.ExptEvaluatorVersionRef{
			EvaluatorID:        versionID2EvaluatorID[versionID],
			EvaluatorVersionID: versionID,
		})
	}
	return refs
}

// buildCaseScoreRequest 基于实验实体与本行评估器记录组装 /score/case 请求。
// 评估器名称取自实验实体中的评估器实体（expt.Evaluators）。
func buildCaseScoreRequest(expt *entity.Experiment, version2Record map[string]*entity.EvaluatorRecord) *entity.CaseScoreRequest {
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

	for instanceKey, record := range version2Record {
		score := effectiveEvaluatorScore(record)
		if score == nil {
			continue
		}
		// 从实例 key 解析回 versionID (兼容 verID 与 verID:alias); 解析失败回退 record 字段
		versionID, _, err := entity.ParseEvaluatorScoreFieldKey(instanceKey)
		if err != nil {
			versionID = record.EvaluatorVersionID
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

// buildScoreWeights 按实验类型分流构建行维度加权计算用的 scoreWeights。
//
// key 统一为评估器实例 key (entity.EncodeEvaluatorInstanceKey(versionID, alias))，
// 与 record 侧 (EncodeEvaluatorInstanceKey(record.EvaluatorVersionID, record.Alias)) 对齐。
//
//   - MultiSetConfig 新实验: 从带 alias 的权威源 expt.EvalConf.EvalSetConfigs[].EvaluatorConfs
//     (ExptEvaluatorConf, 含 Alias) 聚合取权重，key = EncodeEvaluatorInstanceKey(version, alias)。
//   - SingleSet 老实验: 从 expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf (无 alias) 取，
//     key 退化为裸 versionID，与旧行为 byte 级一致。
//
// 加权开关 (EnableScoreWeight) 复用 ConnectorConf.EvaluatorsConf.EnableScoreWeight:
// 该标志由创建期 (expt_manage_impl.buildExpt) 统一根据 EvaluatorConf.ScoreWeight>0 派生设置，
// 新链路的 ConnectorConf 由 buildExptConfFromEvalSetConfigs 从同一份 eval_set_configs 展开,
// ScoreWeight 透传一致，故新老两链路均以此为准；关闭时返回 nil，calculateWeightedScore 按等权计算。
//
// 取权重的口径与老链路保持一致: ScoreWeight 非空且 >0 才计权重，否则等权。
//
// per-item 说明: 运行期单行执行路径理论上可用 per-item 的 ExptItemRef.ItemConfig.EvaluatorConfs
// (ItemEvaluatorConf, 最精确) 取权重; 但当前 4 处加权计算点
// (RecordItemRunLogs / fillExptTurnResultFilters / RecalculateWeightedScore / EvaluatorRecord 重算)
// 均未在作用域内持有 ItemConfig, 也未注入 IExptItemRefRepo, 故统一走 per-set 聚合口径。
// 权重是配置级而非 item 级, per-set 聚合与 per-item 的 (version,alias)->weight 映射相同,
// 故对加权结果等价; 加权 key (version,alias) 与 record 侧 (record.EvaluatorVersionID, record.Alias) 对齐。
func buildScoreWeights(expt *entity.Experiment) map[string]float64 {
	if expt == nil || expt.EvalConf == nil {
		return nil
	}
	evalsConf := expt.EvalConf.ConnectorConf.EvaluatorsConf
	if evalsConf == nil || !evalsConf.EnableScoreWeight {
		return nil
	}

	// 新链路: 从带 alias 的 per-set 配置聚合
	if expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig && len(expt.EvalConf.EvalSetConfigs) > 0 {
		var scoreWeights map[string]float64
		for _, setConf := range expt.EvalConf.EvalSetConfigs {
			if setConf == nil {
				continue
			}
			for _, ec := range setConf.EvaluatorConfs {
				if ec == nil || ec.ScoreWeight == nil || *ec.ScoreWeight <= 0 || ec.EvaluatorVersionID == 0 {
					continue
				}
				if scoreWeights == nil {
					scoreWeights = make(map[string]float64)
				}
				// 同一 (version,alias) 可能在多个 set 重复出现; 按 (version,alias) 去重,
				// 权重冲突时以后者(非空有效值)为准 (直接覆盖)。
				scoreWeights[entity.EncodeEvaluatorInstanceKey(ec.EvaluatorVersionID, ec.Alias)] = *ec.ScoreWeight
			}
		}
		return scoreWeights
	}

	// 老链路: 从无 alias 的 EvaluatorConf 取, key 退化为裸 versionID
	var scoreWeights map[string]float64
	for _, ec := range evalsConf.EvaluatorConf {
		if ec == nil || ec.ScoreWeight == nil || *ec.ScoreWeight <= 0 || ec.EvaluatorVersionID == 0 {
			continue
		}
		if scoreWeights == nil {
			scoreWeights = make(map[string]float64)
		}
		scoreWeights[entity.EncodeEvaluatorInstanceKey(ec.EvaluatorVersionID, "")] = *ec.ScoreWeight
	}
	return scoreWeights
}

// calculateWeightedScore 计算加权分数
func calculateWeightedScore(
	evaluatorRecords map[string]*entity.EvaluatorRecord,
	weights map[string]float64,
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

	for instanceKey, record := range evaluatorRecords {
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
		weight, ok := weights[instanceKey]
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
