// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
)

// CentralExperimentRepo 把中心调度需要的 DB 投影读取收敛到一个适配器，
// 避免 domain service 直接依赖具体 MySQL/DAO。
type CentralExperimentRepo struct {
	exptRepo repo.IExperimentRepo
	itemRepo repo.IExptItemResultRepo
	db       db.Provider
}

func NewCentralExperimentRepo(exptRepo repo.IExperimentRepo, itemRepo repo.IExptItemResultRepo, provider db.Provider) repo.ICentralExperimentRepo {
	return &CentralExperimentRepo{exptRepo: exptRepo, itemRepo: itemRepo, db: provider}
}

func (r *CentralExperimentRepo) GetByID(ctx context.Context, exptID, spaceID int64) (*entity.Experiment, error) {
	return r.exptRepo.GetByID(ctx, exptID, spaceID)
}

func (r *CentralExperimentRepo) MGetBasicByID(ctx context.Context, exptIDs []int64) ([]*entity.Experiment, error) {
	return r.exptRepo.MGetBasicByID(ctx, exptIDs)
}

// ListCentralActive 仅扫描 Pending/Processing 状态，再按 eval_conf 中的中心调度元信息过滤。
// 不做全表扫描；该路径用于 Redis 活跃集丢失后的兜底重建。
func (r *CentralExperimentRepo) ListCentralActive(ctx context.Context, env string, limit int) ([]*entity.Experiment, error) {
	pos, err := mysql.ListCentralActive(ctx, r.db, []int32{int32(entity.ExptStatus_Pending), int32(entity.ExptStatus_Processing)}, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*entity.Experiment, 0, len(pos))
	for _, po := range pos {
		expt, cerr := convert.NewExptConverter().PO2DO(po, nil)
		if cerr != nil {
			continue
		}
		meta := entity.GetSchedulingMeta(expt.EvalConf)
		if !meta.Mode.IsCentralDispatchMode() || meta.Env != env || expt.LatestRunID <= 0 {
			continue
		}
		out = append(out, expt)
	}
	return out, nil
}

func (r *CentralExperimentRepo) ScanQueueingItems(ctx context.Context, exptID, runID, spaceID int64, limit int) ([]int64, error) {
	logs, _, err := r.itemRepo.ScanItemRunLogs(ctx, exptID, runID, &entity.ExptItemRunLogFilter{
		Status: []entity.ItemRunState{entity.ItemRunState_Queueing},
	}, 0, int64(limit), spaceID)
	if err != nil {
		return nil, err
	}
	return itemIDs(logs), nil
}

func (r *CentralExperimentRepo) ScanProcessingItems(ctx context.Context, exptID, runID, spaceID int64, limit int) ([]int64, error) {
	logs, _, err := r.itemRepo.ScanItemRunLogs(ctx, exptID, runID, &entity.ExptItemRunLogFilter{
		Status: []entity.ItemRunState{entity.ItemRunState_Processing},
	}, 0, int64(limit), spaceID)
	if err != nil {
		return nil, err
	}
	return itemIDs(logs), nil
}

func itemIDs(logs []*entity.ExptItemResultRunLog) []int64 {
	ids := make([]int64, 0, len(logs))
	for _, log := range logs {
		ids = append(ids, log.ItemID)
	}
	return ids
}

func (r *CentralExperimentRepo) MarkItemsProcessing(ctx context.Context, exptID, runID, spaceID int64, itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	return r.itemRepo.UpdateItemRunLog(ctx, exptID, runID, itemIDs, map[string]any{"status": int32(entity.ItemRunState_Processing)}, spaceID)
}

func (r *CentralExperimentRepo) MarkItemsQueueing(ctx context.Context, exptID, runID, spaceID int64, itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	return r.itemRepo.UpdateItemRunLog(ctx, exptID, runID, itemIDs, map[string]any{"status": int32(entity.ItemRunState_Queueing)}, spaceID)
}

func (r *CentralExperimentRepo) GetItemStates(ctx context.Context, exptID, runID, spaceID int64, itemIDs []int64) (map[int64]entity.ItemRunState, error) {
	if len(itemIDs) == 0 {
		return map[int64]entity.ItemRunState{}, nil
	}
	logs, err := r.itemRepo.MGetItemRunLog(ctx, exptID, runID, itemIDs, spaceID)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]entity.ItemRunState, len(logs))
	for _, log := range logs {
		out[log.ItemID] = entity.ItemRunState(log.Status)
	}
	return out, nil
}

// EnsureRunItems 校验本次 run 的 item 投影已建立；创建由 SchedulerModeFactory.EnsureRunInitialized 负责，
// 这里不重复实现一套 item 初始化逻辑，也不静默放过未初始化状态。
func (r *CentralExperimentRepo) EnsureRunItems(ctx context.Context, expt *entity.Experiment) error {
	if expt == nil || expt.LatestRunID <= 0 {
		return fmt.Errorf("invalid experiment for run items")
	}
	logs, _, err := r.itemRepo.ScanItemRunLogs(ctx, expt.ID, expt.LatestRunID, nil, 0, 1, expt.SpaceID)
	if err != nil {
		return err
	}
	if len(logs) == 0 {
		return fmt.Errorf("run items of expt %d run %d not initialized", expt.ID, expt.LatestRunID)
	}
	return nil
}
