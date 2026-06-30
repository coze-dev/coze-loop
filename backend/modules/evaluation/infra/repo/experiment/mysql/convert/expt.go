// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"context"

	"github.com/bytedance/gg/gptr"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// notificationConfRawLogLimit 反序列化失败时记录原始内容的截断长度，避免脏数据撑爆日志。
const notificationConfRawLogLimit = 256

// truncateForLog 把原始 BLOB 文本截断到指定长度用于告警日志。
func truncateForLog(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "...(truncated)"
}

func NewExptConverter() ExptConverter {
	return ExptConverter{}
}

type ExptConverter struct{}

func (ExptConverter) DO2PO(experiment *entity.Experiment) (*model.Experiment, error) {
	var exptTemplateID int64
	if experiment.ExptTemplateMeta != nil {
		exptTemplateID = experiment.ExptTemplateMeta.ID
	}

	expt := &model.Experiment{
		ID:                        experiment.ID,
		SpaceID:                   experiment.SpaceID,
		CreatedBy:                 experiment.CreatedBy,
		Name:                      experiment.Name,
		Description:               experiment.Description,
		EvalSetVersionID:          experiment.EvalSetVersionID,
		EvalSetID:                 experiment.EvalSetID,
		TargetVersionID:           experiment.TargetVersionID,
		TargetType:                int64(experiment.TargetType),
		TargetID:                  experiment.TargetID,
		Status:                    int32(experiment.Status),
		StatusMessage:             gptr.Of(conv.UnsafeStringToBytes(experiment.StatusMessage)),
		OfflineExptAnalysisStatus: int32(experiment.OfflineExptAnalysisStatus),
		StartAt:                   experiment.StartAt,
		EndAt:                     experiment.EndAt,
		LatestRunID:               experiment.LatestRunID,
		ExptTemplateID:            exptTemplateID,
		CreditCost:                int32(experiment.CreditCost),
		SourceType:                int32(experiment.SourceType),
		SourceID:                  experiment.SourceID,
		ExptType:                  int32(experiment.ExptType),
		Visibility:                int32(experiment.Visibility),
		ThreadID:                  experiment.ThreadID,
		TriggerType:               experiment.TriggerType,
	}

	if experiment.MaxAliveTime != 0 {
		expt.MaxAliveTime = gptr.Of(experiment.MaxAliveTime)
	}

	if experiment.EvalConf != nil {
		bytes, err := json.Marshal(experiment.EvalConf)
		if err != nil {
			return nil, errorx.Wrapf(err, "EvaluationConfiguration json marshal fail")
		}
		expt.EvalConf = &bytes
	}
	if experiment.TrialRunItemCount != 0 {
		expt.TrialRunItemCount = gptr.Of(experiment.TrialRunItemCount)
	}

	// notification_conf：nil（历史/未配置）→ 不写列（NULL），上层读到 nil 时按默认行为处理。
	if experiment.NotificationConf != nil {
		bytes, err := json.Marshal(experiment.NotificationConf)
		if err != nil {
			return nil, errorx.Wrapf(err, "NotificationConf json marshal fail")
		}
		expt.NotificationConf = &bytes
	}

	return expt, nil
}

func (ExptConverter) PO2DO(expt *model.Experiment, refs []*model.ExptEvaluatorRef) (*entity.Experiment, error) {
	evalConf := new(entity.EvaluationConfiguration)
	if err := lo.TernaryF(
		len(gptr.Indirect(expt.EvalConf)) == 0,
		func() error { return nil },
		func() error { return json.Unmarshal(gptr.Indirect(expt.EvalConf), evalConf) },
	); err != nil {
		return nil, errorx.Wrapf(err, "EvaluationConfiguration json unmarshal fail, expt_id: %v, raw: %v", expt.ID, conv.UnsafeBytesToString(gptr.Indirect(expt.EvalConf)))
	}

	evaluatorVersionRef := make([]*entity.ExptEvaluatorVersionRef, 0, len(refs))
	for _, ref := range refs {
		evaluatorVersionRef = append(evaluatorVersionRef, &entity.ExptEvaluatorVersionRef{
			EvaluatorVersionID: ref.EvaluatorVersionID,
			EvaluatorID:        ref.EvaluatorID,
		})
	}

	// notification_conf：NULL/空（历史实验/未配置）→ nil，上层按 DefaultNotificationConf 兜底（向后兼容、零迁移）。
	// 脏数据/旧格式（如 webhook.urls 历史上存成 string 而非 []string）导致 unmarshal 失败时，
	// 同样降级为 nil 并记 warn，不阻断该实验返回，更不能让整个 list 查询失败。
	var notificationConf *entity.NotificationConf
	if raw := gptr.Indirect(expt.NotificationConf); len(raw) > 0 {
		parsed := new(entity.NotificationConf)
		if err := json.Unmarshal(raw, parsed); err != nil {
			logs.CtxWarn(context.Background(),
				"NotificationConf json unmarshal fail, degrade to default, expt_id: %v, err: %v, raw: %v",
				expt.ID, err, truncateForLog(conv.UnsafeBytesToString(raw), notificationConfRawLogLimit))
			notificationConf = nil
		} else {
			notificationConf = parsed
		}
	}

	res := &entity.Experiment{
		ID:                        expt.ID,
		SpaceID:                   expt.SpaceID,
		CreatedBy:                 expt.CreatedBy,
		Name:                      expt.Name,
		Description:               expt.Description,
		EvalSetVersionID:          expt.EvalSetVersionID,
		EvalSetID:                 expt.EvalSetID,
		TargetVersionID:           expt.TargetVersionID,
		TargetType:                entity.EvalTargetType(expt.TargetType),
		TargetID:                  expt.TargetID,
		EvaluatorVersionRef:       evaluatorVersionRef,
		EvalConf:                  evalConf,
		Status:                    entity.ExptStatus(expt.Status),
		StatusMessage:             conv.UnsafeBytesToString(gptr.Indirect(expt.StatusMessage)),
		OfflineExptAnalysisStatus: entity.OfflineExptAnalysisStatus(expt.OfflineExptAnalysisStatus),
		LatestRunID:               expt.LatestRunID,
		CreditCost:                entity.CreditCost(expt.CreditCost),
		StartAt:                   expt.StartAt,
		EndAt:                     expt.EndAt,
		SourceType:                entity.SourceType(expt.SourceType),
		SourceID:                  expt.SourceID,
		ExptType:                  entity.ExptType(expt.ExptType),
		MaxAliveTime:              gptr.Indirect(expt.MaxAliveTime),
		Visibility:                entity.Visibility(expt.Visibility),
		ThreadID:                  expt.ThreadID,
		TrialRunItemCount:         gptr.Indirect(expt.TrialRunItemCount),
		TriggerType:               expt.TriggerType,
		NotificationConf:          notificationConf,
	}

	// 如果数据库中有模板 ID，则在 ExptTemplateMeta 中回填 ID，方便上层按模板 ID 查询和聚合
	if expt.ExptTemplateID != 0 {
		res.ExptTemplateMeta = &entity.ExptTemplateMeta{
			ID: expt.ExptTemplateID,
		}
	}

	return res, nil
}
