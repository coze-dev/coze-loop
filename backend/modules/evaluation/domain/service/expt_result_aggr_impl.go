// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptAggrResultServiceImpl struct {
	exptTurnResultRepo repo.IExptTurnResultRepo
	exptAggrResultRepo repo.IExptAggrResultRepo
	experimentRepo     repo.IExperimentRepo
	metric             metrics.ExptMetric

	evaluatorService       EvaluatorService
	evaluatorRecordService EvaluatorRecordService
}

func NewExptAggrResultService(
	exptTurnResultRepo repo.IExptTurnResultRepo,
	exptAggrResultRepo repo.IExptAggrResultRepo,
	experimentRepo repo.IExperimentRepo, metric metrics.ExptMetric,
	evaluatorService EvaluatorService,
	evaluatorRecordService EvaluatorRecordService,
) ExptAggrResultService {
	return &ExptAggrResultServiceImpl{
		exptTurnResultRepo:     exptTurnResultRepo,
		exptAggrResultRepo:     exptAggrResultRepo,
		experimentRepo:         experimentRepo,
		metric:                 metric,
		evaluatorService:       evaluatorService,
		evaluatorRecordService: evaluatorRecordService,
	}
}

func (e *ExptAggrResultServiceImpl) CreateExptAggrResult(ctx context.Context, spaceID, experimentID int64) (err error) {
	now := time.Now().Unix()
	defer func() {
		e.metric.EmitCalculateExptAggrResult(spaceID, int64(entity.CreateAllFields), err != nil, now)
	}()

	turnEvaluatorResultRefs, err := e.exptTurnResultRepo.GetTurnEvaluatorResultRefByExptID(ctx, spaceID, experimentID)
	if err != nil {
		return err
	}

	if len(turnEvaluatorResultRefs) == 0 {
		logs.CtxInfo(ctx, "no evaluator result found, skip create expt aggr result")
		return nil
	}

	evaluatorResultIDs := make([]int64, 0)
	evaluatorVersionID2ResultIDs := make(map[int64][]int64)
	for _, turnEvaluatorResultRef := range turnEvaluatorResultRefs {
		evaluatorResultIDs = append(evaluatorResultIDs, turnEvaluatorResultRef.EvaluatorResultID)
		if _, ok := evaluatorVersionID2ResultIDs[turnEvaluatorResultRef.EvaluatorVersionID]; !ok {
			evaluatorVersionID2ResultIDs[turnEvaluatorResultRef.EvaluatorVersionID] = make([]int64, 0)
		}
		evaluatorVersionID2ResultIDs[turnEvaluatorResultRef.EvaluatorVersionID] = append(evaluatorVersionID2ResultIDs[turnEvaluatorResultRef.EvaluatorVersionID], turnEvaluatorResultRef.EvaluatorResultID)
	}

	evaluatorRecords, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, evaluatorResultIDs, false)
	// evalResults, err := e.evalCall.BatchGetEvaluatorRecord(ctx, spaceID, evaluatorResultIDs)
	if err != nil {
		return err
	}
	recordMap := make(map[int64]*entity.EvaluatorRecord)
	for _, record := range evaluatorRecords {
		recordMap[record.ID] = record
	}

	evaluatorVersionID2AggregatorGroup := make(map[int64]*AggregatorGroup)
	for evaluatorVersionID, resultIDs := range evaluatorVersionID2ResultIDs {
		aggregatorGroup := NewAggregatorGroup(WithScoreDistributionAggregator())
		evaluatorVersionID2AggregatorGroup[evaluatorVersionID] = aggregatorGroup
		for _, resultID := range resultIDs {
			evalResult, ok := recordMap[resultID]
			if !ok || evalResult == nil {
				continue
			}
			if evalResult.EvaluatorOutputData == nil ||
				evalResult.EvaluatorOutputData.EvaluatorResult == nil ||
				evalResult.EvaluatorOutputData.EvaluatorResult.Score == nil {
				continue
			}

			aggregatorGroup.Append(gptr.Indirect(evalResult.EvaluatorOutputData.EvaluatorResult.Score))
		}

	}

	return e.createExptAggrResult(ctx, spaceID, experimentID, evaluatorVersionID2AggregatorGroup)
}

