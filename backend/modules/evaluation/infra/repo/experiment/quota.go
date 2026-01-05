// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewQuotaService(quotaDAO dao.IQuotaDAO, mutex lock.ILocker) repo.QuotaRepo {
	return &QuotaRepoImpl{
		quotaDAO: quotaDAO,
		mutex:    mutex,
	}
}

type QuotaRepoImpl struct {
	quotaDAO dao.IQuotaDAO
	mutex    lock.ILocker
}

func (q *QuotaRepoImpl) CreateOrUpdate(ctx context.Context, spaceID int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), session *entity.Session) error {
	return backoff.RetryWithElapsedTime(ctx, time.Second*3, func() error {
		return q.createOrUpdate(ctx, spaceID, updater, session)
	})
}

func (q *QuotaRepoImpl) createOrUpdate(ctx context.Context, spaceID int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), session *entity.Session) error {
	key := fmt.Sprintf("lock:quota_space_expt:%d", spaceID)
	locked, ctx, cancel, err := q.mutex.LockBackoffWithRenew(ctx, key, time.Second, time.Second*3)
	if err != nil {
		return err
	}
	if !locked {
		return errorx.New("quota lock already exist, key: %v", key)
	}

	defer func() {
		cancel()
		if _, err := q.mutex.Unlock(key); err != nil {
			logs.CtxWarn(ctx, "failed to unlock key: %v, err: %v", key, err)
		}
	}()

	oldVal, err := q.quotaDAO.GetQuotaSpaceExpt(ctx, spaceID)
	if err != nil {
		return err
	}

	oldVal = lo.Ternary(oldVal != nil, oldVal, &entity.QuotaSpaceExpt{ExptID2RunTime: make(map[int64]int64)})

	newVal, update, err := updater(&entity.QuotaSpaceExpt{
		ExptID2RunTime: maps.Clone(oldVal.ExptID2RunTime),
	})
	if err != nil {
		return err
	}

	if !update {
		return nil
	}

	if err := q.quotaDAO.SetQuotaSpaceExpt(ctx, spaceID, newVal); err != nil {
		return err
	}

	logs.CtxInfo(ctx, "QuotaRepoImpl.CreateOrUpdate success, space_id: %v, before: %v, after: %v", spaceID, oldVal, newVal)
	return nil
}
