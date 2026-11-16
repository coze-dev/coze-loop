// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// ListCentralActive 只按状态扫描活跃实验（不做全表扫描，也不新增索引/列）。
// central 模式与环境归属在 eval_conf JSON 中，由上层过滤；status 走既有索引。
func ListCentralActive(ctx context.Context, provider db.Provider, statuses []int32, limit int) ([]*model.Experiment, error) {
	pos := make([]*model.Experiment, 0)
	session := provider.NewSession(ctx).Model(&model.Experiment{})
	session = session.Where("status IN ?", statuses).
		Where("visibility <> ?", int32(entity.Visibility_Hidden))
	if limit > 0 {
		session = session.Limit(limit)
	}
	if err := session.Find(&pos).Error; err != nil {
		return nil, err
	}
	return pos, nil
}
