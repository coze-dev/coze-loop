// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptAggrResultServiceImpl struct {
	exptTurnResultRepo repo.IExptTurnResultRepo
	exptAggrResultRepo repo.IExptAggrResultRepo
	experimentRepo     repo.IExperimentRepo
	metric             metrics.ExptMetric
	exptAnnotateRepo   repo.IExptAnnotateRepo

	evaluatorService       EvaluatorService
	evaluatorRecordService EvaluatorRecordService
	tagRPCAdapter          rpc.ITagRPCAdapter
	evalTargetSvc          IEvalTargetService

	publisher events.ExptEventPublisher
	locker    lock.ILocker
}

func NewExptAggrResultService(
	exptTurnResultRepo repo.IExptTurnResultRepo,
	exptAggrResultRepo repo.IExptAggrResultRepo,
	experimentRepo repo.IExperimentRepo, metric metrics.ExptMetric,
	evaluatorService EvaluatorService,
	evaluatorRecordService EvaluatorRecordService,
	tagRPCAdapter rpc.ITagRPCAdapter,
	exptAnnotateRepo repo.IExptAnnotateRepo,
	ets IEvalTargetService,
	pub events.ExptEventPublisher,
	locker lock.ILocker,
) ExptAggrResultService {
	return &ExptAggrResultServiceImpl{
		exptTurnResultRepo:     exptTurnResultRepo,
		exptAggrResultRepo:     exptAggrResultRepo,
		experimentRepo:         experimentRepo,
		metric:                 metric,
		evaluatorService:       evaluatorService,
		evaluatorRecordService: evaluatorRecordService,
		tagRPCAdapter:          tagRPCAdapter,
		exptAnnotateRepo:       exptAnnotateRepo,
		evalTargetSvc:          ets,
		publisher:              pub,
		locker:                 locker,
	}
}

func (e *ExptAggrResultServiceImpl) CreateExptAggrResult(ctx context.Context, spaceID, experimentID int64) (err error) {
	now := time.Now().Unix()
	defer func() { e.metric.EmitCalculateExptAggrResult(spaceID, int64(entity.CreateAllFields), err != nil, now) }()
	defer func() {
		if err == nil {
			if _, unlockErr := e.locker.Unlock(e.MakeCalcExptAggrResultLockKey(experimentID)); unlockErr != nil {
				logs.CtxWarn(ctx, "CreateExptAggrResult unlock fail, expt_id: %v, err: %v", experimentID, unlockErr)
			}
		}
	}()

	existed, err := e.exptAggrResultRepo.GetExptAggrResultByExperimentID(ctx, experimentID)
	if err != nil {
		return err
	}

	turnEvaluatorResultRefs, err := e.exptTurnResultRepo.GetTurnEvaluatorResultRefByExptID(ctx, spaceID, experimentID)
	if err != nil {
		return err
	}

	evaluatorVersionID2AggregatorGroup := make(map[int64]*AggregatorGroup)
	if len(turnEvaluatorResultRefs) > 0 {
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
		if err != nil {
			return err
		}
		recordMap := make(map[int64]*entity.EvaluatorRecord)
		for _, record := range evaluatorRecords {
			recordMap[record.ID] = record
		}

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
	}

	tmag, err := e.buildExptTargetMtrAggregatorGroup(ctx, spaceID, experimentID)
	if err != nil {
		return err
	}

	return e.CreateOrUpdateExptAggrResult(ctx, spaceID, experimentID, evaluatorVersionID2AggregatorGroup, tmag, existed)
}

func (e *ExptAggrResultServiceImpl) buildExptTargetMtrAggregatorGroup(ctx context.Context, spaceID, exptID int64) (*targetMtrAggrGroup, error) {
	const queryInterval = time.Millisecond * 30

	mtrAggrGroup := &targetMtrAggrGroup{
		latency:      NewAggregatorGroup(WithBucketScoreDistributionAggregator(20)),
		inputTokens:  NewAggregatorGroup(WithBucketScoreDistributionAggregator(20)),
		outputTokens: NewAggregatorGroup(WithBucketScoreDistributionAggregator(20)),
		totalTokens:  NewAggregatorGroup(WithBucketScoreDistributionAggregator(20)),
	}

	var targetResultIDs []int64
	maxLoop, cursor, limit := 10000, int64(0), int64(50)

	for i := 0; i < maxLoop; i++ {
		logs.CtxInfo(ctx, "buildExptTargetMtrAggregatorGroup scan item result, expt_id: %v, cursor: %v, limit: %v", exptID, cursor, limit)

		trs, ncursor, err := e.exptTurnResultRepo.ScanTurnResults(ctx, exptID, nil, cursor, limit, spaceID)
		if err != nil {
			return nil, err
		}

		cursor = ncursor

		if len(trs) == 0 {
			break
		}

		for _, tr := range trs {
			targetResultIDs = append(targetResultIDs, tr.TargetResultID)
		}

		time.Sleep(queryInterval)
	}

	for _, resIDs := range gslice.Chunk(targetResultIDs, 50) {
		records, err := e.evalTargetSvc.BatchGetRecordByIDs(ctx, spaceID, resIDs)
		if err != nil {
			return nil, err
		}

		mtrAggrGroup.calcRecord(records)
	}

	return mtrAggrGroup, nil
}

