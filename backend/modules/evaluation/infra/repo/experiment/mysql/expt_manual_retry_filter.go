// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"fmt"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

var _ repo.IExptManualRetryFilterRepo = (*exptItemResultDAOImpl)(nil)

func (dao *exptItemResultDAOImpl) ScanManualRetryFilterItems(ctx context.Context, space, expt, run, cursor int64, limit int) ([]int64, int64, error) {
	if limit <= 0 {
		return nil, 0, fmt.Errorf("invalid manual retry filter page size")
	}
	var rows []*model.ExptItemResult
	err := dao.provider.NewSession(ctx, db.WithMaster()).Select("id,item_id,expt_run_id,ext").Where("space_id=? AND expt_id=? AND expt_run_id=? AND id>? AND ext IS NOT NULL", space, expt, run, cursor).Order("id ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	ids := []int64{}
	for _, row := range rows {
		ext, err := entity.ParseExptItemResultExt(gptr.Indirect(row.Ext))
		if err != nil {
			return nil, 0, err
		}
		manual, err := entity.ParseManualRetryContext(ext)
		if err != nil {
			return nil, 0, err
		}
		if manual == nil {
			continue
		}
		if manual.RunID != row.ExptRunID {
			return nil, 0, fmt.Errorf("manual retry filter canonical run mismatch")
		}
		ids = append(ids, row.ItemID)
	}
	next := int64(0)
	if len(rows) == limit {
		next = rows[len(rows)-1].ID
	}
	return ids, next, nil
}
