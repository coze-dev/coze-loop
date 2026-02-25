// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

func NewExptDeadlineStore(deadlineDAO dao.IExptDeadlineDAO) repo.IExptDeadlineStore {
	return &exptDeadlineStoreImpl{dao: deadlineDAO}
}

type exptDeadlineStoreImpl struct {
	dao dao.IExptDeadlineDAO
}

func (s *exptDeadlineStoreImpl) AddDeadline(ctx context.Context, spaceID, exptID, runID int64, deadlineTs int64) error {
	return s.dao.AddDeadline(ctx, spaceID, exptID, runID, deadlineTs)
}

func (s *exptDeadlineStoreImpl) ScanDue(ctx context.Context, nowUnix int64, limit int) ([]string, error) {
	return s.dao.ScanDue(ctx, nowUnix, limit)
}

func (s *exptDeadlineStoreImpl) TryClaim(ctx context.Context, member string, ttl time.Duration) (bool, error) {
	return s.dao.TryClaim(ctx, member, ttl)
}

func (s *exptDeadlineStoreImpl) Remove(ctx context.Context, member string) error {
	return s.dao.Remove(ctx, member)
}