func (e *ExptAggrResultServiceImpl) CreateOrUpdateExptAggrResult(ctx context.Context, spaceID, experimentID int64,
	evaluatorVersionID2AggregatorGroup map[int64]*AggregatorGroup, tmag *targetMtrAggrGroup, existedAggrResults []*entity.ExptAggrResult,
) error {
	aggrResKeyFn := func(fieldType int32, fieldKey string) string { return fmt.Sprintf("%d:%s", fieldType, fieldKey) }
	existedAggrResultsMap := gslice.ToMap(existedAggrResults, func(val *entity.ExptAggrResult) (string, *entity.ExptAggrResult) {
		return aggrResKeyFn(val.FieldType, val.FieldKey), val
	})

	aggrResults := make([]*entity.ExptAggrResult, 0)
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
		aggrResults = append(aggrResults, &entity.ExptAggrResult{
			SpaceID:      spaceID,
			ExperimentID: experimentID,
			FieldType:    int32(entity.FieldType_EvaluatorScore),
			FieldKey:     strconv.FormatInt(evaluatorVersionID, 10),
			Score:        averageScore,
			AggrResult:   aggrResultBytes,
			Version:      0,
		})
	}

	targetAggrResults, err := tmag.buildAggrResult(spaceID, experimentID)
	if err != nil {
		return err
	}

	aggrResults = append(aggrResults, targetAggrResults...)

	var tocreated []*entity.ExptAggrResult
	var toupdated []*entity.ExptAggrResult
	for _, ar := range aggrResults {
		if existed, ok := existedAggrResultsMap[aggrResKeyFn(ar.FieldType, ar.FieldKey)]; ok {
			if existed.AggrResEqual(ar) {
				continue
			}
			version, err := e.exptAggrResultRepo.UpdateAndGetLatestVersion(ctx, experimentID, ar.FieldType, ar.FieldKey)
			if err != nil {
				return errorx.Wrapf(err, "UpdateAndGetLatestVersion failed, expt_id: %d, field_type: %d, field_key: %s", experimentID, ar.FieldType, ar.FieldKey)
			}
			ar.Version = version
			toupdated = append(toupdated, ar)
		} else {
			tocreated = append(tocreated, ar)
		}
	}

	if len(tocreated) > 0 {
		if err := e.exptAggrResultRepo.BatchCreateExptAggrResult(ctx, tocreated); err != nil {
			return errorx.Wrapf(err, "BatchCreateExptAggrResult failed, expt_id: %d", experimentID)
		}
		logs.CtxInfo(ctx, "create expt aggr result success, expt_id: %d, created: %v", experimentID, json.Jsonify(tocreated))
	}

	if len(toupdated) > 0 {
		for _, ar := range toupdated {
			if err := e.exptAggrResultRepo.UpdateExptAggrResultByVersion(ctx, ar, ar.Version); err != nil {
				return errorx.Wrapf(err, "UpdateExptAggrResultByVersion failed, experimentID: %d, fieldType: %d, fieldKey: %s, version: %d", experimentID, ar.FieldType, ar.FieldKey, ar.Version)
			}
		}
		logs.CtxInfo(ctx, "update expt aggr result success, exptID: %d, updated: %v", experimentID, json.Jsonify(toupdated))
	}

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
	// If initial calculation not finished, return error for MQ retry
	_, err = e.exptAggrResultRepo.GetExptAggrResult(ctx, param.ExperimentID, int32(entity.FieldType_EvaluatorScore), param.FieldKey)
	if err != nil {
		statusErr, ok := errorx.FromStatusError(err)
		if ok && statusErr.Code() == errno.ResourceNotFoundCode {
			experiment, err := e.experimentRepo.GetByID(ctx, param.ExperimentID, param.SpaceID)
			if err != nil {
				return err
			}
			// If experiment not finished, no MQ retry
			if !entity.IsExptFinished(experiment.Status) {
				return nil
			}
		}
		return err
	}

	// Update version number before calculation
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
	expts, err := e.experimentRepo.MGetBasicByID(ctx, exptIDs)
	if err != nil {
		return nil, err
	}

	versionedTargetIDMap := gslice.ToMap(expts, func(t *entity.Experiment) (int64, entity.VersionedTargetID) {
		return t.ID, entity.VersionedTargetID{
			TargetID:  t.TargetID,
			VersionID: t.TargetVersionID,
		}
	})

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
	// Deduplicate
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
	if err != nil {
		return nil, err
	}

	tagInfoMap, err := e.batchGetTagInfoByExperimentIDs(ctx, spaceID, exptIDs)
	if err != nil {
		return nil, err
	}

	versionID2Evaluator := make(map[int64]*entity.Evaluator)
	for _, evaluator := range evaluatorVersionList {
		if (evaluator.EvaluatorType == entity.EvaluatorTypePrompt && evaluator.PromptEvaluatorVersion == nil) || (evaluator.EvaluatorType == entity.EvaluatorTypeCode && evaluator.CodeEvaluatorVersion == nil) || !gslice.Contains(evaluatorVersionIDs, evaluator.GetEvaluatorVersionID()) {
			continue
		}

		versionID2Evaluator[evaluator.GetEvaluatorVersionID()] = evaluator
	}

	results := make([]*entity.ExptAggregateResult, 0, len(expt2AggrResults))
	for exptID, exptResult := range expt2AggrResults {
		evaluatorResults := make(map[int64]*entity.EvaluatorAggregateResult)
		annotationResults := make(map[int64]*entity.AnnotationAggregateResult)
		targetResults := &entity.EvalTargetMtrAggrResult{
			TargetID:        versionedTargetIDMap[exptID].TargetID,
			TargetVersionID: versionedTargetIDMap[exptID].VersionID,
		}

		var latestUpdateAt *time.Time
		for _, fieldResult := range exptResult {
			if gptr.Indirect(fieldResult.UpdateAt).After(gptr.Indirect(latestUpdateAt)) {
				latestUpdateAt = fieldResult.UpdateAt
			}

			switch fieldResult.FieldType {
			case int32(entity.FieldType_Annotation):
				tagKeyID, err := strconv.ParseInt(fieldResult.FieldKey, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse tag key id from field key %s, err: %v", fieldResult.FieldKey, err)
				}
				aggregateResultDO := entity.AggregateResult{}
				err = json.Unmarshal(fieldResult.AggrResult, &aggregateResultDO)
				if err != nil {
					return nil, fmt.Errorf("json.Unmarshal(%s) failed, err: %v", fieldResult.AggrResult, err)
				}

				tagInfo, ok := tagInfoMap[tagKeyID]
				if !ok {
					return nil, fmt.Errorf("failed to get tag info by tag key id %d", tagKeyID)
				}
				annotationResult := &entity.AnnotationAggregateResult{
					TagKeyID:          tagKeyID,
					AggregatorResults: aggregateResultDO.AggregatorResults,
					Name:              ptr.Of(tagInfo.TagKeyName),
				}
				annotationResults[tagKeyID] = annotationResult
			case int32(entity.FieldType_EvaluatorScore):
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

				evaluatorAggrResult := entity.EvaluatorAggregateResult{
					EvaluatorID:        evaluator.ID,
					EvaluatorVersionID: evaluatorVersionID,
					AggregatorResults:  aggregateResultDO.AggregatorResults,
					Name:               gptr.Of(evaluator.Name),
					Version:            gptr.Of(evaluator.GetVersion()),
				}
				evaluatorResults[evaluatorVersionID] = &evaluatorAggrResult
			case int32(entity.FieldType_TargetLatency):
				ar := entity.AggregateResult{}
				if err := json.Unmarshal(fieldResult.AggrResult, &ar); err != nil {
					return nil, errorx.Wrapf(err, "AggregateResult json.Unmarshal failed, raw: %v", string(fieldResult.AggrResult))
				}
				targetResults.LatencyAggrResults = gslice.Clone(ar.AggregatorResults)

			case int32(entity.FieldType_TargetInputTokens):
				ar := entity.AggregateResult{}
				if err := json.Unmarshal(fieldResult.AggrResult, &ar); err != nil {
					return nil, errorx.Wrapf(err, "AggregateResult json.Unmarshal failed, raw: %v", string(fieldResult.AggrResult))
				}
				targetResults.InputTokensAggrResults = gslice.Clone(ar.AggregatorResults)

			case int32(entity.FieldType_TargetOutputTokens):
				ar := entity.AggregateResult{}
				if err := json.Unmarshal(fieldResult.AggrResult, &ar); err != nil {
					return nil, errorx.Wrapf(err, "AggregateResult json.Unmarshal failed, raw: %v", string(fieldResult.AggrResult))
				}
				targetResults.OutputTokensAggrResults = gslice.Clone(ar.AggregatorResults)

			case int32(entity.FieldType_TargetTotalTokens):
				ar := entity.AggregateResult{}
				if err := json.Unmarshal(fieldResult.AggrResult, &ar); err != nil {
					return nil, errorx.Wrapf(err, "AggregateResult json.Unmarshal failed, raw: %v", string(fieldResult.AggrResult))
				}
				targetResults.TotalTokensAggrResults = gslice.Clone(ar.AggregatorResults)

			default:

			}
		}

		results = append(results, &entity.ExptAggregateResult{
			ExperimentID:      exptID,
			EvaluatorResults:  evaluatorResults,
			AnnotationResults: annotationResults,
			TargetResults:     targetResults,
			UpdateTime:        latestUpdateAt,
		})
	}

	return results, nil
}

