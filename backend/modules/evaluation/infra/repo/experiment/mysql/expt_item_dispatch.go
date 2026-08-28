// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/expt_item_dispatch.go -package=mocks . IExptItemDispatchDAO

// IExptItemDispatchDAO run log 派发投影的数据访问。
type IExptItemDispatchDAO interface {
	ClaimQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]int64, error)
	ResetQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]int64, error)
	LoadDispatchRuntime(ctx context.Context, spaceID, exptID, exptRunID int64, candidateLimit int) (*repo.ExptDispatchRuntime, error)
	StartReservedItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64) (bool, error)
	RequeueProcessingItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64) (bool, error)
	MGetDispatchObservations(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]*repo.ExptDispatchObservation, error)
}

func NewExptItemDispatchDAO(db db.Provider) IExptItemDispatchDAO {
	return &exptItemDispatchDAOImpl{db: db}
}

type exptItemDispatchDAOImpl struct {
	db db.Provider
}

// claimBatchSize 单次 CAS 的 item 上限。
// 不做无界 IN 查询：单拍授予数本身有预算上限，超大 IN 会让执行计划退化。
const claimBatchSize = 500

func (d *exptItemDispatchDAOImpl) ClaimQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]int64, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	claimed := make([]int64, 0, len(itemIDs))
	for _, batch := range chunkInt64(itemIDs, claimBatchSize) {
		// 逐个 CAS 而非一条批量 UPDATE：批量 UPDATE 只能拿到总 RowsAffected，
		// 无法知道**哪些** item 成功。而调用方必须精确知道，才能只发布成功项、
		// 并释放失败项的 Redis reservation —— 发布了未 claim 成功的 item 会造成双驱动。
		for _, itemID := range batch {
			res := d.db.NewSession(ctx).Model(&model.ExptItemResultRunLog{}).
				Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ?",
					spaceID, exptID, exptRunID, itemID).
				Where("status = ?", int32(entity.ItemRunState_Queueing)).
				Where("quota_reservation_state = ?", int32(entity.QuotaReservationStateNone)).
				Update("quota_reservation_state", int32(entity.QuotaReservationStateReserved))
			if res.Error != nil {
				return claimed, errorx.Wrapf(res.Error, "claim quota reserved fail, expt_run_id: %v, item_id: %v", exptRunID, itemID)
			}
			if res.RowsAffected > 0 {
				claimed = append(claimed, itemID)
			}
		}
	}
	return claimed, nil
}

func (d *exptItemDispatchDAOImpl) ResetQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]int64, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	reset := make([]int64, 0, len(itemIDs))
	for _, batch := range chunkInt64(itemIDs, claimBatchSize) {
		for _, itemID := range batch {
			// 条件里带 status=Queueing：若 item 已被 consumer 推进到 Processing，
			// 说明消息其实已投递成功，退回投影会让它被重复授予。
			res := d.db.NewSession(ctx).Model(&model.ExptItemResultRunLog{}).
				Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ?",
					spaceID, exptID, exptRunID, itemID).
				Where("status = ?", int32(entity.ItemRunState_Queueing)).
				Where("quota_reservation_state = ?", int32(entity.QuotaReservationStateReserved)).
				Update("quota_reservation_state", int32(entity.QuotaReservationStateNone))
			if res.Error != nil {
				return reset, errorx.Wrapf(res.Error, "reset quota reserved fail, expt_run_id: %v, item_id: %v", exptRunID, itemID)
			}
			if res.RowsAffected > 0 {
				reset = append(reset, itemID)
			}
		}
	}
	return reset, nil
}

func (d *exptItemDispatchDAOImpl) LoadDispatchRuntime(ctx context.Context, spaceID, exptID, exptRunID int64, candidateLimit int) (*repo.ExptDispatchRuntime, error) {
	if candidateLimit <= 0 {
		candidateLimit = defaultLimit
	}

	// 一次查询取回两类记录，而非分两次查 Processing 和 Queueing：
	// 分两次的话，两次查询之间 item 可能从 Queueing 变成 Processing，
	// 于是它在两个结果里都不出现（第一次查时还是 Queueing，第二次查时已是 Processing 但
	// 第二次只查 Queueing），导致并发占用少算、超发。
	type row struct {
		ItemID                int64 `gorm:"column:item_id"`
		Status                int32 `gorm:"column:status"`
		QuotaReservationState int32 `gorm:"column:quota_reservation_state"`
	}
	var rows []row

	err := d.db.NewSession(ctx).Model(&model.ExptItemResultRunLog{}).
		Select("item_id, status, quota_reservation_state").
		Where("space_id = ? AND expt_id = ? AND expt_run_id = ?", spaceID, exptID, exptRunID).
		Where("status IN ?", []int32{
			int32(entity.ItemRunState_Queueing),
			int32(entity.ItemRunState_Processing),
		}).
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, errorx.Wrapf(err, "load dispatch runtime fail, expt_run_id: %v", exptRunID)
	}

	observations := make([]*repo.ExptDispatchObservation, 0, len(rows))
	for _, r := range rows {
		observations = append(observations, &repo.ExptDispatchObservation{
			ItemID:                r.ItemID,
			Status:                r.Status,
			QuotaReservationState: entity.QuotaReservationState(r.QuotaReservationState),
		})
	}

	return classifyDispatchRuntime(observations, candidateLimit), nil
}