func (e *ExptAggrResultServiceImpl) createExptAggrResult(ctx context.Context, spaceID, experimentID int64, evaluatorVersionID2AggregatorGroup map[int64]*AggregatorGroup) error {
	exptAggrResults := make([]*entity.ExptAggrResult, 0)
	for evaluatorVersionID, aggregatorGroup := range evaluatorVersionID2AggregatorGroup {
		aggrResult := aggregatorGroup.Result()
		var averageScore float64
		for _, aggregatorResult := range aggrResult.AggregatorResults {
			if aggregatorResult.AggregatorType == entity.Average {
				averageScore = aggregatorResult.GetScore()
				break
			}
		}
		aggrResultBytes, err := json.Marshal(aggrResult)
		if err != nil {
			return err
		}
		exptAggrResults = append(exptAggrResults, &entity.ExptAggrResult{
			SpaceID:      spaceID,
			ExperimentID: experimentID,
			FieldType:    int32(entity.FieldType_EvaluatorScore),
			FieldKey:     strconv.FormatInt(evaluatorVersionID, 10),
			Score:        averageScore,
			AggrResult:   aggrResultBytes,
			Version:      0,
		})
	}

	err := e.exptAggrResultRepo.BatchCreateExptAggrResult(ctx, exptAggrResults)
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "create expt aggr result success, exptID: %d", experimentID)

	return nil
}

func (e *ExptAggrResultServiceImpl) UpdateExptAggrResult(ctx context.Context, param *entity.UpdateExptAggrResultParam) (err error) {
	now := time.Now().Unix()
	defer func() {
		e.metric.EmitCalculateExptAggrResult(param.SpaceID, int64(entity.UpdateSpecificField), err != nil, now)
	}()

	if param.FieldType != entity.FieldType_EvaluatorScore {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid field type"))
	}
	// 如果首次计算尚未完成 返回error mq重试
	_, err = e.exptAggrResultRepo.GetExptAggrResult(ctx, param.ExperimentID, int32(entity.FieldType_EvaluatorScore), param.FieldKey)
	if err != nil {
		statusErr, ok := errorx.FromStatusError(err)
		if ok && statusErr.Code() == errno.ResourceNotFoundCode {
			experiment, err := e.experimentRepo.GetByID(ctx, param.ExperimentID, param.SpaceID)
			if err != nil {
				return err
			}
			// 如果实验未结束 不进行MQ重试
			if !entity.IsExptFinished(experiment.Status) {
				return nil
			}
		}
		return err
	}

	// 计算前先更新版本号
	version, err := e.exptAggrResultRepo.UpdateAndGetLatestVersion(ctx, param.ExperimentID, int32(param.FieldType), param.FieldKey)
	if err != nil {
		return err
	}

	evaluatorVersionID, err := strconv.ParseInt(param.FieldKey, 10, 64)
	if err != nil {
		return err
	}
	turnEvaluatorResultRefs, err := e.exptTurnResultRepo.GetTurnEvaluatorResultRefByEvaluatorVersionID(ctx, param.SpaceID, param.ExperimentID, evaluatorVersionID)
	if err != nil {
		return err
	}
	evaluatorResultIDs := make([]int64, 0)
	for _, turnEvaluatorResultRef := range turnEvaluatorResultRefs {
		evaluatorResultIDs = append(evaluatorResultIDs, turnEvaluatorResultRef.EvaluatorResultID)
	}

	evaluatorRecords, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, evaluatorResultIDs, false)
	// evalResults, err := e.evalCall.BatchGetEvaluatorRecord(ctx, spaceID, evaluatorResultIDs)
	if err != nil {
		return err
	}
	recordMap := make(map[int64]*entity.EvaluatorRecord)
	for _, record := range evaluatorRecords {
		recordMap[record.ID] = record
	}

	aggregatorGroup := NewAggregatorGroup(WithScoreDistributionAggregator())
	for _, evalResult := range recordMap {
		if evalResult.EvaluatorOutputData == nil || evalResult.EvaluatorOutputData.EvaluatorResult == nil {
			continue
		}
		score := gptr.Indirect(evalResult.EvaluatorOutputData.EvaluatorResult.Score)
		if evalResult.EvaluatorOutputData.EvaluatorResult.Correction != nil {
			score = gptr.Indirect(evalResult.EvaluatorOutputData.EvaluatorResult.Correction.Score)
		}
		aggregatorGroup.Append(score)
	}

	return e.updateExptAggrResult(ctx, param, evaluatorVersionID, aggregatorGroup, version)
}

