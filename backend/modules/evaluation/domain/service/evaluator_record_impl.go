// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var (
	evaluatorRecordServiceOnce      = sync.Once{}
	singletonEvaluatorRecordService EvaluatorRecordService
)

// NewEvaluatorServiceImpl 创建 EvaluatorService 实例
func NewEvaluatorRecordServiceImpl(idgen idgen.IIDGenerator,
	evaluatorRecordRepo repo.IEvaluatorRecordRepo,
	exptPublisher events.ExptEventPublisher,
	evaluatorPublisher events.EvaluatorEventPublisher,
	userInfoService userinfo.UserInfoService,
	exptRepo repo.IExperimentRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
) EvaluatorRecordService {
	evaluatorRecordServiceOnce.Do(func() {
		singletonEvaluatorRecordService = &EvaluatorRecordServiceImpl{
			evaluatorRecordRepo: evaluatorRecordRepo,
			idgen:               idgen,
			exptPublisher:       exptPublisher,
			evaluatorPublisher:  evaluatorPublisher,
			userInfoService:     userInfoService,
			exptRepo:            exptRepo,
			exptTurnResultRepo:  exptTurnResultRepo,
		}
	})
	return singletonEvaluatorRecordService
}

// EvaluatorRecordServiceImpl 实现 EvaluatorService 接口
type EvaluatorRecordServiceImpl struct {
	idgen               idgen.IIDGenerator
	evaluatorRecordRepo repo.IEvaluatorRecordRepo
	exptPublisher       events.ExptEventPublisher
	evaluatorPublisher  events.EvaluatorEventPublisher
	userInfoService     userinfo.UserInfoService
	exptRepo            repo.IExperimentRepo
	exptTurnResultRepo  repo.IExptTurnResultRepo
}

// CorrectEvaluatorRecord 创建 evaluator_version 运行结果
func (s *EvaluatorRecordServiceImpl) CorrectEvaluatorRecord(ctx context.Context, evaluatorRecordDO *entity.EvaluatorRecord, correctionDO *entity.Correction) error {
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	correctionDO.UpdatedBy = userIDInContext
	if evaluatorRecordDO.EvaluatorOutputData == nil {
		evaluatorRecordDO.EvaluatorOutputData = &entity.EvaluatorOutputData{}
	}
	if evaluatorRecordDO.EvaluatorOutputData.EvaluatorResult == nil {
		evaluatorRecordDO.EvaluatorOutputData.EvaluatorResult = &entity.EvaluatorResult{}
	}
	evaluatorRecordDO.EvaluatorOutputData.EvaluatorResult.Correction = correctionDO
	if evaluatorRecordDO.BaseInfo == nil {
		evaluatorRecordDO.BaseInfo = &entity.BaseInfo{}
	}
	evaluatorRecordDO.BaseInfo.UpdatedBy = &entity.UserInfo{
		UserID: gptr.Of(userIDInContext),
	}
	evaluatorRecordDO.BaseInfo.UpdatedAt = gptr.Of(time.Now().UnixMilli())
	err := s.evaluatorRecordRepo.CorrectEvaluatorRecord(ctx, evaluatorRecordDO)
	if err != nil {
		return err
	}

	// 如果修改了评估器得分，优先在服务层重新计算并回写行级加权得分
	if correctionDO != nil && correctionDO.Score != nil {
		if err := s.recalculateWeightedScoreForTurn(ctx, evaluatorRecordDO); err != nil {
			logs.CtxError(ctx, "Failed to recalculate weighted score in CorrectEvaluatorRecord, expt_id: %v, item_id: %v, turn_id: %v, err: %v",
				evaluatorRecordDO.ExperimentID, evaluatorRecordDO.ItemID, evaluatorRecordDO.TurnID, err)
			// 不向上抛错，避免影响主流程和后续聚合事件
		}
	}

	expt, err := s.exptRepo.GetByID(ctx, evaluatorRecordDO.ExperimentID, evaluatorRecordDO.SpaceID)
	if err != nil {
		return err
	}
	// 发送聚合报告计算消息
	evaluatorVersionIDStr := strconv.FormatInt(evaluatorRecordDO.EvaluatorVersionID, 10)
	if err = s.exptPublisher.PublishExptAggrCalculateEvent(ctx, []*entity.AggrCalculateEvent{{
		ExperimentID:  evaluatorRecordDO.ExperimentID,
		SpaceID:       evaluatorRecordDO.SpaceID,
		CalculateMode: entity.UpdateSpecificField,
		SpecificFieldInfo: &entity.SpecificFieldInfo{
			FieldKey:  evaluatorVersionIDStr,
			FieldType: entity.FieldType_EvaluatorScore,
		},
	}}, gptr.Of(time.Second*3)); err != nil {
		logs.CtxError(ctx, "Failed to send AggrCalculateEvent, evaluatorVersionIDStr: %s, experimentID: %s, err: %v", evaluatorVersionIDStr, evaluatorRecordDO.ExperimentID, err)
	}
	if expt.ExptType == entity.ExptType_Online {
		// 发送在线实验结果变更消息
		if err = s.evaluatorPublisher.PublishEvaluatorRecordCorrection(ctx, &entity.EvaluatorRecordCorrectionEvent{
			EvaluatorResult:    evaluatorRecordDO.EvaluatorOutputData.EvaluatorResult,
			EvaluatorRecordID:  evaluatorRecordDO.ID,
			EvaluatorVersionID: evaluatorRecordDO.EvaluatorVersionID,
			Ext:                evaluatorRecordDO.Ext,
			CreatedAt:          gptr.Indirect(evaluatorRecordDO.BaseInfo.CreatedAt),
			UpdatedAt:          gptr.Indirect(evaluatorRecordDO.BaseInfo.UpdatedAt),
		}, gptr.Of(time.Second*3)); err != nil {
			return err
		}
	}

	if err = s.exptPublisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{
		ExperimentID: evaluatorRecordDO.ExperimentID,
		SpaceID:      evaluatorRecordDO.SpaceID,
		ItemID:       []int64{evaluatorRecordDO.ItemID},
	}, nil); err != nil {
		logs.CtxError(ctx, "Failed to send ExptTurnResultFilterEvent, err: %v", err)
	}

	err = s.exptPublisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{
		ExperimentID: evaluatorRecordDO.ExperimentID,
		SpaceID:      evaluatorRecordDO.SpaceID,
		ItemID:       []int64{evaluatorRecordDO.ItemID},
		RetryTimes:   ptr.Of(int32(0)),
		FilterType:   ptr.Of(entity.UpsertExptTurnResultFilterTypeCheck),
	}, ptr.Of(10*time.Second))
	if err != nil {
		return err
	}

	return nil
}