func (e *ExptAggrResultServiceImpl) batchGetTagInfoByExperimentIDs(ctx context.Context, spaceID int64, exptIDs []int64) (map[int64]*entity.TagInfo, error) {
	refs, err := e.exptAnnotateRepo.BatchGetExptTurnAnnotateRecordRefs(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, err
	}

	tagKeyIDMap := make(map[int64]bool)
	for _, ref := range refs {
		tagKeyIDMap[ref.TagKeyID] = true
	}
	tagKeyIDs := maps.ToSlice(tagKeyIDMap, func(k int64, v bool) int64 {
		return k
	})

	tagInfos, err := e.tagRPCAdapter.BatchGetTagInfo(ctx, spaceID, tagKeyIDs)
	if err != nil {
		return nil, err
	}
	return tagInfos, nil
}

func (e *ExptAggrResultServiceImpl) CreateAnnotationAggrResult(ctx context.Context, param *entity.CreateSpecificFieldAggrResultParam) (err error) {
	now := time.Now().Unix()
	defer func() {
		e.metric.EmitCalculateExptAggrResult(param.SpaceID, int64(entity.CreateAnnotationFields), err != nil, now)
	}()

	if param.FieldType != entity.FieldType_Annotation {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid field type"))
	}

	tagKeyID, err := strconv.ParseInt(param.FieldKey, 10, 64)
	if err != nil {
		return err
	}

	annotateRecordRefs, err := e.exptAnnotateRepo.GetExptTurnAnnotateRecordRefsByTagKeyID(ctx, param.ExperimentID, param.SpaceID, tagKeyID)
	if err != nil {
		return err
	}

	if len(annotateRecordRefs) == 0 {
		logs.CtxInfo(ctx, "no evaluator result found, skip create expt aggr result")
		return nil
	}

	recordIDs := make([]int64, 0)
	for _, ref := range annotateRecordRefs {
		recordIDs = append(recordIDs, ref.AnnotateRecordID)
	}
	annotateRecords, err := e.exptAnnotateRepo.GetAnnotateRecordsByIDs(ctx, param.SpaceID, recordIDs)
	if err != nil {
		return err
	}

	if len(annotateRecords) == 0 {
		logs.CtxInfo(ctx, "no annotate record found, skip create expt aggr result")
		return nil
	}
	tagContentType := annotateRecords[0].AnnotateData.TagContentType

	switch tagContentType {
	case entity.TagContentTypeContinuousNumber:
		return e.createContinuousNumberExptAggrResult(ctx, param, annotateRecords)
	case entity.TagContentTypeBoolean:
		return e.createBooleanExptAggrResult(ctx, param, annotateRecords)
	case entity.TagContentTypeCategorical:
		return e.createCategoricalExptAggrResult(ctx, param, annotateRecords)
	case entity.TagContentTypeFreeText:
		return nil
	default:
		return nil
	}
}

