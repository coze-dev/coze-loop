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
)

type ExptResultPublishRepoImpl struct{ dao repo.IExptResultPublishRepo }

func NewExptResultPublishRepo(provider db.Provider) repo.IExptResultPublishRepo {
	return &ExptResultPublishRepoImpl{dao: mysql.NewExptResultPublishDAO(provider)}
}

func (r *ExptResultPublishRepoImpl) GetItemResultPublishSource(ctx context.Context, space, expt, run, item int64) (*entity.ExptItemResultPublishSource, error) {
	return r.dao.GetItemResultPublishSource(ctx, space, expt, run, item)
}

func (r *ExptResultPublishRepoImpl) GetStandardEvalOutputSource(ctx context.Context, space, expt, run, item int64) (*entity.ExptItemResultPublishSource, error) {
	port, ok := r.dao.(repo.IExptStandardEvalOutputSourceRepo)
	if !ok {
		return nil, fmt.Errorf("standard eval output source reader missing")
	}
	return port.GetStandardEvalOutputSource(ctx, space, expt, run, item)
}

func (r *ExptResultPublishRepoImpl) IsItemResultPublishSourceCurrent(ctx context.Context, source *entity.ExptItemResultPublishSource) (bool, error) {
	return r.dao.IsItemResultPublishSourceCurrent(ctx, source)
}

func (r *ExptResultPublishRepoImpl) CommitItemResult(ctx context.Context, publication *entity.ExptItemResultPublication) (bool, error) {
	return r.dao.CommitItemResult(ctx, publication)
}