func (e *ExptAggrResultServiceImpl) updateExptAggrResult(ctx context.Context, param *entity.UpdateExptAggrResultParam, evaluatorVersionID int64, aggregatorGroup *AggregatorGroup, version int64) error {
	aggrResult := aggregatorGroup.Result()
	var averageScore float64
	for _, aggregatorResult := range aggrResult.AggregatorResults {
		if aggregatorResult.AggregatorType == entity.Average {
			averageScore = aggregatorResult.GetScore()
			break
		}
	}
	aggrResultBytes, err := json.Marshal(aggrResult)
	if err != nil {
		return err
	}
	exptAggrResults := &entity.ExptAggrResult{
		SpaceID:      param.SpaceID,
		ExperimentID: param.ExperimentID,
		FieldType:    int32(entity.FieldType_EvaluatorScore),
		FieldKey:     strconv.FormatInt(evaluatorVersionID, 10),
		Score:        averageScore,
		AggrResult:   aggrResultBytes,
	}

	err = e.exptAggrResultRepo.UpdateExptAggrResultByVersion(ctx, exptAggrResults, version)
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "update expt aggr result success, exptID: %d", param.ExperimentID)
	return nil
}

func (e *ExptAggrResultServiceImpl) BatchGetExptAggrResultByExperimentIDs(ctx context.Context, spaceID int64, exptIDs []int64) ([]*entity.ExptAggregateResult, error) {
	aggrResults, err := e.exptAggrResultRepo.BatchGetExptAggrResultByExperimentIDs(ctx, exptIDs)
	if err != nil {
		return nil, err
	}

	// split aggrResults by experimentID
	expt2AggrResults := make(map[int64][]*entity.ExptAggrResult)
	for _, aggrResult := range aggrResults {
		if _, ok := expt2AggrResults[aggrResult.ExperimentID]; !ok {
			expt2AggrResults[aggrResult.ExperimentID] = make([]*entity.ExptAggrResult, 0)
		}
		expt2AggrResults[aggrResult.ExperimentID] = append(expt2AggrResults[aggrResult.ExperimentID], aggrResult)
	}

	evaluatorRef, err := e.experimentRepo.GetEvaluatorRefByExptIDs(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, err
	}
	// 去重
	evaluatorVersionIDMap := make(map[int64]bool)
	versionID2evaluatorID := make(map[int64]int64)
	for _, ref := range evaluatorRef {
		evaluatorVersionIDMap[ref.EvaluatorVersionID] = true
		versionID2evaluatorID[ref.EvaluatorVersionID] = ref.EvaluatorID
	}

	evaluatorVersionIDs := maps.ToSlice(evaluatorVersionIDMap, func(k int64, v bool) int64 {
		return k
	})
	evaluatorVersionList, err := e.evaluatorService.BatchGetEvaluatorVersion(ctx, nil, evaluatorVersionIDs, true)
	// evaluators, err := e.evalCall.BatchGetEvaluatorVersion(ctx, spaceID, evaluatorVersionIDs, true)
	if err != nil {
		return nil, err
	}

	versionID2Evaluator := make(map[int64]*entity.Evaluator)
	for _, evaluator := range evaluatorVersionList {
		evaluatorVersion := evaluator.GetEvaluatorVersion()
		if evaluatorVersion == nil || !gslice.Contains(evaluatorVersionIDs, evaluatorVersion.GetID()) {
			continue
		}

		versionID2Evaluator[evaluatorVersion.GetID()] = evaluator
	}

	results := make([]*entity.ExptAggregateResult, 0, len(expt2AggrResults))
	for exptID, exptResult := range expt2AggrResults {
		evaluatorResults := make(map[int64]*entity.EvaluatorAggregateResult)

		for _, fieldResult := range exptResult {
			if fieldResult.FieldType != int32(entity.FieldType_EvaluatorScore) {
				continue
			}

			evaluatorVersionID, err := strconv.ParseInt(fieldResult.FieldKey, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse evaluator version id from field key %s, err: %v", fieldResult.FieldKey, err)
			}

			aggregateResultDO := entity.AggregateResult{}
			err = json.Unmarshal(fieldResult.AggrResult, &aggregateResultDO)
			if err != nil {
				return nil, fmt.Errorf("json.Unmarshal(%s) failed, err: %v", fieldResult.AggrResult, err)
			}

			evaluator, ok := versionID2Evaluator[evaluatorVersionID]
			if !ok {
				return nil, fmt.Errorf("failed to get evaluator by version_id %d", evaluatorVersionID)
			}

			evaluatorVersion := evaluator.PromptEvaluatorVersion
			if evaluatorVersion == nil {
				return nil, fmt.Errorf("failed to get evaluator version by version_id %d", evaluatorVersionID)
			}

			evaluatorAggrResult := entity.EvaluatorAggregateResult{
				EvaluatorVersionID: evaluatorVersionID,
				AggregatorResults:  aggregateResultDO.AggregatorResults,
				Name:               gptr.Of(evaluator.Name),
				Version:            gptr.Of(evaluatorVersion.Version),
			}
			evaluatorResults[evaluatorVersionID] = &evaluatorAggrResult

		}
		results = append(results, &entity.ExptAggregateResult{
			ExperimentID:     exptID,
			EvaluatorResults: evaluatorResults,
		})
	}

	return results, nil
}