func (e *ExptAggrResultServiceImpl) createCategoricalExptAggrResult(ctx context.Context, param *entity.CreateSpecificFieldAggrResultParam, annotateRecords []*entity.AnnotateRecord) error {
	categoricalAggregatorGroup := NewCategoricalAggregatorGroup()
	for _, annotateRecord := range annotateRecords {
		categoricalAggregatorGroup.Append(strconv.FormatInt(annotateRecord.TagValueID, 10))
	}
	aggrResult := categoricalAggregatorGroup.Result()

	aggrResultBytes, err := json.Marshal(aggrResult)
	if err != nil {
		return err
	}
	exptAggrResult := &entity.ExptAggrResult{
		SpaceID:      param.SpaceID,
		ExperimentID: param.ExperimentID,
		FieldType:    int32(entity.FieldType_Annotation),
		FieldKey:     param.FieldKey,
		AggrResult:   aggrResultBytes,
		Version:      0,
	}

	err = e.exptAggrResultRepo.CreateExptAggrResult(ctx, exptAggrResult)
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "CreateCategoricalExptAggrResult success, exptID: %d", param.ExperimentID)
	return nil
}

func (e *ExptAggrResultServiceImpl) createContinuousNumberExptAggrResult(ctx context.Context, param *entity.CreateSpecificFieldAggrResultParam, annotateRecords []*entity.AnnotateRecord) error {
	aggregatorGroup := NewAggregatorGroup(WithScoreDistributionAggregator())
	for _, annotateRecord := range annotateRecords {
		if annotateRecord.AnnotateData.Score == nil {
			continue
		}
		aggregatorGroup.Append(gptr.Indirect(annotateRecord.AnnotateData.Score))
	}
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
	exptAggrResult := &entity.ExptAggrResult{
		SpaceID:      param.SpaceID,
		ExperimentID: param.ExperimentID,
		FieldType:    int32(entity.FieldType_Annotation),
		FieldKey:     param.FieldKey,
		AggrResult:   aggrResultBytes,
		Version:      0,
		Score:        averageScore,
	}

	err = e.exptAggrResultRepo.CreateExptAggrResult(ctx, exptAggrResult)
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "CreateContinuousNumberExptAggrResult success, exptID: %d", param.ExperimentID)
	return nil
}

