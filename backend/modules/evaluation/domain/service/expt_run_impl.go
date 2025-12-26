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

		if len(cur.ExptID2RunTime) >= concurLimit {
			return nil, false, errorx.NewByCode(errno.ExperimentRunningCountLimitCode, errorx.WithExtraMsg(fmt.Sprintf("max limit: %v", concurLimit)))
		}

		if cur.ExptID2RunTime == nil {
			cur.ExptID2RunTime = make(map[int64]int64)
		}

		cur.ExptID2RunTime[exptID] = now

		for eid, rt := range cur.ExptID2RunTime {
			if int(now-rt) > zombieInterval {
				delete(cur.ExptID2RunTime, eid)
			}
		}

		return cur, true, nil
	}, session)
}