// classifyDispatchRuntime 把 run log 观测按 (status, 预占态) 分成「占用并发」与「可授予候选」。
//
// 抽成纯函数是为了让这段分类规则可直接单测 —— 它是本次改动的正确性核心：
// 漏把 Queueing/reserved 计入占用会导致 deficit 高估、同一批 item 被反复授予；
// 误把它算进候选则会重复派发。
func classifyDispatchRuntime(observations []*repo.ExptDispatchObservation, candidateLimit int) *repo.ExptDispatchRuntime {
	runtime := &repo.ExptDispatchRuntime{}
	for _, o := range observations {
		if o == nil {
			continue
		}
		switch {
		case o.Status == int32(entity.ItemRunState_Processing):
			runtime.OccupiedItemIDs = append(runtime.OccupiedItemIDs, o.ItemID)
		case o.QuotaReservationState.IsQuotaReserved():
			// Queueing + reserved：已预占未消费，计入占用且不得再次授予
			runtime.OccupiedItemIDs = append(runtime.OccupiedItemIDs, o.ItemID)
		default:
			// Queueing + none：唯一的可授予候选
			if candidateLimit <= 0 || len(runtime.CandidateItemIDs) < candidateLimit {
				runtime.CandidateItemIDs = append(runtime.CandidateItemIDs, o.ItemID)
			}
		}
	}
	return runtime
}

func (d *exptItemDispatchDAOImpl) StartReservedItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64) (bool, error) {
	// Queueing/reserved → Processing/none：预占兑现为实际执行。
	// 同时清掉预占标记，因为 Processing 本身已经代表占用，留着 reserved 会让对账看到
	// 一个自相矛盾的状态（Processing 且 reserved）。
	res := d.db.NewSession(ctx).Model(&model.ExptItemResultRunLog{}).
		Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ?",
			spaceID, exptID, exptRunID, itemID).
		Where("status = ?", int32(entity.ItemRunState_Queueing)).
		Where("quota_reservation_state = ?", int32(entity.QuotaReservationStateReserved)).
		Updates(map[string]any{
			"status":                  int32(entity.ItemRunState_Processing),
			"quota_reservation_state": int32(entity.QuotaReservationStateNone),
		})
	if res.Error != nil {
		return false, errorx.Wrapf(res.Error, "start reserved item fail, expt_run_id: %v, item_id: %v", exptRunID, itemID)
	}
	return res.RowsAffected > 0, nil
}

func (d *exptItemDispatchDAOImpl) RequeueProcessingItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64) (bool, error) {
	// Processing/none → Queueing/none：孤儿 item 退回授予候选。
	//
	// 条件里同时钉住 status=Processing 与 quota_reservation_state=none，是为了让这次退回
	// 只作用于「已兑现执行但额度已消失」这一种形状：
	//   - status 若已是终态，说明消息只是迟到，item 早就跑完了，不能退回（会重复执行）；
	//   - qrs 若是 reserved，说明它还没被 StartReservedItem 兑现，该走 ResetQuotaReserved。
	res := d.db.NewSession(ctx).Model(&model.ExptItemResultRunLog{}).
		Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ?",
			spaceID, exptID, exptRunID, itemID).
		Where("status = ?", int32(entity.ItemRunState_Processing)).
		Where("quota_reservation_state = ?", int32(entity.QuotaReservationStateNone)).
		Update("status", int32(entity.ItemRunState_Queueing))
	if res.Error != nil {
		return false, errorx.Wrapf(res.Error, "requeue processing item fail, expt_run_id: %v, item_id: %v", exptRunID, itemID)
	}
	return res.RowsAffected > 0, nil
}

func (d *exptItemDispatchDAOImpl) MGetDispatchObservations(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]*repo.ExptDispatchObservation, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	type row struct {
		ItemID                int64 `gorm:"column:item_id"`
		Status                int32 `gorm:"column:status"`
		QuotaReservationState int32 `gorm:"column:quota_reservation_state"`
	}

	out := make([]*repo.ExptDispatchObservation, 0, len(itemIDs))
	for _, batch := range chunkInt64(itemIDs, claimBatchSize) {
		var rows []row
		err := d.db.NewSession(ctx).Model(&model.ExptItemResultRunLog{}).
			Select("item_id, status, quota_reservation_state").
			Where("space_id = ? AND expt_id = ? AND expt_run_id = ?", spaceID, exptID, exptRunID).
			Where("item_id IN ?", batch).
			Find(&rows).Error
		if err != nil {
			return nil, errorx.Wrapf(err, "mget dispatch observations fail, expt_run_id: %v", exptRunID)
		}
		for _, r := range rows {
			out = append(out, &repo.ExptDispatchObservation{
				ItemID:                r.ItemID,
				Status:                r.Status,
				QuotaReservationState: entity.QuotaReservationState(r.QuotaReservationState),
			})
		}
	}
	return out, nil
}

func chunkInt64(in []int64, size int) [][]int64 {
	if size <= 0 || len(in) == 0 {
		return nil
	}
	out := make([][]int64, 0, (len(in)+size-1)/size)
	for i := 0; i < len(in); i += size {
		end := i + size
		if end > len(in) {
			end = len(in)
		}
		out = append(out, in[i:end])
	}
	return out
}