func (e *ExptAggrResultServiceImpl) createBooleanExptAggrResult(ctx context.Context, param *entity.CreateSpecificFieldAggrResultParam, annotateRecords []*entity.AnnotateRecord) error {
	booleanAggregatorGroup := NewCategoricalAggregatorGroup()
	for _, annotateRecord := range annotateRecords {
		booleanAggregatorGroup.Append(strconv.FormatInt(annotateRecord.TagValueID, 10))
	}
	aggrResult := booleanAggregatorGroup.Result()

	aggrResultBytes, err := json.Marshal(aggrResult)
	if err != nil {
		return err
	}
	exptAggrResult := &entity.ExptAggrResult{
		SpaceID:      param.SpaceID,
		ExperimentID: param.ExperimentID,
		FieldType:    int32(entity.FieldType_Annotation),
		FieldKey:     param.FieldKey,
		AggrResult:   aggrResultBytes,
		Version:      0,
	}

	err = e.exptAggrResultRepo.CreateExptAggrResult(ctx, exptAggrResult)
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "CreateBooleanExptAggrResult success, exptID: %d", param.ExperimentID)
	return nil
}

func (e *ExptAggrResultServiceImpl) UpdateAnnotationAggrResult(ctx context.Context, param *entity.UpdateExptAggrResultParam) (err error) {
	now := time.Now().Unix()
	defer func() {
		e.metric.EmitCalculateExptAggrResult(param.SpaceID, int64(entity.UpdateSpecificField), err != nil, now)
	}()

	if param.FieldType != entity.FieldType_Annotation {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid field type"))
	}
	// If the initial calculation is not yet completed, return an error for MQ retry
	_, err = e.exptAggrResultRepo.GetExptAggrResult(ctx, param.ExperimentID, int32(entity.FieldType_Annotation), param.FieldKey)
	if err != nil {
		statusErr, ok := errorx.FromStatusError(err)
		if ok && statusErr.Code() == errno.ResourceNotFoundCode {
			experiment, err := e.experimentRepo.GetByID(ctx, param.ExperimentID, param.SpaceID)
			if err != nil {
				return err
			}
			// If experiment not finished, no MQ retry
			if !entity.IsExptFinished(experiment.Status) {
				return nil
			}
		}
		return err
	}

	// Update version number before calculation
	version, err := e.exptAggrResultRepo.UpdateAndGetLatestVersion(ctx, param.ExperimentID, int32(param.FieldType), param.FieldKey)
	if err != nil {
		return err
	}

	tagKeyID, err := strconv.ParseInt(param.FieldKey, 10, 64)
	if err != nil {
		return err
	}

	annotateRecordRefs, err := e.exptAnnotateRepo.GetExptTurnAnnotateRecordRefsByTagKeyID(ctx, param.ExperimentID, param.SpaceID, tagKeyID)
	if err != nil {
		return err
	}

	if len(annotateRecordRefs) == 0 {
		logs.CtxInfo(ctx, "no evaluator result found, skip create expt aggr result")
		return nil
	}

	recordIDs := make([]int64, 0)
	for _, ref := range annotateRecordRefs {
		recordIDs = append(recordIDs, ref.AnnotateRecordID)
	}
	annotateRecords, err := e.exptAnnotateRepo.GetAnnotateRecordsByIDs(ctx, param.SpaceID, recordIDs)
	if err != nil {
		return err
	}

	if len(annotateRecords) == 0 {
		logs.CtxInfo(ctx, "no annotate record found, skip create expt aggr result")
		return nil
	}
	tagContentType := annotateRecords[0].AnnotateData.TagContentType

	switch tagContentType {
	case entity.TagContentTypeContinuousNumber:
		return e.updateContinuousNumberExptAggrResult(ctx, param, annotateRecords, version)
	case entity.TagContentTypeBoolean:
		return e.updateBooleanExptAggrResult(ctx, param, annotateRecords, version)
	case entity.TagContentTypeCategorical:
		return e.updateCategoricalExptAggrResult(ctx, param, annotateRecords, version)
	case entity.TagContentTypeFreeText:
		return nil
	default:
		return nil
	}
}

func (e *ExptAggrResultServiceImpl) updateContinuousNumberExptAggrResult(ctx context.Context, param *entity.UpdateExptAggrResultParam, annotateRecords []*entity.AnnotateRecord, version int64) error {
	aggregatorGroup := NewAggregatorGroup(WithScoreDistributionAggregator())
	for _, annotateRecord := range annotateRecords {
		if annotateRecord.AnnotateData.Score == nil {
			continue
		}
		aggregatorGroup.Append(gptr.Indirect(annotateRecord.AnnotateData.Score))
	}
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
		FieldType:    int32(entity.FieldType_Annotation),
		FieldKey:     param.FieldKey,
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

func (e *ExptAggrResultServiceImpl) updateCategoricalExptAggrResult(ctx context.Context, param *entity.UpdateExptAggrResultParam, annotateRecords []*entity.AnnotateRecord, version int64) error {
	categoricalAggregatorGroup := NewCategoricalAggregatorGroup()
	for _, annotateRecord := range annotateRecords {
		categoricalAggregatorGroup.Append(strconv.FormatInt(annotateRecord.TagValueID, 10))
	}
	aggrResult := categoricalAggregatorGroup.Result()

	aggrResultBytes, err := json.Marshal(aggrResult)
	if err != nil {
		return err
	}
	exptAggrResult := &entity.ExptAggrResult{
		SpaceID:      param.SpaceID,
		ExperimentID: param.ExperimentID,
		FieldType:    int32(entity.FieldType_Annotation),
		FieldKey:     param.FieldKey,
		AggrResult:   aggrResultBytes,
	}

	err = e.exptAggrResultRepo.UpdateExptAggrResultByVersion(ctx, exptAggrResult, version)
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "update expt aggr result success, exptID: %d", param.ExperimentID)
	return nil
}