type AggregatorGroup struct {
	Aggregators         []Aggregator
	AggregatorResultMap map[entity.AggregatorType]*entity.AggregateData
}

type NewAggregatorGroupOption func(aggregatorGroup *AggregatorGroup)

func NewAggregatorGroup(opts ...NewAggregatorGroupOption) *AggregatorGroup {
	m := &AggregatorGroup{
		Aggregators: []Aggregator{},
	}

	m.Aggregators = append(m.Aggregators, &BasicAggregator{})

	// optional aggregators
	for _, opt := range opts {
		opt(m)
	}

	return m
}

func WithScoreDistributionAggregator() NewAggregatorGroupOption {
	return func(aggregatorGroup *AggregatorGroup) {
		aggregatorGroup.Aggregators = append(aggregatorGroup.Aggregators, &ScoreDistributionAggregator{})
	}
}

func (a *AggregatorGroup) Append(score float64) {
	for _, aggregator := range a.Aggregators {
		aggregator.Append(score)
	}
}

func (a *AggregatorGroup) Result() *entity.AggregateResult {
	aggregatorResults := make([]*entity.AggregatorResult, 0)
	for _, aggregator := range a.Aggregators {
		for aggregatorType, result := range aggregator.Result() {
			aggregatorResult := entity.AggregatorResult{
				AggregatorType: aggregatorType,
				Data:           result,
			}
			aggregatorResults = append(aggregatorResults, &aggregatorResult)
		}
	}

	return &entity.AggregateResult{
		AggregatorResults: aggregatorResults,
	}
}