// recalculateWeightedScoreForTurn 在不依赖其他 service 的前提下，基于 repo 重新计算并回写某一轮的加权得分
func (s *EvaluatorRecordServiceImpl) recalculateWeightedScoreForTurn(ctx context.Context, rec *entity.EvaluatorRecord) error {
	// 1. 获取该轮次的 turn_result
	turnResult, err := s.exptTurnResultRepo.Get(ctx, rec.SpaceID, rec.ExperimentID, rec.ItemID, rec.TurnID)
	if err != nil {
		return err
	}
	if turnResult == nil {
		// 找不到直接返回，不算错误
		return nil
	}

	// 2. 获取实验配置，判断是否开启加权得分
	expt, err := s.exptRepo.GetByID(ctx, rec.ExperimentID, rec.SpaceID)
	if err != nil {
		return err
	}
	if expt == nil || expt.EvalConf == nil || expt.EvalConf.ConnectorConf.EvaluatorsConf == nil ||
		!expt.EvalConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight {
		// 未启用加权得分，则不需要重算
		return nil
	}

	// 3. 获取该 turn_result 的所有评估器结果引用
	refs, err := s.exptTurnResultRepo.BatchGetTurnEvaluatorResultRef(ctx, rec.SpaceID, []int64{turnResult.ID})
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}

	// 4. 收集所有 evaluator_result_id
	evaluatorResultIDs := make([]int64, 0, len(refs))
	for _, r := range refs {
		if r.EvaluatorResultID > 0 {
			evaluatorResultIDs = append(evaluatorResultIDs, r.EvaluatorResultID)
		}
	}
	if len(evaluatorResultIDs) == 0 {
		return nil
	}

	// 5. 批量获取 evaluator_record
	records, err := s.evaluatorRecordRepo.BatchGetEvaluatorRecord(ctx, evaluatorResultIDs, false)
	if err != nil {
		return err
	}
	version2Record := make(map[int64]*entity.EvaluatorRecord, len(records))
	for _, r := range records {
		if r != nil {
			version2Record[r.EvaluatorVersionID] = r
		}
	}

	// 6. 构建权重映射
	scoreWeights := make(map[int64]float64)
	if expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf != nil {
		for _, ec := range expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
			if ec != nil && ec.ScoreWeight != nil && *ec.ScoreWeight > 0 && ec.EvaluatorVersionID > 0 {
				scoreWeights[ec.EvaluatorVersionID] = *ec.ScoreWeight
			}
		}
	}

	// 7. 计算新的 weighted_score（共用 expt_result_impl.go 中的 calculateWeightedScore）
	ws := calculateWeightedScore(version2Record, scoreWeights)

	// 8. 写回 expt_turn_result 的 weighted_score 字段
	updateFields := map[string]any{
		"weighted_score": ws,
	}
	itemTurnIDs := []*entity.ItemTurnID{{
		ItemID: rec.ItemID,
		TurnID: rec.TurnID,
	}}
	return s.exptTurnResultRepo.UpdateTurnResults(ctx, rec.ExperimentID, itemTurnIDs, rec.SpaceID, updateFields)
}

func (s *EvaluatorRecordServiceImpl) GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool) (*entity.EvaluatorRecord, error) {
	return s.evaluatorRecordRepo.GetEvaluatorRecord(ctx, evaluatorRecordID, includeDeleted)
}

func (s *EvaluatorRecordServiceImpl) BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted bool) ([]*entity.EvaluatorRecord, error) {
	records, err := s.evaluatorRecordRepo.BatchGetEvaluatorRecord(ctx, evaluatorRecordIDs, includeDeleted)
	if err != nil {
		return nil, err
	}
	s.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDO2UserInfoDomainCarrier(records))
	return records, nil
}