func (e *ExptAggrResultServiceImpl) updateBooleanExptAggrResult(ctx context.Context, param *entity.UpdateExptAggrResultParam, annotateRecords []*entity.AnnotateRecord, version int64) error {
	booleanAggregatorGroup := NewCategoricalAggregatorGroup()
	for _, annotateRecord := range annotateRecords {
		booleanAggregatorGroup.Append(strconv.FormatInt(annotateRecord.TagValueID, 10))
	}
	aggrResult := booleanAggregatorGroup.Result()

	aggrResultBytes, err := json.Marshal(aggrResult)
	if err != nil {
		return err
	}

	exptAggrResults := &entity.ExptAggrResult{
		SpaceID:      param.SpaceID,
		ExperimentID: param.ExperimentID,
		FieldType:    int32(entity.FieldType_Annotation),
		FieldKey:     param.FieldKey,
		AggrResult:   aggrResultBytes,
	}

	err = e.exptAggrResultRepo.UpdateExptAggrResultByVersion(ctx, exptAggrResults, version)
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "update expt aggr result success, exptID: %d", param.ExperimentID)
	return nil
}

type targetMtrAggrGroup struct {
	latency      *AggregatorGroup
	inputTokens  *AggregatorGroup
	outputTokens *AggregatorGroup
	totalTokens  *AggregatorGroup
}

func (t *targetMtrAggrGroup) calcRecord(records []*entity.EvalTargetRecord) {
	for _, record := range records {
		if record == nil || record.EvalTargetOutputData == nil {
			continue
		}

		t.latency.Append(float64(gptr.Indirect(record.EvalTargetOutputData.TimeConsumingMS)))
		if record.EvalTargetOutputData.EvalTargetUsage != nil {
			t.inputTokens.Append(float64(record.EvalTargetOutputData.EvalTargetUsage.InputTokens))
			t.outputTokens.Append(float64(record.EvalTargetOutputData.EvalTargetUsage.OutputTokens))
			t.totalTokens.Append(float64(record.EvalTargetOutputData.EvalTargetUsage.TotalTokens))
		}
	}
}

func (t *targetMtrAggrGroup) buildAggrResult(spaceID, exptID int64) ([]*entity.ExptAggrResult, error) {
	var res []*entity.ExptAggrResult

	builder := func(fieldType entity.FieldType, fieldKey string, aggr *AggregatorGroup) error {
		if aggr == nil {
			return nil
		}

		agRes := aggr.Result()

		var averageScore float64
		for _, aggregatorResult := range agRes.AggregatorResults {
			if aggregatorResult.AggregatorType == entity.Average {
				averageScore = aggregatorResult.GetScore()
				break
			}
		}

		aggrResultBytes, err := json.Marshal(agRes)
		if err != nil {
			return errorx.Wrapf(err, "AggregateResult json marshal fail")
		}

		res = append(res, &entity.ExptAggrResult{
			SpaceID:      spaceID,
			ExperimentID: exptID,
			FieldType:    int32(fieldType),
			FieldKey:     fieldKey,
			Score:        averageScore,
			AggrResult:   aggrResultBytes,
		})
		return nil
	}

	for _, cfg := range []struct {
		fieldType entity.FieldType
		fieldKey  string
		aggr      *AggregatorGroup
	}{
		{entity.FieldType_TargetLatency, entity.AggrResultFieldKey_TargetLatency, t.latency},
		{entity.FieldType_TargetInputTokens, entity.AggrResultFieldKey_TargetInputTokens, t.inputTokens},
		{entity.FieldType_TargetOutputTokens, entity.AggrResultFieldKey_TargetOutputTokens, t.outputTokens},
		{entity.FieldType_TargetTotalTokens, entity.AggrResultFieldKey_TargetTotalTokens, t.totalTokens},
	} {
		if err := builder(cfg.fieldType, cfg.fieldKey, cfg.aggr); err != nil {
			return nil, err
		}
	}

	return res, nil
}

