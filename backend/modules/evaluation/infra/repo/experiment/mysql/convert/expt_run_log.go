// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/bytedance/gg/gptr"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
)

func NewExptTurnResultRunLogConvertor() ExptTurnResultRunLogConvertor {
	return ExptTurnResultRunLogConvertor{}
}

type ExptTurnResultRunLogConvertor struct{}

func (ExptTurnResultRunLogConvertor) DO2PO(log *entity.ExptTurnResultRunLog) (*model.ExptTurnResultRunLog, error) {
	evalResIDs, err := json.Marshal(log.EvaluatorResultIds)
	if err != nil {
		return nil, errorx.Wrapf(err, "ExptTurnEvaluatorResultIDs json marshal fail")
	}
	return &model.ExptTurnResultRunLog{
		ID:                 log.ID,
		SpaceID:            log.SpaceID,
		ExptID:             log.ExptID,
		ExptRunID:          log.ExptRunID,
		ItemID:             log.ItemID,
		TurnID:             log.TurnID,
		Status:             int32(log.Status),
		LogID:              log.LogID,
		TargetResultID:     log.TargetResultID,
		EvaluatorResultIds: gptr.Of(evalResIDs),
		ErrMsg:             gptr.Of(conv.UnsafeStringToBytes(log.ErrMsg)),
	}, nil
}

func (ExptTurnResultRunLogConvertor) PO2DO(log *model.ExptTurnResultRunLog) (*entity.ExptTurnResultRunLog, error) {
	evalResIDs := new(entity.EvaluatorResults)

	if err := lo.TernaryF(
		len(gptr.Indirect(log.EvaluatorResultIds)) == 0,
		func() error { return nil },
		func() error { return json.Unmarshal(gptr.Indirect(log.EvaluatorResultIds), evalResIDs) },
	); err != nil {
		return nil, errorx.Wrapf(err, "EvaluatorResults json unmarshal fail, expt_id: %v, expt_run_id: %v", log.ExptID, log.ExptRunID)
	}

	return &entity.ExptTurnResultRunLog{
		ID:                 log.ID,
		SpaceID:            log.SpaceID,
		ExptID:             log.ExptID,
		ExptRunID:          log.ExptRunID,
		ItemID:             log.ItemID,
		TurnID:             log.TurnID,
		Status:             entity.TurnRunState(log.Status),
		LogID:              log.LogID,
		TargetResultID:     log.TargetResultID,
		EvaluatorResultIds: evalResIDs,
		ErrMsg:             conv.UnsafeBytesToString(gptr.Indirect(log.ErrMsg)),
		UpdatedAt:          log.UpdatedAt,
	}, nil
}

type ExptRunLogConvertor struct{}

func NewExptRunLogConvertor() ExptRunLogConvertor {
	return ExptRunLogConvertor{}
}

func (ExptRunLogConvertor) DO2PO(log *entity.ExptRunLog) *model.ExptRunLog {
	if log == nil {
		return nil
	}
	return &model.ExptRunLog{
		ID:            log.ID,
		SpaceID:       log.SpaceID,
		CreatedBy:     log.CreatedBy,
		ExptID:        log.ExptID,
		ExptRunID:     log.ExptRunID,
		ItemIds:       gptr.Of(log.ItemIds),
		Mode:          gptr.Of(log.Mode),
		Status:        gptr.Of(log.Status),
		PendingCnt:    log.PendingCnt,
		SuccessCnt:    log.SuccessCnt,
		FailCnt:       log.FailCnt,
		CreditCost:    log.CreditCost,
		TokenCost:     gptr.Of(log.TokenCost),
		StatusMessage: gptr.Of(log.StatusMessage),
		ProcessingCnt: log.ProcessingCnt,
		TerminatedCnt: log.TerminatedCnt,
		CreatedAt:     log.CreatedAt,
		UpdatedAt:     log.UpdatedAt,
	}
}

func (ExptRunLogConvertor) PO2DO(log *model.ExptRunLog) *entity.ExptRunLog {
	if log == nil {
		return nil
	}
	return &entity.ExptRunLog{
		ID:            log.ID,
		SpaceID:       log.SpaceID,
		CreatedBy:     log.CreatedBy,
		ExptID:        log.ExptID,
		ExptRunID:     log.ExptRunID,
		ItemIds:       gptr.Indirect(log.ItemIds),
		Mode:          gptr.Indirect(log.Mode),
		Status:        gptr.Indirect(log.Status),
		PendingCnt:    log.PendingCnt,
		SuccessCnt:    log.SuccessCnt,
		FailCnt:       log.FailCnt,
		CreditCost:    log.CreditCost,
		TokenCost:     gptr.Indirect(log.TokenCost),
		StatusMessage: gptr.Indirect(log.StatusMessage),
		ProcessingCnt: log.ProcessingCnt,
		TerminatedCnt: log.TerminatedCnt,
		CreatedAt:     log.CreatedAt,
		UpdatedAt:     log.UpdatedAt,
	}
}
