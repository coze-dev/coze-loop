// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate  mockgen -destination=mocks/expt_insight_analysis_record.go  -package mocks . IExptInsightAnalysisRecordDAO
type IExptInsightAnalysisRecordDAO interface {
	Create(ctx context.Context, record *model.ExptInsightAnalysisRecord, opts ...db.Option) error
	Update(ctx context.Context, record *model.ExptInsightAnalysisRecord, opts ...db.Option) error
	GetByID(ctx context.Context, spaceID, exptID, recordID int64, opts ...db.Option) (*model.ExptInsightAnalysisRecord, error)
	List(ctx context.Context, spaceID, exptID int64, page entity.Page) ([]*model.ExptInsightAnalysisRecord, int64, error)
	Delete(ctx context.Context, spaceID, exptID, recordID int64) error
}

func NewExptInsightAnalysisRecordDAO(db db.Provider) IExptInsightAnalysisRecordDAO {
	return &exptInsightAnalysisRecordDAO{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type exptInsightAnalysisRecordDAO struct {
	db    db.Provider
	query *query.Query
}

func (e exptInsightAnalysisRecordDAO) Create(ctx context.Context, record *model.ExptInsightAnalysisRecord, opts ...db.Option) error {
	if err := e.db.NewSession(ctx, opts...).Create(record).Error; err != nil {
		return errorx.Wrapf(err, "exptInsightAnalysisRecordDAO create fail, model: %v", json.Jsonify(record))
	}
	return nil
}

func (e exptInsightAnalysisRecordDAO) Update(ctx context.Context, record *model.ExptInsightAnalysisRecord, opts ...db.Option) error {
	if err := e.db.NewSession(ctx, opts...).Model(&model.ExptInsightAnalysisRecord{}).Where("id = ?", record.ID).Updates(record).Error; err != nil {
		return errorx.Wrapf(err, "exptInsightAnalysisRecordDAO update fail, model: %v", json.Jsonify(record))
	}
	return nil
}

func (e exptInsightAnalysisRecordDAO) GetByID(ctx context.Context, spaceID, exptID, recordID int64, opts ...db.Option) (*model.ExptInsightAnalysisRecord, error) {
	db := e.db.NewSession(ctx)
	q := query.Use(db).ExptInsightAnalysisRecord

	record, err := q.WithContext(ctx).Where(
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.ID.Eq(recordID),
	).First()
	if err != nil {
		return nil, errorx.Wrapf(err, "exptInsightAnalysisRecordDAO GetByID fail, recordID: %v", recordID)
	}

	return record, nil
}

func (e exptInsightAnalysisRecordDAO) List(ctx context.Context, spaceID, exptID int64, page entity.Page) ([]*model.ExptInsightAnalysisRecord, int64, error) {
	var (
		finds []*model.ExptInsightAnalysisRecord
		total int64
	)

	db := e.db.NewSession(ctx).Model(&model.ExptInsightAnalysisRecord{}).Where("space_id = ?", spaceID).Where("expt_id = ?", exptID)

	db = db.Order("created_at desc")
	// 总记录数
	db = db.Count(&total)
	// 分页
	db = db.Offset(page.Offset()).Limit(page.Limit())
	err := db.Find(&finds).Error
	if err != nil {
		return nil, 0, err
	}
	return finds, total, nil
}

func (e exptInsightAnalysisRecordDAO) Delete(ctx context.Context, spaceID, exptID, recordID int64) error {
	po := &model.ExptInsightAnalysisRecord{}
	db := e.db.NewSession(ctx)
	err := db.Where("space_id = ? AND expt_id = ?  AND record_id = ?", spaceID, exptID, recordID).
		Delete(po).Error
	if err != nil {
		return err
	}

	return nil
}