type AggregatorGroup struct {
	Aggregators []Aggregator
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

func WithBucketScoreDistributionAggregator(numBuckets int) NewAggregatorGroupOption {
	return func(aggregatorGroup *AggregatorGroup) {
		aggregatorGroup.Aggregators = append(aggregatorGroup.Aggregators, NewBucketScoreDistributionAggregator(numBuckets))
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
	gslice.SortBy(aggregatorResults, func(l *entity.AggregatorResult, r *entity.AggregatorResult) bool {
		return l.AggregatorType < r.AggregatorType
	})
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

	Count int // Number of aggregated data
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

// ScoreDistributionAggregator distribution aggregator.
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
	const topN = -1
	scoreCounts := GetTopNScores(a.Score2Count, topN)
	data := &entity.AggregateData{
		DataType: entity.ScoreDistribution,
		ScoreDistribution: &entity.ScoreDistributionData{
			ScoreDistributionItems: make([]*entity.ScoreDistributionItem, 0, len(scoreCounts)),
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
	gslice.SortBy(data.ScoreDistribution.ScoreDistributionItems, func(l *entity.ScoreDistributionItem, r *entity.ScoreDistributionItem) bool {
		return l.Score < r.Score
	})

	return map[entity.AggregatorType]*entity.AggregateData{
		entity.Distribution: data,
	}
}

// BucketScoreDistributionAggregator distribution aggregator using buckets.
// Uses configurable number of buckets to distribute scores between min and max values.
// This is more memory-efficient for large datasets compared to ScoreDistributionAggregator.
type BucketScoreDistributionAggregator struct {
	Scores     []float64 // Store all scores for bucket calculation in Result()
	Min        float64   // Minimum score value
	Max        float64   // Maximum score value
	Total      int64     // Total number of scores
	NumBuckets int       // Number of buckets
}

func NewBucketScoreDistributionAggregator(numBuckets int) *BucketScoreDistributionAggregator {
	if numBuckets <= 0 {
		numBuckets = 20
	}
	return &BucketScoreDistributionAggregator{
		Scores:     make([]float64, 0),
		NumBuckets: numBuckets,
	}
}

func (a *BucketScoreDistributionAggregator) Append(score float64) {
	a.Scores = append(a.Scores, score)

	if len(a.Scores) == 1 {
		a.Min = score
		a.Max = score
	} else {
		if score < a.Min {
			a.Min = score
		}
		if score > a.Max {
			a.Max = score
		}
	}

	a.Total++
}

// getBucketIndex calculates which bucket (0 to numBuckets-1) a score belongs to
// Uses left-closed right-open intervals [start, end) to handle boundary values correctly
// For bucket i: [Min + i*width, Min + (i+1)*width)
// Boundary values (equal to bucket end) belong to the next bucket
func (a *BucketScoreDistributionAggregator) getBucketIndex(score float64) int {
	if len(a.Scores) == 0 {
		return 0
	}

	if a.Max == a.Min {
		return 0
	}

	// Handle boundary cases
	if score <= a.Min {
		return 0
	}
	if score >= a.Max {
		return a.NumBuckets - 1
	}

	// Calculate bucket index using floor to ensure left-closed right-open intervals
	// bucketWidth = (Max - Min) / NumBuckets
	// For score in [Min + i*width, Min + (i+1)*width), it belongs to bucket i
	// Using floor ensures that boundary values (equal to bucket end) go to next bucket
	bucketWidth := (a.Max - a.Min) / float64(a.NumBuckets)
	offset := score - a.Min
	bucketIndex := int(math.Floor(offset / bucketWidth))

	// Ensure bucket index is within valid range [0, NumBuckets-1]
	if bucketIndex < 0 {
		bucketIndex = 0
	} else if bucketIndex >= a.NumBuckets {
		bucketIndex = a.NumBuckets - 1
	}

	return bucketIndex
}

// getBucketRange returns the score range for a given bucket index
// bucketWidth is pre-calculated to avoid repeated computation
func (a *BucketScoreDistributionAggregator) getBucketRange(bucketIndex int, bucketWidth float64) (start, end float64) {
	if len(a.Scores) == 0 || a.Max == a.Min {
		return a.Min, a.Max
	}

	start = a.Min + float64(bucketIndex)*bucketWidth
	if bucketIndex == a.NumBuckets-1 {
		end = a.Max
	} else {
		end = a.Min + float64(bucketIndex+1)*bucketWidth
	}

	return start, end
}

func (a *BucketScoreDistributionAggregator) Result() map[entity.AggregatorType]*entity.AggregateData {
	data := &entity.AggregateData{
		DataType: entity.ScoreDistribution,
		ScoreDistribution: &entity.ScoreDistributionData{
			ScoreDistributionItems: make([]*entity.ScoreDistributionItem, 0, a.NumBuckets),
		},
	}

	// Calculate bucket counts based on final min/max
	bucketCounts := make([]int64, a.NumBuckets)
	for _, score := range a.Scores {
		bucketIndex := a.getBucketIndex(score)
		bucketCounts[bucketIndex]++
	}

	// Calculate bucket width once for all buckets
	var bucketWidth float64
	if len(a.Scores) > 0 && a.Max != a.Min {
		bucketWidth = (a.Max - a.Min) / float64(a.NumBuckets)
	}

	// Generate distribution items for all buckets (including empty buckets)
	for i := 0; i < a.NumBuckets; i++ {
		count := bucketCounts[i]

		start, end := a.getBucketRange(i, bucketWidth)
		displayEnd := end
		if i < a.NumBuckets-1 {
			displayEnd = end - 0.01
			displayEnd = math.Floor(displayEnd*100) / 100
		}
		scoreRange := fmt.Sprintf("%.2f-%.2f", start, displayEnd)

		percentage := 0.0
		if a.Total > 0 {
			percentage = float64(count) / float64(a.Total)
		}

		scoreDistributionItem := &entity.ScoreDistributionItem{
			Score:      scoreRange,
			Count:      count,
			Percentage: percentage,
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

// GetTopNScores get top N scores with highest counts
func GetTopNScores(score2Count map[float64]int64, n int) []ScoreCount {
	scoreCounts := make([]ScoreCount, 0, len(score2Count))
	for score, count := range score2Count {
		scoreCounts = append(scoreCounts, ScoreCount{Score: strconv.FormatFloat(score, 'f', 2, 64), Count: count})
	}

	// Sort by Count in descending order
	sort.Slice(scoreCounts, func(i, j int) bool {
		return scoreCounts[i].Count > scoreCounts[j].Count
	})

	if n == -1 {
		return scoreCounts
	}

	// Take top N (if less than N, return all)
	if len(scoreCounts) > n {
		aggregatedCount := int64(0)
		for i := 5; i < len(scoreCounts); i++ {
			aggregatedCount += scoreCounts[i].Count
		}
		scoreCounts = append(scoreCounts[:n], ScoreCount{Score: "Other", Count: aggregatedCount})
	}
	return scoreCounts
}

type CategoricalAggregatorGroup struct {
	Aggregators         []CategoricalAggregator
	AggregatorResultMap map[entity.AggregatorType]*entity.AggregateData
}

type CategoricalAggregator interface {
	Append(option string)
	Result() map[entity.AggregatorType]*entity.AggregateData
}

func (a *CategoricalAggregatorGroup) Append(option string) {
	for _, aggregator := range a.Aggregators {
		aggregator.Append(option)
	}
}

func (a *CategoricalAggregatorGroup) Result() *entity.AggregateResult {
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

func NewCategoricalAggregatorGroup() *CategoricalAggregatorGroup {
	m := &CategoricalAggregatorGroup{
		Aggregators: []CategoricalAggregator{},
	}

	m.Aggregators = append(m.Aggregators, &OptionDistributionAggregator{})

	return m
}

// OptionDistributionAggregator option distribution aggregator
type OptionDistributionAggregator struct {
	Option2Count map[string]int64 // optionID -> count
	Total        int64
}

// Append adds an option to the aggregator
func (a *OptionDistributionAggregator) Append(option string) {
	if a.Option2Count == nil {
		a.Option2Count = make(map[string]int64)
	}
	count, ok := a.Option2Count[option]
	if !ok {
		a.Option2Count[option] = 1
	} else {
		a.Option2Count[option] = count + 1
	}

	a.Total++
}

// Result calculates and returns the option distribution result
func (a *OptionDistributionAggregator) Result() map[entity.AggregatorType]*entity.AggregateData {
	optionCounts := GetTopNOptions(a.Option2Count, -1)
	data := &entity.AggregateData{
		DataType: entity.OptionDistribution,
		OptionDistribution: &entity.OptionDistributionData{
			OptionDistributionItems: make([]*entity.OptionDistributionItem, 0, len(optionCounts)),
		},
	}

	for _, optionCount := range optionCounts {
		optionDistributionItem := &entity.OptionDistributionItem{
			Option:     optionCount.Option,
			Count:      optionCount.Count,
			Percentage: float64(optionCount.Count) / float64(a.Total),
		}
		data.OptionDistribution.OptionDistributionItems = append(data.OptionDistribution.OptionDistributionItems, optionDistributionItem)
	}

	return map[entity.AggregatorType]*entity.AggregateData{
		entity.Distribution: data,
	}
}

// OptionCount option and its count
type OptionCount struct {
	Option string
	Count  int64
}

// GetTopNOptions get top N options with highest counts
func GetTopNOptions(option2Count map[string]int64, n int) []OptionCount {
	optionCounts := make([]OptionCount, 0, len(option2Count))
	for option, count := range option2Count {
		optionCounts = append(optionCounts, OptionCount{Option: option, Count: count})
	}

	// Sort by Count in descending order
	sort.Slice(optionCounts, func(i, j int) bool {
		return optionCounts[i].Count > optionCounts[j].Count
	})

	if n == -1 {
		return optionCounts
	}

	// Take top N (if less than N, return all)
	if len(optionCounts) > n {
		aggregatedCount := int64(0)
		for i := 5; i < len(optionCounts); i++ {
			aggregatedCount += optionCounts[i].Count
		}
		optionCounts = append(optionCounts[:n], OptionCount{Option: "Other", Count: aggregatedCount})
	}
	return optionCounts
}

func (e *ExptAggrResultServiceImpl) MakeCalcExptAggrResultLockKey(exptID int64) string {
	return fmt.Sprintf("calc_expt_result_aggr:%d", exptID)
}

func (e *ExptAggrResultServiceImpl) PublishExptAggrResultEvent(ctx context.Context, event *entity.AggrCalculateEvent, duration *time.Duration) error {
	locked, err := e.locker.Lock(ctx, e.MakeCalcExptAggrResultLockKey(event.ExperimentID), time.Minute*10)
	if err != nil {
		return err
	}

	if !locked {
		return errorx.NewByCode(errno.DuplicateCalcExptAggrResultErrorCode)
	}

	return e.publisher.PublishExptAggrCalculateEvent(ctx, []*entity.AggrCalculateEvent{event}, duration)
}