type Aggregator interface {
	Append(score float64)
	Result() map[entity.AggregatorType]*entity.AggregateData
}

type BasicAggregator struct {
	Max float64
	Min float64
	Sum float64

	Count int // 聚合数据个数
}

func (a *BasicAggregator) Append(score float64) {
	a.Count++

	if a.Count == 1 {
		a.Min = score
		a.Max = score
		a.Sum = score
		return
	}

	if score < a.Min {
		a.Min = score
	}

	if score > a.Max {
		a.Max = score
	}

	a.Sum += score
}

func (a *BasicAggregator) Result() map[entity.AggregatorType]*entity.AggregateData {
	res := make(map[entity.AggregatorType]*entity.AggregateData, 4)

	avg := 0.0
	if a.Count != 0 {
		avg = a.Sum / float64(a.Count)
	}
	res[entity.Average] = &entity.AggregateData{
		Value:    &avg,
		DataType: entity.Double,
	}
	res[entity.Sum] = &entity.AggregateData{
		Value:    &a.Sum,
		DataType: entity.Double,
	}
	res[entity.Max] = &entity.AggregateData{
		Value:    &a.Max,
		DataType: entity.Double,
	}
	res[entity.Min] = &entity.AggregateData{
		Value:    &a.Min,
		DataType: entity.Double,
	}

	return res
}

// ScoreDistributionAggregator 分布聚合器.
type ScoreDistributionAggregator struct {
	Score2Count map[float64]int64
	Total       int64
}

func (a *ScoreDistributionAggregator) Append(score float64) {
	if a.Score2Count == nil {
		a.Score2Count = make(map[float64]int64)
	}
	count, ok := a.Score2Count[score]
	if !ok {
		a.Score2Count[score] = 1
	} else {
		a.Score2Count[score] = count + 1
	}

	a.Total++
}

func (a *ScoreDistributionAggregator) Result() map[entity.AggregatorType]*entity.AggregateData {
	const topN = 5
	scoreCounts := GetTopNScores(a.Score2Count, topN)
	data := &entity.AggregateData{
		DataType: entity.ScoreDistribution,
		ScoreDistribution: &entity.ScoreDistributionData{
			ScoreDistributionItems: make([]*entity.ScoreDistributionItem, len(scoreCounts)),
		},
	}

	for _, scoreCount := range scoreCounts {
		scoreDistributionItem := &entity.ScoreDistributionItem{
			Score:      scoreCount.Score,
			Count:      scoreCount.Count,
			Percentage: float64(scoreCount.Count) / float64(a.Total),
		}
		data.ScoreDistribution.ScoreDistributionItems = append(data.ScoreDistribution.ScoreDistributionItems, scoreDistributionItem)
	}

	return map[entity.AggregatorType]*entity.AggregateData{
		entity.Distribution: data,
	}
}

type ScoreCount struct {
	Score string
	Count int64
}

// GetTopNScores 获取出现次数最高的前 N 个分数
func GetTopNScores(score2Count map[float64]int64, n int) []ScoreCount {
	scoreCounts := make([]ScoreCount, 0, len(score2Count))
	for score, count := range score2Count {
		scoreCounts = append(scoreCounts, ScoreCount{Score: strconv.FormatFloat(score, 'f', 2, 64), Count: count})
	}

	// 按照 Count 降序排序
	sort.Slice(scoreCounts, func(i, j int) bool {
		return scoreCounts[i].Count > scoreCounts[j].Count
	})

	// 取出前 N 个（如果不足 N 个则返回全部）
	if len(scoreCounts) > n {
		aggregatedCount := int64(0)
		for i := 5; i < len(scoreCounts); i++ {
			aggregatedCount += scoreCounts[i].Count
		}
		scoreCounts = append(scoreCounts[:n], ScoreCount{Score: "其他", Count: aggregatedCount})
	}
	return scoreCounts
}
