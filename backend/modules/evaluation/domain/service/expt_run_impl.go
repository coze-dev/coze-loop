// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func NewQuotaService(quotaRepo repo.QuotaRepo,
	configer component.IConfiger,
) QuotaService {
	return &QuotaServiceImpl{
		QuotaRepo: quotaRepo,
		Configer:  configer,
	}
}

type QuotaServiceImpl struct {
	QuotaRepo repo.QuotaRepo
	Configer  component.IConfiger
}

func (q *QuotaServiceImpl) ReleaseExptRun(ctx context.Context, exptID, spaceID int64, session *entity.Session) error {
	return q.QuotaRepo.CreateOrUpdate(ctx, spaceID, func(cur *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
		if cur == nil || cur.ExptID2RunTime == nil {
			return cur, false, nil
		}

		if _, ok := cur.ExptID2RunTime[exptID]; ok {
			delete(cur.ExptID2RunTime, exptID)
			return cur, true, nil
		}

		return cur, false, nil
	}, session)
}

func (q *QuotaServiceImpl) AllowExptRun(ctx context.Context, exptID, spaceID int64, session *entity.Session) error {
	var (
		now            = time.Now().Unix()
		zombieInterval = q.Configer.GetExptExecConf(ctx, spaceID).GetZombieIntervalSecond()
		concurLimit    = q.Configer.GetExptExecConf(ctx, spaceID).GetSpaceExptConcurLimit()
	)

	return q.QuotaRepo.CreateOrUpdate(ctx, spaceID, func(cur *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
		if cur == nil {
			return &entity.QuotaSpaceExpt{
				ExptID2RunTime: map[int64]int64{
					exptID: now,
				},
			}, true, nil
		}

		if cur.ExptID2RunTime == nil {
			cur.ExptID2RunTime = make(map[int64]int64)
		}

		// 幂等：同一实验重复 Run/RetryItems 只刷新时间，不重复占名额。
		// 必须先判“已存在”，再判 200 上限；否则在正好 200 个运行时，
		// 已在 map 里的实验重试会被误判为超限，导致重试被拒（双计/漏计）。
		if _, exists := cur.ExptID2RunTime[exptID]; exists {
			cur.ExptID2RunTime[exptID] = now
			pruneZombieExptRuns(cur.ExptID2RunTime, now, zombieInterval)
			return cur, true, nil
		}

		if len(cur.ExptID2RunTime) >= concurLimit {
			return nil, false, errorx.NewByCode(errno.ExperimentRunningCountLimitCode, errorx.WithExtraMsg(fmt.Sprintf("max limit: %v", concurLimit)))
		}

		cur.ExptID2RunTime[exptID] = now
		pruneZombieExptRuns(cur.ExptID2RunTime, now, zombieInterval)

		return cur, true, nil
	}, session)
}

func pruneZombieExptRuns(runs map[int64]int64, now int64, zombieInterval int) {
	for eid, rt := range runs {
		if int(now-rt) > zombieInterval {
			delete(runs, eid)
		}
	}
}
