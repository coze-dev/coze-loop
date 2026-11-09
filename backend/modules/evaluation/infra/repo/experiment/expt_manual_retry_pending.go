// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"
	redisc "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const manualRetryReceiptRetention = time.Hour

type pendingRetryResult struct {
	Receipt   *entity.ManualRetryReceipt
	ErrorCode int32
}

func pendingRetryKey(space, expt, run int64) string {
	return fmt.Sprintf("[experiment]pending_manual_retry:%d:%d:%d", space, expt, run)
}

func pendingReceiptKey(p *repo.ManualRetryPending) string {
	r := p.Request
	return pendingRetryKey(r.SpaceID, r.ExptID, r.RunID) + ":" + r.RequestID
}

func (r *ExptManualRetryRepoImpl) GetPendingRetry(ctx context.Context, space, expt, run int64) (*repo.ManualRetryPending, error) {
	if r.cache == nil {
		return nil, nil
	}
	raw, err := r.cache.Get(ctx, pendingRetryKey(space, expt, run)).Bytes()
	if err == redisc.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p repo.ManualRetryPending
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if p.Request == nil || p.Request.SpaceID != space || p.Request.ExptID != expt || p.Request.RunID != run || p.Request.RequestID == "" {
		return nil, fmt.Errorf("invalid pending manual retry identity")
	}
	return &p, nil
}

func (r *ExptManualRetryRepoImpl) withRetryRunRows(ctx context.Context, request *entity.RequeueTerminalItemsRequest, next func(*model.Experiment, *model.ExptRunLog, []db.Option) error) error {
	return r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx), db.WithMaster(), db.WithSelectForUpdate()}
		expt, err := mysql.NewExptDAO(r.provider).GetByID(ctx, request.ExptID, opts...)
		if err != nil {
			return err
		}
		run, err := mysql.NewExptRunLogDAO(r.provider).Get(ctx, request.ExptID, request.RunID, opts...)
		if err != nil {
			return err
		}
		if expt.SpaceID != request.SpaceID || run.SpaceID != request.SpaceID {
			return errorx.NewByCode(errno.ManualRetryRunConflictCode)
		}
		return next(expt, run, opts)
	}, db.WithMaster())
}

func (r *ExptManualRetryRepoImpl) RegisterPendingRetry(ctx context.Context, p *repo.ManualRetryPending) error {
	if r.cache == nil {
		return fmt.Errorf("pending manual retry cache unavailable")
	}
	err := r.registerPendingRetry(ctx, p)
	status, busy := errorx.FromStatusError(err)
	if !busy || status.Code() != errno.ManualRetryBusyCode {
		return err
	}
	previous, readErr := r.GetPendingRetry(ctx, p.Request.SpaceID, p.Request.ExptID, p.Request.RunID)
	if readErr != nil {
		return readErr
	}
	if previous != nil {
		if previous.ExpiresAtMS > time.Now().UnixMilli() {
			return err
		}
		// Resolve outside the registration transaction: it acquires the same
		// experiment/run rows and preserves a committed result before deletion.
		_, resolveErr := r.ResolvePendingRetry(ctx, previous, true)
		if resolveErr != nil {
			resolved, ok := errorx.FromStatusError(resolveErr)
			if !ok || resolved.Code() != errno.ManualRetryBusyCode {
				return resolveErr
			}
		}
	}
	return r.registerPendingRetry(ctx, p)
}

func (r *ExptManualRetryRepoImpl) registerPendingRetry(ctx context.Context, p *repo.ManualRetryPending) error {
	return r.withRetryRunRows(ctx, p.Request, func(expt *model.Experiment, run *model.ExptRunLog, _ []db.Option) error {
		if expt.LatestRunID != p.Request.RunID {
			return errorx.NewByCode(errno.ManualRetryRunConflictCode)
		}
		if expt.Status != int32(entity.ExptStatus_Processing) || (gptr.Indirect(run.Status) != int64(entity.ExptStatus_Processing) && gptr.Indirect(run.Status) != int64(entity.ExptStatus_Pending)) {
			return errorx.NewByCode(errno.ManualRetryRunFinishedCode)
		}
		if p.ExpiresAtMS <= time.Now().UnixMilli() {
			return errorx.NewByCode(errno.ManualRetryBusyCode)
		}
		raw, err := json.Marshal(p)
		if err != nil {
			return err
		}
		ok, err := r.cache.SetNX(ctx, pendingRetryKey(p.Request.SpaceID, p.Request.ExptID, p.Request.RunID), raw, manualRetryReceiptRetention).Result()
		if err != nil {
			return err
		}
		if !ok {
			return errorx.NewByCode(errno.ManualRetryBusyCode)
		}
		return nil
	})
}

// Called by RequeueTerminalItems after it has acquired the same SQL row locks.
func (r *ExptManualRetryRepoImpl) CheckPendingRetry(ctx context.Context, p *repo.ManualRetryPending) error {
	current, err := r.GetPendingRetry(ctx, p.Request.SpaceID, p.Request.ExptID, p.Request.RunID)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if current == nil || current.Request.RequestID != p.Request.RequestID || current.ExpiresAtMS <= time.Now().UnixMilli() {
		return errorx.NewByCode(errno.ManualRetryBusyCode)
	}
	return nil
}

