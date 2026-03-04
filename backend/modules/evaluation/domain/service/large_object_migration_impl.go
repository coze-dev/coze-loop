// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ILargeObjectMigrationService 大对象迁移服务：将已完成实验中的 target 记录和 evaluator 记录的大对象迁移到 TOS
type ILargeObjectMigrationService interface {
	MigrateExperimentLargeObjects(ctx context.Context, spaceID int64, exptIDs []int64) (targetMigrated, evaluatorMigrated int64, err error)
}

type LargeObjectMigrationServiceImpl struct {
	exptRunLogRepo       repo.IExptRunLogRepo
	exptTurnResultRepo   repo.IExptTurnResultRepo
	evalTargetRepo       repo.IEvalTargetRepo
	evaluatorRecordRepo  repo.IEvaluatorRecordRepo
}

func NewLargeObjectMigrationService(
	exptRunLogRepo repo.IExptRunLogRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	evalTargetRepo repo.IEvalTargetRepo,
	evaluatorRecordRepo repo.IEvaluatorRecordRepo,
) ILargeObjectMigrationService {
	return &LargeObjectMigrationServiceImpl{
		exptRunLogRepo:      exptRunLogRepo,
		exptTurnResultRepo:  exptTurnResultRepo,
		evalTargetRepo:      evalTargetRepo,
		evaluatorRecordRepo: evaluatorRecordRepo,
	}
}

func (s *LargeObjectMigrationServiceImpl) MigrateExperimentLargeObjects(ctx context.Context, spaceID int64, exptIDs []int64) (targetMigrated, evaluatorMigrated int64, err error) {
	if len(exptIDs) == 0 {
		return 0, 0, nil
	}

	for _, exptID := range exptIDs {
		t, e, err := s.migrateOneExperiment(ctx, spaceID, exptID)
		if err != nil {
			return targetMigrated, evaluatorMigrated, err
		}
		targetMigrated += t
		evaluatorMigrated += e
	}

	logs.CtxInfo(ctx, "[MigrateLargeObjects] done expt_ids=%v, target_migrated=%v, evaluator_migrated=%v",
		exptIDs, targetMigrated, evaluatorMigrated)
	return targetMigrated, evaluatorMigrated, nil
}

func (s *LargeObjectMigrationServiceImpl) migrateOneExperiment(ctx context.Context, spaceID, exptID int64) (targetMigrated, evaluatorMigrated int64, err error) {
	runIDs, err := s.exptRunLogRepo.ListCompletedRunIDsByExptID(ctx, spaceID, exptID)
	if err != nil {
		return 0, 0, err
	}
	if len(runIDs) == 0 {
		logs.CtxInfo(ctx, "[MigrateLargeObjects] no completed runs for expt_id=%v", exptID)
		return 0, 0, nil
	}

	// 从 expt_turn_result 查出 target_record_id 和 evaluator_record_id（按 ID 查询，避免无索引字段全表扫描）
	targetResultIDs, evaluatorResultIDs, err := s.exptTurnResultRepo.ListTargetResultIDsAndEvaluatorResultIDsBySpaceIDAndExptRunIDs(ctx, spaceID, exptID, runIDs)
	if err != nil {
		return 0, 0, err
	}
	if len(targetResultIDs) == 0 && len(evaluatorResultIDs) == 0 {
		return 0, 0, nil
	}

	// 按 ID 查询 target 记录并迁移
	if len(targetResultIDs) > 0 {
		targetRecords, err := s.evalTargetRepo.ListEvalTargetRecordByIDsAndSpaceID(ctx, spaceID, targetResultIDs)
		if err != nil {
			return targetMigrated, evaluatorMigrated, err
		}
		for _, record := range targetRecords {
			if err := s.evalTargetRepo.SaveEvalTargetRecord(ctx, record); err != nil {
				return targetMigrated, evaluatorMigrated, err
			}
			targetMigrated++
		}
	}

	// 按 ID 查询 evaluator 记录并迁移
	if len(evaluatorResultIDs) > 0 {
		evaluatorRecords, err := s.evaluatorRecordRepo.BatchGetEvaluatorRecord(ctx, evaluatorResultIDs, false)
		if err != nil {
			return targetMigrated, evaluatorMigrated, err
		}
		for _, record := range evaluatorRecords {
			if err := s.evaluatorRecordRepo.CorrectEvaluatorRecord(ctx, record); err != nil {
				return targetMigrated, evaluatorMigrated, err
			}
			evaluatorMigrated++
		}
	}

	return targetMigrated, evaluatorMigrated, nil
}
