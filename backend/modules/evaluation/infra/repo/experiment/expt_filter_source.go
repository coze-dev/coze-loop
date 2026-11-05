// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type exptFilterSourceScanner struct{ provider db.Provider }

func NewExptFilterSourceScanner(provider db.Provider) repo.IExptFilterSourceScanner {
	return &exptFilterSourceScanner{provider: provider}
}

func (s *exptFilterSourceScanner) BeginScan(ctx context.Context) (entity.ExptFilterScanBounds, error) {
	var upper int64
	err := s.provider.NewSession(ctx, db.WithMaster()).Model(&model.Experiment{}).Select("COALESCE(MAX(id), 0)").Scan(&upper).Error
	return entity.ExptFilterScanBounds{MaxExptID: upper}, err
}

func validFilterScan(cursor, upper int64, limit int) error {
	if cursor < 0 || upper < 0 || cursor > upper || limit < 1 || limit > 1000 {
		return fmt.Errorf("invalid filter scan bounds or page size")
	}
	return nil
}

func (s *exptFilterSourceScanner) ScanExperiments(ctx context.Context, bounds entity.ExptFilterScanBounds, cursor int64, limit int) (entity.ExptFilterExperimentPage, error) {
	page := entity.ExptFilterExperimentPage{}
	if err := validFilterScan(cursor, bounds.MaxExptID, limit); err != nil {
		return page, err
	}
	var rows []*model.Experiment
	err := s.provider.NewSession(ctx, db.WithMaster()).Model(&model.Experiment{}).Select("id, space_id").Where("id > ? AND id <= ?", cursor, bounds.MaxExptID).Order("id ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return page, err
	}
	for _, row := range rows {
		if row == nil || row.ID <= cursor || row.ID > bounds.MaxExptID || row.SpaceID <= 0 {
			return entity.ExptFilterExperimentPage{}, fmt.Errorf("source_unknown: invalid experiment cursor")
		}
		page.Scopes = append(page.Scopes, entity.ExptFilterScope{SpaceID: row.SpaceID, ExptID: row.ID})
		page.NextExptID = row.ID
		cursor = row.ID
	}
	return page, nil
}

func (s *exptFilterSourceScanner) TurnUpperBound(ctx context.Context, scope entity.ExptFilterScope) (int64, error) {
	if scope.SpaceID <= 0 || scope.ExptID <= 0 {
		return 0, fmt.Errorf("invalid filter scan scope")
	}
	var upper int64
	err := s.provider.NewSession(ctx, db.WithMaster()).Model(&model.ExptTurnResult{}).Select("COALESCE(MAX(id), 0)").Where("space_id = ? AND expt_id = ?", scope.SpaceID, scope.ExptID).Scan(&upper).Error
	return upper, err
}

func (s *exptFilterSourceScanner) ScanTurns(ctx context.Context, scope entity.ExptFilterScope, cursor, upper int64, limit int) ([]*entity.ExptTurnResult, int64, error) {
	if scope.SpaceID <= 0 || scope.ExptID <= 0 {
		return nil, 0, fmt.Errorf("invalid filter scan scope")
	}
	if err := validFilterScan(cursor, upper, limit); err != nil {
		return nil, 0, err
	}
	var rows []*model.ExptTurnResult
	err := s.provider.NewSession(ctx, db.WithMaster()).Where("space_id = ? AND expt_id = ? AND id > ? AND id <= ?", scope.SpaceID, scope.ExptID, cursor, upper).Order("id ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	result := make([]*entity.ExptTurnResult, 0, len(rows))
	var next int64
	for _, row := range rows {
		if row == nil || row.ID <= cursor || row.ID > upper || row.SpaceID != scope.SpaceID || row.ExptID != scope.ExptID || row.ItemID <= 0 || row.TurnID < 0 {
			return nil, 0, fmt.Errorf("source_unknown: invalid turn cursor or ownership")
		}
		result = append(result, convert.NewExptTurnResultConvertor().PO2DO(row, nil))
		next = row.ID
		cursor = row.ID
	}
	return result, next, nil
}