func (r *ExptManualRetryRepoImpl) PreparePendingRetryCommit(ctx context.Context, p *repo.ManualRetryPending, receipt *entity.ManualRetryReceipt, ids []int64) error {
	if err := r.CheckPendingRetry(ctx, p); err != nil {
		return err
	}
	receipt.RequestID = p.Request.RequestID
	p.PreparedReceipt, p.AcceptedItemIDs = receipt, ids
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err := r.cache.Set(ctx, pendingRetryKey(p.Request.SpaceID, p.Request.ExptID, p.Request.RunID), raw, manualRetryReceiptRetention).Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	return nil
}

func (r *ExptManualRetryRepoImpl) CompletePendingRetry(ctx context.Context, p *repo.ManualRetryPending, receipt *entity.ManualRetryReceipt, cause error) error {
	result := pendingRetryResult{Receipt: receipt}
	if cause != nil {
		result.ErrorCode = errno.CommonInternalErrorCode
		if status, ok := errorx.FromStatusError(cause); ok {
			result.ErrorCode = status.Code()
		}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := r.cache.Set(ctx, pendingReceiptKey(p), raw, manualRetryReceiptRetention).Err(); err != nil {
		return err
	}
	return r.deletePendingRetry(ctx, p)
}

func (r *ExptManualRetryRepoImpl) deletePendingRetry(ctx context.Context, p *repo.ManualRetryPending) error {
	const script = `local v = redis.call('GET', KEYS[1]); if v and cjson.decode(v).Request.RequestID == ARGV[1] then return redis.call('DEL', KEYS[1]) end; return 0`
	return r.cache.Eval(ctx, script, []string{pendingRetryKey(p.Request.SpaceID, p.Request.ExptID, p.Request.RunID)}, p.Request.RequestID).Err()
}

func (r *ExptManualRetryRepoImpl) ResolvePendingRetry(ctx context.Context, p *repo.ManualRetryPending, cancel bool) (*entity.ManualRetryReceipt, error) {
	if r.cache == nil {
		return nil, fmt.Errorf("pending manual retry cache unavailable")
	}
	var receipt *entity.ManualRetryReceipt
	err := r.withRetryRunRows(ctx, p.Request, func(expt *model.Experiment, run *model.ExptRunLog, opts []db.Option) error {
		raw, err := r.cache.Get(ctx, pendingReceiptKey(p)).Bytes()
		if err == nil {
			var result pendingRetryResult
			if err := json.Unmarshal(raw, &result); err != nil {
				return err
			}
			if result.ErrorCode != 0 {
				if err := r.deletePendingRetry(ctx, p); err != nil {
					return err
				}
				return errorx.NewByCode(result.ErrorCode)
			}
			receipt = result.Receipt
			if receipt == nil || receipt.RunID != p.Request.RunID || receipt.RequestID != p.Request.RequestID {
				return fmt.Errorf("invalid manual retry result identity")
			}
			if err := r.deletePendingRetry(ctx, p); err != nil {
				logs.CtxWarn(ctx, "committed manual retry pending cleanup failed, request_id=%s, err=%v", p.Request.RequestID, err)
			}
			return nil
		}
		if err != redisc.Nil {
			return err
		}
		current, err := r.GetPendingRetry(ctx, p.Request.SpaceID, p.Request.ExptID, p.Request.RunID)
		if err != nil {
			return err
		}
		if current != nil && current.Request.RequestID == p.Request.RequestID {
			p = current
		}
		// The canonical token survives ordinary dispatch and run completion. The
		// active-only source reader cannot recover a receipt after run completion.
		if p.PreparedReceipt != nil && len(p.AcceptedItemIDs) > 0 {
			items, err := mysql.NewExptItemResultDAO(r.provider).BatchGet(ctx, p.Request.SpaceID, p.Request.ExptID, p.AcceptedItemIDs, opts...)
			if err != nil {
				return err
			}
			matched, unchanged := 0, 0
			previous := make(map[int64]*entity.PreparedRetryItem, len(p.Request.Items))
			for _, item := range p.Request.Items {
				previous[item.SourceItem.ItemID] = item
			}
			for _, item := range items {
				ext, err := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
				if err != nil {
					return err
				}
				token, err := entity.ParseManualRetryContext(ext)
				if err != nil {
					return err
				}
				if token != nil && token.RunID == p.Request.RunID && token.RequestID == p.Request.RequestID {
					matched++
				}
				old := previous[item.ItemID]
				priorRequest := ""
				if token != nil {
					priorRequest = token.RequestID
				}
				if old != nil && item.ExptRunID == old.PreviousRunID && priorRequest == old.PreviousRequestID {
					unchanged++
				}
			}
			if matched > 0 {
				if matched != len(p.AcceptedItemIDs) || matched != int(p.PreparedReceipt.AcceptedCount) {
					return fmt.Errorf("manual retry commit evidence is incomplete")
				}
				receipt = p.PreparedReceipt
				return r.CompletePendingRetry(ctx, p, receipt, nil)
			}
			if unchanged != len(p.AcceptedItemIDs) {
				return fmt.Errorf("manual retry request %s result cannot be proven from its primary source token", p.Request.RequestID)
			}
		}
		finished := expt.LatestRunID != p.Request.RunID || expt.Status != int32(entity.ExptStatus_Processing) || (gptr.Indirect(run.Status) != int64(entity.ExptStatus_Processing) && gptr.Indirect(run.Status) != int64(entity.ExptStatus_Pending))
		if cancel || finished || p.ExpiresAtMS <= time.Now().UnixMilli() {
			if err := r.deletePendingRetry(ctx, p); err != nil {
				return err
			}
			if finished {
				return errorx.NewByCode(errno.ManualRetryRunFinishedCode)
			}
			return errorx.NewByCode(errno.ManualRetryBusyCode)
		}
		return nil
	})
	return receipt, err
}
